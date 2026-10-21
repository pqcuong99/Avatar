package authn

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"example.com/project-thanh/game-server/internal/mail"
	"example.com/project-thanh/game-server/internal/secretbox"
)

// Optional player 2FA (P0-146), recovery email and password reset (P0-011,
// 014..016). Admin 2FA is mandatory and lives in package admin.

var (
	ErrTOTPRequired      = errors.New("authn: totp code required")
	ErrBadCode           = errors.New("authn: wrong or expired code")
	ErrTOTPAlreadyOn     = errors.New("authn: totp already enabled")
	ErrTOTPNotOn         = errors.New("authn: totp not enabled")
	ErrNoEnrollment      = errors.New("authn: no totp enrollment in progress")
	ErrMailNotConfigured = errors.New("authn: email is not configured")
	ErrBadEmail          = errors.New("authn: invalid email")
)

const (
	ResetCodeTTL    = 10 * time.Minute // P0-016
	VerifyCodeTTL   = 30 * time.Minute // [ĐỀ XUẤT]
	ChallengeTTL    = 5 * time.Minute
	maxCodeAttempts = 5
	RecoveryCodes   = 10
)

// Security bundles what the security flows need besides the Store.
type Security struct {
	Box    *secretbox.Box
	Mail   mail.Sender // nil: email features off (GAP-16)
	Issuer string
}

// TOTPChallenge is returned by Login when the account has 2FA on.
type TOTPChallenge struct{ Token string }

func (c *TOTPChallenge) Error() string { return ErrTOTPRequired.Error() }
func (c *TOTPChallenge) Unwrap() error { return ErrTOTPRequired }

func digest(parts ...string) []byte {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return h[:]
}

func randomToken() (string, []byte) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	t := base64.RawURLEncoding.EncodeToString(b[:])
	return t, digest(t)
}

func sixDigits() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// newChallenge is the first half of a 2FA login.
func (s *Store) newChallenge(ctx context.Context, accountID string) (*TOTPChallenge, error) {
	token, h := randomToken()
	_, err := s.Pool.Exec(ctx, `INSERT INTO login_challenges (token_hash, account_id, expires_at) VALUES ($1, $2, $3)`,
		h, accountID, s.Now().UTC().Add(ChallengeTTL))
	return &TOTPChallenge{Token: token}, err
}

// CompleteTOTP finishes a 2FA login with a TOTP code or a one-time recovery
// code and opens the session.
func (s *Store) CompleteTOTP(ctx context.Context, sec *Security, challenge, code string) (*Session, error) {
	var accountID string
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var attempts int
		var expires time.Time
		var used *time.Time
		err := tx.QueryRow(ctx, `SELECT account_id::text, attempts, expires_at, used_at FROM login_challenges WHERE token_hash = $1 FOR UPDATE`,
			digest(challenge)).Scan(&accountID, &attempts, &expires, &used)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBadCode
		}
		if err != nil {
			return err
		}
		if used != nil || s.Now().After(expires) || attempts >= maxCodeAttempts {
			return ErrBadCode
		}
		ok, err := s.checkSecondFactor(ctx, tx, sec, accountID, code)
		if err != nil {
			return err
		}
		if !ok {
			return &wrongCode{stmt: `UPDATE login_challenges SET attempts = attempts + 1 WHERE token_hash = $1`, arg: digest(challenge)}
		}
		_, err = tx.Exec(ctx, `UPDATE login_challenges SET used_at = $2 WHERE token_hash = $1`, digest(challenge), s.Now().UTC())
		return err
	})
	if err = s.recordWrong(ctx, err); err != nil {
		return nil, err
	}
	return s.openSession(ctx, accountID)
}

// wrongCode reports a wrong code. The failed attempt is counted after the
// transaction (which rolls back) so that the count survives.
type wrongCode struct {
	stmt string
	arg  any
}

func (e *wrongCode) Error() string { return ErrBadCode.Error() }

func (s *Store) recordWrong(ctx context.Context, err error) error {
	var w *wrongCode
	if errors.As(err, &w) {
		_, _ = s.Pool.Exec(ctx, w.stmt, w.arg)
		return ErrBadCode
	}
	return err
}

