// Package admin is the Admin side (MVP-08, docs/03 §7, P0-142..144, P0-152):
// identities separate from players, mandatory TOTP, permission-based RBAC,
// two-person asset adjustments, an append-only audit log, the maintenance
// switch and config publishing through maintenance.
package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"example.com/avatar/game-server/internal/secretbox"
)

var (
	ErrInvalidLogin    = errors.New("admin: invalid login")
	ErrNoSession       = errors.New("admin: no session")
	ErrForbidden       = errors.New("admin: permission denied")
	ErrSelfApproval    = errors.New("admin: cannot approve own request")
	ErrNotPending      = errors.New("admin: request already decided")
	ErrNotFound        = errors.New("admin: not found")
	ErrMaintenanceOff  = errors.New("admin: maintenance must be active")
	ErrInvalidDraft    = errors.New("admin: invalid draft")
	ErrLoginNameTaken  = errors.New("admin: login name taken")
	ErrInvalidArgument = errors.New("admin: invalid argument")
)

// Permissions (migration 0008).
const (
	PermManage      = "admin.manage"
	PermPlayerRead  = "player.read"
	PermPlayerBan   = "player.ban"
	PermPropose     = "asset.adjust.propose"
	PermApprove     = "asset.adjust.approve"
	PermMaintenance = "maintenance.manage"
	PermPublish     = "config.publish"
	PermAudit       = "audit.read"
)

type Store struct {
	Pool       *pgxpool.Pool
	Box        *secretbox.Box
	Now        func() time.Time
	BcryptCost int
	SessionTTL time.Duration
	dummyOnce  sync.Once
	dummyHash  []byte
}

func New(pool *pgxpool.Pool, box *secretbox.Box) *Store {
	return &Store{Pool: pool, Box: box, Now: time.Now, BcryptCost: 12, SessionTTL: 8 * time.Hour}
}

func hash(parts ...string) []byte {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return h[:]
}

func token() (string, []byte) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	t := base64.RawURLEncoding.EncodeToString(b[:])
	return t, hash(t)
}

var loginPattern = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)

// Created is shown once to the operator who ran the bootstrap command.
type Created struct {
	ID            string   `json:"id"`
	Login         string   `json:"login"`
	Password      string   `json:"password"`
	TOTPSecret    string   `json:"totp_secret"`
	TOTPURL       string   `json:"totp_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// Create makes an Admin with a generated password and TOTP secret. TOTP is
// never optional for Admins (P0-143): the account cannot log in without it.
func (s *Store) Create(ctx context.Context, login string, roles []string) (*Created, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if !loginPattern.MatchString(login) || len(roles) == 0 {
		return nil, ErrInvalidArgument
	}
	var pw [12]byte
	if _, err := rand.Read(pw[:]); err != nil {
		return nil, err
	}
	password := base64.RawURLEncoding.EncodeToString(pw[:])
	ph, err := bcrypt.GenerateFromPassword([]byte(password), s.BcryptCost)
	if err != nil {
		return nil, err
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Avatar Admin", AccountName: login})
	if err != nil {
		return nil, err
	}
	out := &Created{Login: login, Password: password, TOTPSecret: key.Secret(), TOTPURL: key.URL()}
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO admin_users (login_name, password_hash, totp_secret_enc) VALUES ($1, $2, $3) RETURNING id::text`,
			login, string(ph), s.Box.Seal([]byte(key.Secret()))).Scan(&out.ID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrLoginNameTaken
			}
			return err
		}
		for _, r := range roles {
			if _, err := tx.Exec(ctx, `INSERT INTO admin_user_roles (admin_id, role_code) VALUES ($1, $2)`, out.ID, strings.ToUpper(r)); err != nil {
				return err
			}
		}
		for i := 0; i < 10; i++ {
			var b [5]byte
			_, _ = rand.Read(b[:])
			c := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
			c = c[:4] + "-" + c[4:]
			out.RecoveryCodes = append(out.RecoveryCodes, c)
			if _, err := tx.Exec(ctx, `INSERT INTO admin_recovery_codes (admin_id, code_hash) VALUES ($1, $2)`, out.ID, hash(out.ID, c)); err != nil {
				return err
			}
		}
		return s.audit(ctx, tx, nil, "admin.create", "admin_user", out.ID, map[string]any{"login": login, "roles": roles})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StartLogin checks the password and returns the TOTP challenge.
func (s *Store) StartLogin(ctx context.Context, login, password string) (string, error) {
	var id, ph string
	var active bool
	err := s.Pool.QueryRow(ctx, `SELECT id::text, password_hash, is_active FROM admin_users WHERE login_name = $1`,
		strings.ToLower(login)).Scan(&id, &ph, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		s.dummyOnce.Do(func() { s.dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing"), s.BcryptCost) })
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password)) // same cost as a real check
		_ = s.auditPlain(ctx, nil, "admin.login_failed", "admin_user", "", map[string]any{"login": login})
		return "", ErrInvalidLogin
	}
	if err != nil {
		return "", err
	}
	if !active || bcrypt.CompareHashAndPassword([]byte(ph), []byte(password)) != nil {
		_ = s.auditPlain(ctx, &id, "admin.login_failed", "admin_user", id, nil)
		return "", ErrInvalidLogin
	}
	t, h := token()
	_, err = s.Pool.Exec(ctx, `INSERT INTO admin_login_challenges (token_hash, admin_id, expires_at) VALUES ($1, $2, $3)`, h, id, s.Now().UTC().Add(5*time.Minute))
	return t, err
}

type Session struct {
	Token     string
	ExpiresAt time.Time
}

