package authn_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"example.com/project-thanh/game-server/internal/authn"
	"example.com/project-thanh/game-server/internal/testenv"
)

func newStore(t *testing.T) *authn.Store {
	s := authn.NewStore(testenv.DB(t))
	s.BcryptCost = bcrypt.MinCost
	return s
}

func TestA01NewLoginInvalidatesTheOldSession(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "Thanh_01", "correct horse"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(ctx, "thanh_01", "other password"); !errors.Is(err, authn.ErrUsernameTaken) {
		t.Fatalf("case-insensitive duplicate: %v", err)
	}
	a, err := s.Login(ctx, "THANH_01", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, a.Token); err != nil {
		t.Fatalf("session A: %v", err)
	}
	b, err := s.Login(ctx, "thanh_01", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, a.Token); !errors.Is(err, authn.ErrNoSession) {
		t.Fatalf("A must be invalid after B logs in, got %v", err)
	}
	if p, err := s.Authenticate(ctx, b.Token); err != nil || p.Username != "Thanh_01" {
		t.Fatalf("session B: %+v %v", p, err)
	}
	// Logging out A (already superseded) does not touch B.
	_ = s.Logout(ctx, a.Token)
	if _, err := s.Authenticate(ctx, b.Token); err != nil {
		t.Fatalf("B after A logout: %v", err)
	}
	_ = s.Logout(ctx, b.Token)
	if _, err := s.Authenticate(ctx, b.Token); !errors.Is(err, authn.ErrNoSession) {
		t.Fatal("B still valid after logout")
	}
}

func TestLoginRejectsWrongPasswordUnknownUserAndBans(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id, _ := s.Register(ctx, "player1", "password-123")
	if _, err := s.Login(ctx, "player1", "wrong-password"); !errors.Is(err, authn.ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := s.Login(ctx, "nobody", "password-123"); !errors.Is(err, authn.ErrInvalidCredentials) {
		t.Fatalf("unknown user: %v", err)
	}
	sess, _ := s.Login(ctx, "player1", "password-123")
	if _, err := s.Pool.Exec(ctx, `UPDATE accounts SET status = 'banned' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, sess.Token); !errors.Is(err, authn.ErrBanned) {
		t.Fatalf("banned session: %v", err)
	}
	if _, err := s.Login(ctx, "player1", "password-123"); !errors.Is(err, authn.ErrBanned) {
		t.Fatalf("banned login: %v", err)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_, _ = s.Register(ctx, "player2", "password-123")
	sess, _ := s.Login(ctx, "player2", "password-123")
	s.Now = func() time.Time { return time.Now().Add(8 * 24 * time.Hour) }
	if _, err := s.Authenticate(ctx, sess.Token); !errors.Is(err, authn.ErrNoSession) {
		t.Fatalf("expired session: %v", err)
	}
}

func TestA12LimiterSlowsDownButNeverLocksForever(t *testing.T) {
	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	l := authn.NewLimiter()
	l.Now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow("1.1.1.1", "victim"); !ok {
			t.Fatalf("attempt %d blocked too early", i)
		}
		l.Failure("victim")
	}
	ok, wait := l.Allow("2.2.2.2", "victim")
	if ok || wait <= 0 || wait > time.Second {
		t.Fatalf("6th attempt: ok=%v wait=%v", ok, wait)
	}
	for i := 0; i < 30; i++ {
		l.Failure("victim")
	}
	_, wait = l.Allow("3.3.3.3", "victim")
	if wait > 5*time.Minute {
		t.Fatalf("delay %v exceeds the cap", wait)
	}
	now = now.Add(5 * time.Minute)
	if ok, _ := l.Allow("4.4.4.4", "victim"); !ok {
		t.Fatal("still locked after the maximum delay")
	}
	l.Success("victim")
	if ok, _ := l.Allow("5.5.5.5", "victim"); !ok {
		t.Fatal("success did not clear the delay")
	}
	// Per-IP window.
	l2 := authn.NewLimiter()
	l2.Now = func() time.Time { return now }
	for i := 0; i < l2.PerIPPerMinute; i++ {
		l2.Allow("9.9.9.9", "user")
	}
	if ok, _ := l2.Allow("9.9.9.9", "other"); ok {
		t.Fatal("per-IP limit not applied")
	}
}