// checkSecondFactor accepts a current TOTP code or an unused recovery code
// (which is then spent).
func (s *Store) checkSecondFactor(ctx context.Context, tx pgx.Tx, sec *Security, accountID, code string) (bool, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	var enc []byte
	if err := tx.QueryRow(ctx, `SELECT totp_secret_enc FROM accounts WHERE id = $1 AND totp_enabled_at IS NOT NULL`, accountID).Scan(&enc); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	secret, err := sec.Box.Open(enc)
	if err != nil {
		return false, err
	}
	if len(code) == 6 && validTOTP(code, string(secret), s.Now()) {
		return true, nil
	}
	tag, err := tx.Exec(ctx, `UPDATE account_recovery_codes SET used_at = $3 WHERE account_id = $1 AND code_hash = $2 AND used_at IS NULL`,
		accountID, digest(accountID, code), s.Now().UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func validTOTP(code, secret string, at time.Time) bool {
	ok, _ := totp.ValidateCustom(code, secret, at, totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	return ok
}

type Enrollment struct {
	Secret    string `json:"secret"`
	URL       string `json:"otpauth_url"`
	QRDataURL string `json:"qr_png"`
}

// EnrollTOTP prepares a new secret; 2FA is on only after EnableTOTP proves
// the authenticator works.
func (s *Store) EnrollTOTP(ctx context.Context, sec *Security, accountID, username string) (*Enrollment, error) {
	var on *time.Time
	if err := s.Pool.QueryRow(ctx, `SELECT totp_enabled_at FROM accounts WHERE id = $1`, accountID).Scan(&on); err != nil {
		return nil, err
	}
	if on != nil {
		return nil, ErrTOTPAlreadyOn
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: sec.Issuer, AccountName: username})
	if err != nil {
		return nil, err
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE accounts SET totp_pending_secret_enc = $2 WHERE id = $1`, accountID, sec.Box.Seal([]byte(key.Secret()))); err != nil {
		return nil, err
	}
	img, err := key.Image(200, 200)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return &Enrollment{Secret: key.Secret(), URL: key.URL(), QRDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())}, nil
}

// EnableTOTP checks a code against the pending secret, turns 2FA on and
// returns recovery codes, shown once and stored only as hashes.
func (s *Store) EnableTOTP(ctx context.Context, sec *Security, accountID, code string) ([]string, error) {
	var codes []string
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var pending []byte
		var on *time.Time
		if err := tx.QueryRow(ctx, `SELECT totp_pending_secret_enc, totp_enabled_at FROM accounts WHERE id = $1 FOR UPDATE`, accountID).Scan(&pending, &on); err != nil {
			return err
		}
		if on != nil {
			return ErrTOTPAlreadyOn
		}
		if pending == nil {
			return ErrNoEnrollment
		}
		secret, err := sec.Box.Open(pending)
		if err != nil {
			return err
		}
		if !validTOTP(strings.TrimSpace(code), string(secret), s.Now()) {
			return ErrBadCode
		}
		if _, err := tx.Exec(ctx, `UPDATE accounts SET totp_secret_enc = totp_pending_secret_enc, totp_pending_secret_enc = NULL, totp_enabled_at = $2 WHERE id = $1`,
			accountID, s.Now().UTC()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM account_recovery_codes WHERE account_id = $1`, accountID); err != nil {
			return err
		}
		codes = make([]string, RecoveryCodes)
		for i := range codes {
			codes[i] = recoveryCode()
			if _, err := tx.Exec(ctx, `INSERT INTO account_recovery_codes (account_id, code_hash) VALUES ($1, $2)`, accountID, digest(accountID, codes[i])); err != nil {
				return err
			}
		}
		return nil
	})
	return codes, err
}

func recoveryCode() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	s := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
	return s[:4] + "-" + s[4:]
}

