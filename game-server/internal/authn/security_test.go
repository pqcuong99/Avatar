package authn_test

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"example.com/project-thanh/game-server/internal/authn"
	"example.com/project-thanh/game-server/internal/secretbox"
)

type captureMail struct {
	mu   sync.Mutex
	last string
}

func (m *captureMail) Send(_ context.Context, _, _, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = body
	return nil
}

var sixDigits = regexp.MustCompile(`\d{6}`)

func (m *captureMail) code(t *testing.T) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := sixDigits.FindString(m.last)
	if c == "" {
		t.Fatalf("no code in %q", m.last)
	}
	return c
}

func newSecurity(t *testing.T) (*authn.Store, *authn.Security, *captureMail, *time.Time) {
	s := newStore(t)
	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	m := &captureMail{}
	return s, &authn.Security{Box: secretbox.DevBox(), Mail: m, Issuer: "Test"}, m, &now
}

func TestA10TOTPLoginNeedsTheSecondStep(t *testing.T) {
	s, sec, _, now := newSecurity(t)
	ctx := context.Background()
	id, _ := s.Register(ctx, "twofa_user", "password-123")
	// Without 2FA the password alone opens a session.
	if _, err := s.Login(ctx, "twofa_user", "password-123"); err != nil {
		t.Fatalf("plain login: %v", err)
	}
	enr, err := s.EnrollTOTP(ctx, sec, id, "twofa_user")
	if err != nil || enr.Secret == "" || enr.QRDataURL == "" {
		t.Fatalf("enroll: %+v %v", enr, err)
	}
	if _, err := s.EnableTOTP(ctx, sec, id, "000000"); !errors.Is(err, authn.ErrBadCode) {
		t.Fatalf("wrong enable code: %v", err)
	}
	code, _ := totp.GenerateCode(enr.Secret, *now)
	recovery, err := s.EnableTOTP(ctx, sec, id, code)
	if err != nil || len(recovery) != authn.RecoveryCodes {
		t.Fatalf("enable: %v %v", recovery, err)
	}

	_, err = s.Login(ctx, "twofa_user", "password-123")
	var ch *authn.TOTPChallenge
	if !errors.As(err, &ch) {
		t.Fatalf("login must ask for TOTP, got %v", err)
	}
	*now = now.Add(time.Minute)
	code, _ = totp.GenerateCode(enr.Secret, *now)
	sess, err := s.CompleteTOTP(ctx, sec, ch.Token, code)
	if err != nil {
		t.Fatalf("totp step: %v", err)
	}
	if _, err := s.Authenticate(ctx, sess.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteTOTP(ctx, sec, ch.Token, code); !errors.Is(err, authn.ErrBadCode) {
		t.Fatal("a challenge works once")
	}

	// A recovery code works once.
	_, err = s.Login(ctx, "twofa_user", "password-123")
	errors.As(err, &ch)
	if _, err := s.CompleteTOTP(ctx, sec, ch.Token, recovery[0]); err != nil {
		t.Fatalf("recovery code: %v", err)
	}
	_, err = s.Login(ctx, "twofa_user", "password-123")
	errors.As(err, &ch)
	if _, err := s.CompleteTOTP(ctx, sec, ch.Token, recovery[0]); !errors.Is(err, authn.ErrBadCode) {
		t.Fatal("recovery code reused")
	}
	// Five wrong tries burn the challenge, even for a right code afterwards.
	_, err = s.Login(ctx, "twofa_user", "password-123")
	errors.As(err, &ch)
	for i := 0; i < 5; i++ {
		_, _ = s.CompleteTOTP(ctx, sec, ch.Token, "111111")
	}
	code, _ = totp.GenerateCode(enr.Secret, *now)
	if _, err := s.CompleteTOTP(ctx, sec, ch.Token, code); !errors.Is(err, authn.ErrBadCode) {
		t.Fatal("challenge not burnt after 5 wrong codes")
	}
	// Disabling needs password and a code.
	if err := s.DisableTOTP(ctx, sec, id, "wrong-password", code); !errors.Is(err, authn.ErrInvalidCredentials) {
		t.Fatalf("disable with wrong password: %v", err)
	}
	if err := s.DisableTOTP(ctx, sec, id, "password-123", code); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := s.Login(ctx, "twofa_user", "password-123"); err != nil {
		t.Fatalf("login after disable: %v", err)
	}
}

func TestA08A09PasswordResetExpiresAndRevokesSessions(t *testing.T) {
	s, sec, mail, now := newSecurity(t)
	ctx := context.Background()
	id, _ := s.Register(ctx, "forgetful", "password-123")
	old, _ := s.Login(ctx, "forgetful", "password-123")

	// No verified email: the request answers the same, but nothing is sent.
	if err := s.RequestReset(ctx, sec, "forgetful"); err != nil {
		t.Fatal(err)
	}
	if mail.last != "" {
		t.Fatal("code sent to an unverified account")
	}
	if err := s.RequestReset(ctx, sec, "nobody_here"); err != nil {
		t.Fatal("unknown account must look the same")
	}

	if err := s.SetEmail(ctx, sec, id, "Player@Example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyEmail(ctx, id, mail.code(t)); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// A-08: a code used after 10 minutes is refused and changes nothing.
	_ = s.RequestReset(ctx, sec, "forgetful")
	late := mail.code(t)
	*now = now.Add(10*time.Minute + time.Second)
	if _, err := s.ConfirmReset(ctx, "forgetful", late, "new-password-1"); !errors.Is(err, authn.ErrBadCode) {
		t.Fatalf("expired code: %v", err)
	}
	if _, err := s.Login(ctx, "forgetful", "password-123"); err != nil {
		t.Fatal("password changed by an expired code")
	}
	old, _ = s.Login(ctx, "forgetful", "password-123")

	_ = s.RequestReset(ctx, sec, "forgetful")
	good := mail.code(t)
	if _, err := s.ConfirmReset(ctx, "forgetful", good, "new-password-1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	// A-09: the old cookie is revoked.
	if _, err := s.Authenticate(ctx, old.Token); !errors.Is(err, authn.ErrNoSession) {
		t.Fatalf("old session still valid: %v", err)
	}
	if _, err := s.ConfirmReset(ctx, "forgetful", good, "another-pass-2"); !errors.Is(err, authn.ErrBadCode) {
		t.Fatal("reset code reused")
	}
	if _, err := s.Login(ctx, "forgetful", "new-password-1"); err != nil {
		t.Fatalf("new password: %v", err)
	}
}

func TestEmailFeaturesOffWithoutAProvider(t *testing.T) {
	s, sec, _, _ := newSecurity(t)
	sec.Mail = nil
	id, _ := s.Register(context.Background(), "nomail", "password-123")
	if err := s.SetEmail(context.Background(), sec, id, "a@b.cd"); !errors.Is(err, authn.ErrMailNotConfigured) {
		t.Fatalf("got %v", err)
	}
	if err := s.RequestReset(context.Background(), sec, "nomail"); !errors.Is(err, authn.ErrMailNotConfigured) {
		t.Fatalf("got %v", err)
	}
}
