// Package authn is account and session handling (M1, P0-006, P0-011, P0-145).
//
// Sessions are random 256-bit tokens; only their SHA-256 is stored. Each login
// increments accounts.login_generation and a session is valid only while its
// generation matches, so a new login invalidates every older session of the
// account on the next check (A-01).
package authn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUsernameTaken      = errors.New("authn: username taken")
	ErrInvalidCredentials = errors.New("authn: invalid credentials")
	ErrBanned             = errors.New("authn: account banned")
	ErrNoSession          = errors.New("authn: no valid session")
)

// Login names are ASCII for now (M1 v0.2); character names are the Unicode
// display names players see.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

const (
	MinPasswordBytes = 8  // [ĐỀ XUẤT] final password policy is not decided
	MaxPasswordBytes = 72 // bcrypt limit
)

func ValidUsername(u string) bool { return usernamePattern.MatchString(u) }

func ValidPassword(p string) bool { return len(p) >= MinPasswordBytes && len(p) <= MaxPasswordBytes }

type Store struct {
	Pool       *pgxpool.Pool
	SessionTTL time.Duration
	BcryptCost int
	Now        func() time.Time
	dummyOnce  sync.Once
	dummyHash  []byte
}

func NewStore(pool *pgxpool.Pool) *Store {
	s := &Store{Pool: pool, SessionTTL: 7 * 24 * time.Hour, BcryptCost: 12, Now: time.Now}
	return s
}

// Register creates an account. It never creates a character (docs/03 §1).
func (s *Store) Register(ctx context.Context, username, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.BcryptCost)
	if err != nil {
		return "", err
	}
	var id string
	err = s.Pool.QueryRow(ctx, `INSERT INTO accounts (username, username_key, password_hash) VALUES ($1, $2, $3) RETURNING id::text`,
		username, strings.ToLower(username), string(hash)).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return "", ErrUsernameTaken
	}
	return id, err
}

type Session struct {
	Token     string
	AccountID string
	ExpiresAt time.Time
}

// Login checks the password and opens a new session that supersedes all
// earlier sessions of the account.
func (s *Store) Login(ctx context.Context, username, password string) (*Session, error) {
	var id, hash, status string
	var totpOn *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT id::text, password_hash, status, totp_enabled_at FROM accounts WHERE username_key = $1`,
		strings.ToLower(username)).Scan(&id, &hash, &status, &totpOn)
	if errors.Is(err, pgx.ErrNoRows) {
		// Same cost as a real check, so timing does not reveal which names exist.
		_ = bcrypt.CompareHashAndPassword(s.dummy(), []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	if status != "active" {
		return nil, ErrBanned
	}
	if totpOn != nil {
		// Password is right; the session opens only after the TOTP step.
		ch, err := s.newChallenge(ctx, id)
		if err != nil {
			return nil, err
		}
		return nil, ch
	}
	return s.openSession(ctx, id)
}

// openSession supersedes every earlier session of the account (P0-006).
func (s *Store) openSession(ctx context.Context, id string) (*Session, error) {
	token, digest, err := newToken()
	if err != nil {
		return nil, err
	}
	now := s.Now().UTC()
	sess := &Session{Token: token, AccountID: id, ExpiresAt: now.Add(s.SessionTTL)}
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var gen int64
		if err := tx.QueryRow(ctx, `UPDATE accounts SET login_generation = login_generation + 1, updated_at = $2
			WHERE id = $1 RETURNING login_generation`, id, now).Scan(&gen); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO auth_sessions (account_id, token_hash, login_generation, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5)`, id, digest, gen, now, sess.ExpiresAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

type Principal struct {
	AccountID string
	Username  string
}

// BanReason is shown to a banned player at login (TH-14).
func (s *Store) BanReason(ctx context.Context, username string) string {
	var reason *string
	_ = s.Pool.QueryRow(ctx, `SELECT ban_reason FROM accounts WHERE username_key = $1`, strings.ToLower(username)).Scan(&reason)
	if reason == nil {
		return ""
	}
	return *reason
}

// Authenticate validates a session token against the database every time:
// revocation, expiry, ban and newer logins all take effect immediately.
func (s *Store) Authenticate(ctx context.Context, token string) (*Principal, error) {
	if token == "" || len(token) > 128 {
		return nil, ErrNoSession
	}
	digest := sha256.Sum256([]byte(token))
	p := &Principal{}
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT a.id::text, a.username, a.status
		FROM auth_sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > $2
		  AND s.login_generation = a.login_generation`, digest[:], s.Now().UTC()).Scan(&p.AccountID, &p.Username, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	if status != "active" {
		return nil, ErrBanned
	}
	return p, nil
}

// Logout revokes only this session; a newer login elsewhere is unaffected.
func (s *Store) Logout(ctx context.Context, token string) error {
	digest := sha256.Sum256([]byte(token))
	_, err := s.Pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`, digest[:], s.Now().UTC())
	return err
}

func newToken() (string, []byte, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b[:])
	digest := sha256.Sum256([]byte(token))
	return token, digest[:], nil
}

func (s *Store) dummy() []byte {
	s.dummyOnce.Do(func() {
		s.dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalizer"), s.BcryptCost)
	})
	return s.dummyHash
}