// DisableTOTP needs the password and a current code (or a recovery code):
// proof of ownership, not just a stolen session (docs/03 §1).
func (s *Store) DisableTOTP(ctx context.Context, sec *Security, accountID, password, code string) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var hash string
		var on *time.Time
		if err := tx.QueryRow(ctx, `SELECT password_hash, totp_enabled_at FROM accounts WHERE id = $1 FOR UPDATE`, accountID).Scan(&hash, &on); err != nil {
			return err
		}
		if on == nil {
			return ErrTOTPNotOn
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			return ErrInvalidCredentials
		}
		ok, err := s.checkSecondFactor(ctx, tx, sec, accountID, code)
		if err != nil {
			return err
		}
		if !ok {
			return ErrBadCode
		}
		if _, err := tx.Exec(ctx, `UPDATE accounts SET totp_secret_enc = NULL, totp_enabled_at = NULL, totp_pending_secret_enc = NULL WHERE id = $1`, accountID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM account_recovery_codes WHERE account_id = $1`, accountID)
		return err
	})
}

type SecurityStatus struct {
	Email         *string `json:"email"`
	EmailVerified bool    `json:"email_verified"`
	TOTPEnabled   bool    `json:"totp_enabled"`
	RecoveryLeft  int     `json:"recovery_codes_left"`
	EmailEnabled  bool    `json:"email_available"`
}

func (s *Store) Status(ctx context.Context, sec *Security, accountID string) (*SecurityStatus, error) {
	st := &SecurityStatus{EmailEnabled: sec.Mail != nil}
	var verified, on *time.Time
	if err := s.Pool.QueryRow(ctx, `SELECT email, email_verified_at, totp_enabled_at,
		(SELECT count(*) FROM account_recovery_codes WHERE account_id = $1 AND used_at IS NULL)
		FROM accounts WHERE id = $1`, accountID).Scan(&st.Email, &verified, &on, &st.RecoveryLeft); err != nil {
		return nil, err
	}
	st.EmailVerified, st.TOTPEnabled = verified != nil, on != nil
	return st, nil
}

var emailPattern = regexp.MustCompile(`^[^@\s]{1,64}@[^@\s]{1,190}\.[^@\s]{2,}$`)

// SetEmail records a new, unverified recovery address and mails a code.
func (s *Store) SetEmail(ctx context.Context, sec *Security, accountID, email string) error {
	if sec.Mail == nil {
		return ErrMailNotConfigured
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if !emailPattern.MatchString(email) {
		return ErrBadEmail
	}
	code := sixDigits()
	now := s.Now().UTC()
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE accounts SET email = $2, email_verified_at = NULL WHERE id = $1`, accountID, email); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO account_codes (account_id, purpose, email, code_hash, created_at, expires_at)
			VALUES ($1, 'verify_email', $2, $3, $4, $5)`, accountID, email, digest(accountID, "verify_email", code), now, now.Add(VerifyCodeTTL))
		return err
	})
	if err != nil {
		return err
	}
	return sec.Mail.Send(ctx, email, "Project Thanh: mã xác minh email", "Mã xác minh: "+code+" (hết hạn sau 30 phút).")
}

// useCode spends the newest unexpired code of a purpose; five wrong tries
// burn it.
func (s *Store) useCode(ctx context.Context, tx pgx.Tx, accountID, purpose, code string) (string, error) {
	var id, email string
	var hash []byte
	var attempts int
	err := tx.QueryRow(ctx, `SELECT id::text, email, code_hash, attempts FROM account_codes
		WHERE account_id = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, accountID, purpose, s.Now().UTC()).Scan(&id, &email, &hash, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrBadCode
	}
	if err != nil {
		return "", err
	}
	if attempts >= maxCodeAttempts {
		return "", ErrBadCode
	}
	if !bytes.Equal(hash, digest(accountID, purpose, strings.TrimSpace(code))) {
		return "", &wrongCode{stmt: `UPDATE account_codes SET attempts = attempts + 1 WHERE id = $1`, arg: id}
	}
	_, err = tx.Exec(ctx, `UPDATE account_codes SET used_at = $2 WHERE id = $1`, id, s.Now().UTC())
	return email, err
}

func (s *Store) VerifyEmail(ctx context.Context, accountID, code string) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		email, err := s.useCode(ctx, tx, accountID, "verify_email", code)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE accounts SET email_verified_at = $3 WHERE id = $1 AND email = $2`, accountID, email, s.Now().UTC())
		return err
	})
	return s.recordWrong(ctx, err)
}

// RequestReset mails a one-time code valid 10 minutes when the account has a
// verified email. It never tells the caller whether the account exists or
// has an email (docs/09 §2): the caller always answers the same way.
func (s *Store) RequestReset(ctx context.Context, sec *Security, username string) error {
	if sec.Mail == nil {
		return ErrMailNotConfigured
	}
	var id string
	var email *string
	var verified *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT id::text, email, email_verified_at FROM accounts WHERE username_key = $1`,
		strings.ToLower(username)).Scan(&id, &email, &verified)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (email == nil || verified == nil)) {
		return nil
	}
	if err != nil {
		return err
	}
	code := sixDigits()
	now := s.Now().UTC()
	if _, err := s.Pool.Exec(ctx, `INSERT INTO account_codes (account_id, purpose, email, code_hash, created_at, expires_at)
		VALUES ($1, 'reset_password', $2, $3, $4, $5)`, id, *email, digest(id, "reset_password", code), now, now.Add(ResetCodeTTL)); err != nil {
		return err
	}
	return sec.Mail.Send(ctx, *email, "Project Thanh: đặt lại mật khẩu", "Mã đặt lại mật khẩu: "+code+" (hết hạn sau 10 phút). Bỏ qua thư này nếu bạn không yêu cầu.")
}

// ConfirmReset sets a new password and revokes every session of the account
// (P0-015): the generation bump invalidates them all, revoked_at records it.
func (s *Store) ConfirmReset(ctx context.Context, username, code, newPassword string) (string, error) {
	if !ValidPassword(newPassword) {
		return "", ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.BcryptCost)
	if err != nil {
		return "", err
	}
	var accountID string
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT id::text FROM accounts WHERE username_key = $1 FOR UPDATE`, strings.ToLower(username)).Scan(&accountID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBadCode
			}
			return err
		}
		if _, err := s.useCode(ctx, tx, accountID, "reset_password", code); err != nil {
			return err
		}
		now := s.Now().UTC()
		if _, err := tx.Exec(ctx, `UPDATE accounts SET password_hash = $2, login_generation = login_generation + 1, updated_at = $3 WHERE id = $1`,
			accountID, string(hash), now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at = $2 WHERE account_id = $1 AND revoked_at IS NULL`, accountID, now)
		return err
	})
	if err = s.recordWrong(ctx, err); err != nil {
		return "", err
	}
	return accountID, nil
}