// FinishLogin verifies the TOTP (or a one-time recovery code) and opens a
// session; a new Admin login ends that Admin's older sessions.
func (s *Store) FinishLogin(ctx context.Context, challenge, code string) (*Session, error) {
	var sess *Session
	var adminID string
	wrong := false
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var attempts int
		var expires time.Time
		var used *time.Time
		var enc []byte
		err := tx.QueryRow(ctx, `SELECT c.admin_id::text, c.attempts, c.expires_at, c.used_at, u.totp_secret_enc
			FROM admin_login_challenges c JOIN admin_users u ON u.id = c.admin_id
			WHERE c.token_hash = $1 AND u.is_active FOR UPDATE OF c`, hash(challenge)).Scan(&adminID, &attempts, &expires, &used, &enc)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidLogin
		}
		if err != nil {
			return err
		}
		if used != nil || s.Now().After(expires) || attempts >= 5 {
			return ErrInvalidLogin
		}
		secret, err := s.Box.Open(enc)
		if err != nil {
			return err
		}
		code = strings.ToLower(strings.TrimSpace(code))
		ok, _ := totp.ValidateCustom(code, string(secret), s.Now(), totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if !ok {
			tag, err := tx.Exec(ctx, `UPDATE admin_recovery_codes SET used_at = $3 WHERE admin_id = $1 AND code_hash = $2 AND used_at IS NULL`,
				adminID, hash(adminID, code), s.Now().UTC())
			if err != nil {
				return err
			}
			ok = tag.RowsAffected() == 1
		}
		if !ok {
			wrong = true
			return ErrInvalidLogin
		}
		now := s.Now().UTC()
		if _, err := tx.Exec(ctx, `UPDATE admin_login_challenges SET used_at = $2 WHERE token_hash = $1`, hash(challenge), now); err != nil {
			return err
		}
		var gen int64
		if err := tx.QueryRow(ctx, `UPDATE admin_users SET login_generation = login_generation + 1 WHERE id = $1 RETURNING login_generation`, adminID).Scan(&gen); err != nil {
			return err
		}
		t, h := token()
		sess = &Session{Token: t, ExpiresAt: now.Add(s.SessionTTL)}
		if _, err := tx.Exec(ctx, `INSERT INTO admin_sessions (token_hash, admin_id, login_generation, created_at, expires_at) VALUES ($1, $2, $3, $4, $5)`,
			h, adminID, gen, now, sess.ExpiresAt); err != nil {
			return err
		}
		return s.audit(ctx, tx, &adminID, "admin.login", "admin_user", adminID, nil)
	})
	if wrong {
		_, _ = s.Pool.Exec(ctx, `UPDATE admin_login_challenges SET attempts = attempts + 1 WHERE token_hash = $1`, hash(challenge))
		_ = s.auditPlain(ctx, &adminID, "admin.totp_failed", "admin_user", adminID, nil)
	}
	return sess, err
}

type Principal struct {
	ID          string          `json:"id"`
	Login       string          `json:"login"`
	Roles       []string        `json:"roles"`
	Permissions map[string]bool `json:"permissions"`
}

func (p *Principal) Can(perm string) bool { return p != nil && p.Permissions[perm] }

func (s *Store) Authenticate(ctx context.Context, tok string) (*Principal, error) {
	if tok == "" {
		return nil, ErrNoSession
	}
	p := &Principal{Permissions: map[string]bool{}}
	err := s.Pool.QueryRow(ctx, `SELECT u.id::text, u.login_name FROM admin_sessions s JOIN admin_users u ON u.id = s.admin_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > $2 AND s.login_generation = u.login_generation AND u.is_active`,
		hash(tok), s.Now().UTC()).Scan(&p.ID, &p.Login)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT ur.role_code, rp.permission_code FROM admin_user_roles ur
		LEFT JOIN admin_role_permissions rp ON rp.role_code = ur.role_code WHERE ur.admin_id = $1`, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var role string
		var perm *string
		if err := rows.Scan(&role, &perm); err != nil {
			return nil, err
		}
		if !seen[role] {
			seen[role] = true
			p.Roles = append(p.Roles, role)
		}
		if perm != nil {
			p.Permissions[*perm] = true
		}
	}
	return p, rows.Err()
}

func (s *Store) Logout(ctx context.Context, tok string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE admin_sessions SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`, hash(tok), s.Now().UTC())
	return err
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (s *Store) audit(ctx context.Context, q execer, adminID *string, action, objType, objID string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, _ := json.Marshal(details)
	_, err := q.Exec(ctx, `INSERT INTO admin_audit_logs (admin_id, action, object_type, object_id, details, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		adminID, action, objType, objID, raw, s.Now().UTC())
	return err
}

func (s *Store) auditPlain(ctx context.Context, adminID *string, action, objType, objID string, details map[string]any) error {
	return s.audit(ctx, s.Pool, adminID, action, objType, objID, details)
}

// Audit records an action of an authenticated Admin.
func (s *Store) Audit(ctx context.Context, p *Principal, action, objType, objID string, details map[string]any) error {
	return s.auditPlain(ctx, &p.ID, action, objType, objID, details)
}

type AuditEntry struct {
	ID        int64           `json:"id"`
	Admin     *string         `json:"admin"`
	Action    string          `json:"action"`
	Object    string          `json:"object"`
	ObjectID  *string         `json:"object_id"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

func (s *Store) AuditLog(ctx context.Context, before int64, limit int) ([]AuditEntry, error) {
	if before <= 0 {
		before = 1 << 62
	}
	rows, err := s.Pool.Query(ctx, `SELECT l.id, u.login_name, l.action, l.object_type, l.object_id, l.details, l.created_at
		FROM admin_audit_logs l LEFT JOIN admin_users u ON u.id = l.admin_id
		WHERE l.id < $1 ORDER BY l.id DESC LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[AuditEntry])
}
