package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"example.com/project-thanh/game-server/internal/admin"
	"example.com/project-thanh/game-server/internal/secretbox"
	"example.com/project-thanh/game-server/internal/testenv"
)

type env struct {
	t     *testing.T
	store *admin.Store
}

func newEnv(t *testing.T) *env {
	pool := testenv.DB(t)
	s := admin.New(pool, secretbox.DevBox())
	s.BcryptCost = bcrypt.MinCost
	return &env{t: t, store: s}
}

// login creates an Admin and logs it in through both steps.
func (e *env) login(name string, roles ...string) *admin.Principal {
	e.t.Helper()
	ctx := context.Background()
	c, err := e.store.Create(ctx, name, roles)
	if err != nil {
		e.t.Fatal(err)
	}
	ch, err := e.store.StartLogin(ctx, name, c.Password)
	if err != nil {
		e.t.Fatal(err)
	}
	code, _ := totp.GenerateCode(c.TOTPSecret, time.Now())
	sess, err := e.store.FinishLogin(ctx, ch, code)
	if err != nil {
		e.t.Fatalf("finish login: %v", err)
	}
	p, err := e.store.Authenticate(ctx, sess.Token)
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *env) character() string {
	e.t.Helper()
	var acc string
	if err := e.store.Pool.QueryRow(context.Background(), `INSERT INTO accounts (username, username_key, password_hash) VALUES ('p1', 'p1', 'x') RETURNING id::text`).Scan(&acc); err != nil {
		e.t.Fatal(err)
	}
	return testenv.Character(e.t, e.store.Pool, acc, "Người Chơi")
}

func TestA11AdminLoginAlwaysNeedsTOTP(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c, err := e.store.Create(ctx, "ops", []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.StartLogin(ctx, "ops", "wrong"); !errors.Is(err, admin.ErrInvalidLogin) {
		t.Fatalf("wrong password: %v", err)
	}
	ch, err := e.store.StartLogin(ctx, "ops", c.Password)
	if err != nil {
		t.Fatal(err)
	}
	// The password alone gives no session.
	if _, err := e.store.Authenticate(ctx, ch); !errors.Is(err, admin.ErrNoSession) {
		t.Fatal("challenge usable as a session")
	}
	if _, err := e.store.FinishLogin(ctx, ch, "000000"); !errors.Is(err, admin.ErrInvalidLogin) {
		t.Fatalf("wrong code: %v", err)
	}
	if _, err := e.store.FinishLogin(ctx, ch, c.RecoveryCodes[0]); err != nil {
		t.Fatalf("recovery code: %v", err)
	}
	ch, _ = e.store.StartLogin(ctx, "ops", c.Password)
	if _, err := e.store.FinishLogin(ctx, ch, c.RecoveryCodes[0]); !errors.Is(err, admin.ErrInvalidLogin) {
		t.Fatal("recovery code reused")
	}
	if _, err := e.store.Create(ctx, "ops", []string{"SUPPORT"}); !errors.Is(err, admin.ErrLoginNameTaken) {
		t.Fatalf("duplicate login: %v", err)
	}
}

func TestT09T10TwoPersonAdjustments(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a1 := e.login("admin_one", "GAME_ADMIN")
	a2 := e.login("admin_two", "GAME_ADMIN")
	boss := e.login("boss", "SUPER_ADMIN")
	support := e.login("helper", "SUPPORT")
	char := e.character()

	if _, err := e.store.Propose(ctx, support, admin.Proposal{CharacterID: char, AssetKind: "XU", AmountDelta: 100, Reason: "đền bù lỗi"}); !errors.Is(err, admin.ErrForbidden) {
		t.Fatalf("support proposed: %v", err)
	}
	if _, err := e.store.Propose(ctx, a1, admin.Proposal{CharacterID: char, AssetKind: "XU", AmountDelta: 100, Reason: "x"}); !errors.Is(err, admin.ErrInvalidArgument) {
		t.Fatalf("short reason: %v", err)
	}
	adj, err := e.store.Propose(ctx, boss, admin.Proposal{CharacterID: char, AssetKind: "XU", AmountDelta: 100, Reason: "đền bù sự cố bảo trì"})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	exec := func(ctx context.Context, a *admin.Adjustment) (json.RawMessage, bool, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return json.RawMessage(`{"ok":true}`), true, nil
	}
	// T-09: not even a Super Admin approves their own request.
	if _, err := e.store.Decide(ctx, boss, adj.ID, true, "", exec); !errors.Is(err, admin.ErrSelfApproval) {
		t.Fatalf("self approval: %v", err)
	}
	if _, err := e.store.Decide(ctx, support, adj.ID, true, "", exec); !errors.Is(err, admin.ErrForbidden) {
		t.Fatalf("support approved: %v", err)
	}
	// T-10: two approvers at once -> exactly one execution.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, p := range []*admin.Principal{a1, a2} {
		wg.Add(1)
		go func(i int, p *admin.Principal) {
			defer wg.Done()
			_, errs[i] = e.store.Decide(ctx, p, adj.ID, true, "ok", exec)
		}(i, p)
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("executed %d times", calls.Load())
	}
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !errors.Is(err, admin.ErrNotPending) {
			t.Fatalf("unexpected %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d approvals succeeded", ok)
	}
	got, _ := e.store.Adjustment(ctx, adj.ID)
	if got.State != "executed" || got.Approver == nil || *got.Approver == "boss" {
		t.Fatalf("adjustment %+v", got)
	}
	// A refused execution is recorded as failed, not retried silently.
	adj2, _ := e.store.Propose(ctx, a1, admin.Proposal{CharacterID: char, AssetKind: "XU", AmountDelta: -1 << 40, Reason: "thu hồi nhầm"})
	fail := func(context.Context, *admin.Adjustment) (json.RawMessage, bool, error) {
		return json.RawMessage(`{"code":"INSUFFICIENT_FUNDS"}`), false, nil
	}
	got, err = e.store.Decide(ctx, a2, adj2.ID, true, "", fail)
	if err != nil || got.State != "failed" {
		t.Fatalf("failed execution: %+v %v", got, err)
	}
	log, _ := e.store.AuditLog(ctx, 0, 100)
	actions := map[string]int{}
	for _, l := range log {
		actions[l.Action]++
	}
	if actions["asset_adjustment.propose"] != 2 || actions["asset_adjustment.executed"] != 1 || actions["asset_adjustment.failed"] != 1 {
		t.Fatalf("audit %v", actions)
	}
	if _, err := e.store.Pool.Exec(ctx, `DELETE FROM admin_audit_logs`); err == nil {
		t.Fatal("audit log deletable")
	}
}

func TestO06PublishOnlyDuringMaintenance(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	ga := e.login("game_admin", "GAME_ADMIN")
	draft := &admin.Draft{Values: map[string]json.RawMessage{"world.room_capacity": json.RawMessage(`25`)}}
	id, err := e.store.SaveDraft(ctx, ga, 0, "V-TEST", "test", draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.Publish(ctx, ga, id); !errors.Is(err, admin.ErrMaintenanceOff) {
		t.Fatalf("publish outside maintenance: %v", err)
	}
	if err := e.store.SetMaintenance(ctx, ga, admin.Maintenance{Active: true, Message: "Cập nhật giá"}); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Publish(ctx, ga, id); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := e.store.SaveDraft(ctx, ga, id, "", "", draft); !errors.Is(err, admin.ErrInvalidDraft) {
		t.Fatalf("published version edited: %v", err)
	}
	back, err := e.store.ReadVersion(ctx, ga, id)
	if err != nil || string(back.Values["world.room_capacity"]) != "25" {
		t.Fatalf("read back: %+v %v", back, err)
	}
	support := e.login("viewer", "SUPPORT")
	if _, err := e.store.Versions(ctx, support); !errors.Is(err, admin.ErrForbidden) {
		t.Fatal("support saw config versions")
	}
}
