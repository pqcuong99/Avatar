package econ_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-thanh/transaction-service/internal/apperr"
	"example.com/project-thanh/transaction-service/internal/econ"
	"example.com/project-thanh/transaction-service/internal/testdb"
)

// stubRand: Float64 below a crop's pest probability means "pest"; Int64N
// returns intValue, capped at n-1, so spawn offsets and penalties are known.
type stubRand struct {
	mu       sync.Mutex
	float    float64
	intValue int64
}

func (r *stubRand) Float64() float64 { r.mu.Lock(); defer r.mu.Unlock(); return r.float }
func (r *stubRand) Int64N(n int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.intValue >= n {
		return n - 1
	}
	return r.intValue
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) Advance(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

type env struct {
	t     *testing.T
	pool  *pgxpool.Pool
	svc   *econ.Service
	clock *clock
	rand  *stubRand
	cfgV  int64
	n     int
}

var baseConfig = testdb.Config{
	Label: "TEST-1",
	Values: map[string]any{
		"wallet.xu_cap":      999999999,
		"wallet.diamond_cap": 1000,
		"inventory.upgrade":  map[string]any{"currency": "XU", "price": 1000, "slots_per_upgrade": 5, "max_slot_cap": 30},
		"farm.plot_unlock":   map[string]any{"currency": "XU", "price": 2000, "max_plots": 8},
	},
	Crops: []testdb.Crop{
		{Code: "carrot", GrowthSeconds: 1800, BaseYield: 120, PestProbability: 0.5},
		{Code: "potato", GrowthSeconds: 3600, BaseYield: 70, PestProbability: 0.5},
		{Code: "tomato", GrowthSeconds: 7200, BaseYield: 50, PestProbability: 0.5},
	},
	Offers: []testdb.Offer{
		{NPC: "FARM_SHOP", Item: "seed_carrot", Direction: "NPC_SELLS", Currency: "XU", Price: 100},
		{NPC: "FARM_SHOP", Item: "seed_potato", Direction: "NPC_SELLS", Currency: "XU", Price: 250},
		{NPC: "FARM_SHOP", Item: "seed_tomato", Direction: "NPC_SELLS", Currency: "XU", Price: 500},
		{NPC: "FARM_SHOP", Item: "crop_carrot", Direction: "NPC_BUYS", Currency: "XU", Price: 15},
		{NPC: "FARM_SHOP", Item: "crop_potato", Direction: "NPC_BUYS", Currency: "XU", Price: 30},
		{NPC: "FARM_SHOP", Item: "seed_cabbage", Direction: "NPC_SELLS", Currency: "DIAMOND", Price: 100},
	},
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	v := testdb.Publish(t, pool, baseConfig)
	svc, err := econ.New(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, pool: pool, svc: svc, cfgV: v,
		clock: &clock{t: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)},
		rand:  &stubRand{float: 0.99}} // no pests unless a test asks
	svc.Now = e.clock.Now
	svc.Rand = e.rand
	return e
}

func (e *env) character(name string) string {
	e.t.Helper()
	acc := testdb.CreateAccount(e.t, e.pool, "acc_"+fmt.Sprint(time.Now().UnixNano()))
	c, _, err := e.svc.CreateCharacter(context.Background(), acc, name, "create-"+acc[:8])
	if err != nil {
		e.t.Fatalf("create character: %v", err)
	}
	return c.ID
}

func (e *env) reqID() string {
	e.n++
	return fmt.Sprintf("req-%08d", e.n)
}

func (e *env) do(char, op string, args map[string]any) (json.RawMessage, error) {
	e.t.Helper()
	raw, _ := json.Marshal(args)
	out, err := e.svc.Command(context.Background(), char, e.reqID(), op, raw)
	if err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (e *env) must(char, op string, args map[string]any) json.RawMessage {
	e.t.Helper()
	res, err := e.do(char, op, args)
	if err != nil {
		e.t.Fatalf("%s %v: %v", op, args, err)
	}
	return res
}

func (e *env) wantCode(err error, code string) {
	e.t.Helper()
	biz, ok := apperr.As(err)
	if !ok || biz.Code != code {
		e.t.Fatalf("want %s, got %v", code, err)
	}
}

func (e *env) state(char string) *econ.State {
	e.t.Helper()
	st, err := e.svc.State(context.Background(), char)
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}

func (e *env) offerID(item, dir string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(context.Background(), `SELECT o.id::text FROM npc_offers o JOIN game_config_versions v ON v.id = o.config_version_id
		WHERE v.state = 'published' AND o.item_code = $1 AND o.direction = $2`, item, dir).Scan(&id); err != nil {
		e.t.Fatalf("offer %s %s: %v", item, dir, err)
	}
	return id
}

func (e *env) buy(char, item string, qty int64) {
	e.t.Helper()
	e.must(char, "npc.buy", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID(item, "NPC_SELLS"), "quantity": qty, "config_version": e.cfgV})
}

func (e *env) count(char, item string) int64 {
	return testdb.Int64(e.t, e.pool, `SELECT COALESCE(SUM(quantity), 0) FROM inventory_slots WHERE character_id = $1 AND item_code = $2`, char, item)
}

func (e *env) xu(char string) int64 {
	return testdb.Int64(e.t, e.pool, `SELECT balance FROM character_wallets WHERE character_id = $1 AND currency = 'XU'`, char)
}

// fillBag puts a full stack of item into every slot from fromSlot to toSlot.
func (e *env) fillBag(char string, fromSlot, toSlot int, item string, qty int) {
	for no := fromSlot; no <= toSlot; no++ {
		testdb.Exec(e.t, e.pool, `INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity) VALUES ($1, $2, $3, $4)`, char, no, item, qty)
	}
}

func (e *env) pending(char, crop string) (int64, time.Time) {
	var q int64
	var ts time.Time
	err := e.pool.QueryRow(context.Background(), `SELECT quantity, earliest_created_at FROM pending_crops WHERE character_id = $1 AND crop_code = $2`, char, crop).Scan(&q, &ts)
	if err != nil {
		return 0, time.Time{}
	}
	return q, ts.UTC()
}

// ---------------------------------------------------------------------------
// Character creation (M1b)
// ---------------------------------------------------------------------------

func TestCreateCharacterGrantsStarterAssets(t *testing.T) {
	e := newEnv(t)
	c := e.character("Thành")
	st := e.state(c)
	if st.Wallets["XU"].Balance != 5000 || st.Wallets["DIAMOND"].Balance != 0 {
		t.Fatalf("wallets %+v", st.Wallets)
	}
	if *st.Wallets["XU"].Cap != 999999999 {
		t.Fatalf("xu cap %d", *st.Wallets["XU"].Cap)
	}
	if st.Inventory.SlotCap != 10 || len(st.Inventory.Slots) != 0 {
		t.Fatalf("inventory %+v", st.Inventory)
	}
	if len(st.Farm.Plots) != 6 || st.Farm.Plots[0].PlotNo != 1 || st.Farm.Plots[5].PlotNo != 6 {
		t.Fatalf("plots %+v", st.Farm.Plots)
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM asset_ledger WHERE character_id = $1 AND reason_code = 'STARTER_GRANT' AND amount_delta = 5000`, c); n != 1 {
		t.Fatalf("starter ledger rows %d", n)
	}
}

func TestA02ConcurrentCreateMakesExactlyOneCharacter(t *testing.T) {
	e := newEnv(t)
	acc := testdb.CreateAccount(t, e.pool, "racer")
	var wg sync.WaitGroup
	results := make([]error, 10)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, results[i] = e.svc.CreateCharacter(context.Background(), acc, fmt.Sprintf("Racer %d", i), fmt.Sprintf("create-%04d", i))
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		} else if biz, isBiz := apperr.As(err); !isBiz || biz.Code != "CHARACTER_ALREADY_EXISTS" {
			t.Fatalf("unexpected error %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d creations succeeded", ok)
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM characters WHERE account_id = $1`, acc); n != 1 {
		t.Fatalf("%d characters", n)
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM asset_ledger l JOIN characters c ON c.id = l.character_id WHERE c.account_id = $1`, acc); n != 1 {
		t.Fatalf("%d starter grants", n)
	}
}

func TestCreateCharacterReplayAndUniqueness(t *testing.T) {
	e := newEnv(t)
	acc := testdb.CreateAccount(t, e.pool, "first")
	c1, replayed, err := e.svc.CreateCharacter(context.Background(), acc, "Thanh", "create-req-1")
	if err != nil || replayed {
		t.Fatalf("create: %v %v", err, replayed)
	}
	c2, replayed, err := e.svc.CreateCharacter(context.Background(), acc, "Thanh", "create-req-1")
	if err != nil || !replayed || c2.ID != c1.ID {
		t.Fatalf("replay: %v %v %+v", err, replayed, c2)
	}
	_, _, err = e.svc.CreateCharacter(context.Background(), acc, "Other", "create-req-2")
	e.wantCode(err, "CHARACTER_ALREADY_EXISTS")

	other := testdb.CreateAccount(t, e.pool, "second")
	_, _, err = e.svc.CreateCharacter(context.Background(), other, "thanh", "create-req-3") // A-03
	e.wantCode(err, "CHARACTER_NAME_TAKEN")
	_, _, err = e.svc.CreateCharacter(context.Background(), other, "THANH", "create-req-4")
	e.wantCode(err, "CHARACTER_NAME_TAKEN")
	if _, _, err = e.svc.CreateCharacter(context.Background(), other, "Thành", "create-req-5"); err != nil { // A-04
		t.Fatalf("Thành must be allowed next to Thanh: %v", err)
	}
	third := testdb.CreateAccount(t, e.pool, "third")
	_, _, err = e.svc.CreateCharacter(context.Background(), third, "Tha\U00000300nh", "create-req-6") // A-05, decomposed
	e.wantCode(err, "CHARACTER_NAME_TAKEN")
	_, _, err = e.svc.CreateCharacter(context.Background(), third, "ab", "create-req-7") // A-06
	e.wantCode(err, "CHARACTER_NAME_INVALID")
}

// ---------------------------------------------------------------------------
// Wallet, idempotency, ledger (M3a)
// ---------------------------------------------------------------------------

func TestT01BuyDebitsAndGrantsWithLedger(t *testing.T) {
	e := newEnv(t)
	c := e.character("Buyer")
	testdb.Exec(t, e.pool, `UPDATE character_wallets SET balance = 500 WHERE character_id = $1 AND currency = 'DIAMOND'`, c)
	e.must(c, "npc.buy", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("seed_cabbage", "NPC_SELLS"), "quantity": 1, "config_version": e.cfgV})
	st := e.state(c)
	if st.Wallets["DIAMOND"].Balance != 400 || e.count(c, "seed_cabbage") != 1 {
		t.Fatalf("diamond %d cabbage seeds %d", st.Wallets["DIAMOND"].Balance, e.count(c, "seed_cabbage"))
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM asset_ledger WHERE character_id = $1 AND reason_code = 'NPC_BUY'`, c); n != 2 {
		t.Fatalf("ledger rows %d", n)
	}
	if n := testdb.Int64(t, e.pool, `SELECT balance_after FROM asset_ledger WHERE character_id = $1 AND asset_kind = 'DIAMOND'`, c); n != 400 {
		t.Fatalf("balance_after %d", n)
	}
}

func TestBuyRejectsWithoutPartialEffects(t *testing.T) {
	e := newEnv(t)
	c := e.character("Poor")
	_, err := e.do(c, "npc.buy", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("seed_carrot", "NPC_SELLS"), "quantity": 51, "config_version": e.cfgV})
	e.wantCode(err, "INSUFFICIENT_FUNDS")
	// Bag full: money must not move either (T-04 shape: both or neither).
	e.fillBag(c, 1, 10, "crop_potato", 999)
	_, err = e.do(c, "npc.buy", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("seed_carrot", "NPC_SELLS"), "quantity": 1, "config_version": e.cfgV})
	e.wantCode(err, "INVENTORY_CAPACITY_INSUFFICIENT")
	if e.xu(c) != 5000 {
		t.Fatalf("xu changed to %d", e.xu(c))
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM asset_ledger WHERE character_id = $1 AND reason_code <> 'STARTER_GRANT'`, c); n != 0 {
		t.Fatalf("%d ledger rows for rejected commands", n)
	}
}

func TestT02ConcurrentRetriesExecuteOnce(t *testing.T) {
	e := newEnv(t)
	c := e.character("Retry")
	args, _ := json.Marshal(map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("seed_carrot", "NPC_SELLS"), "quantity": 3, "config_version": e.cfgV})
	var wg sync.WaitGroup
	results := make([]*econ.Outcome, 100)
	errs := make([]error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = e.svc.Command(context.Background(), c, "same-request-x", "npc.buy", args)
		}(i)
	}
	wg.Wait()
	fresh := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		if !results[i].Replayed {
			fresh++
		}
		if string(results[i].Result) != string(results[0].Result) {
			t.Fatalf("different results %s vs %s", results[i].Result, results[0].Result)
		}
	}
	if fresh != 1 {
		t.Fatalf("%d executions", fresh)
	}
	if e.xu(c) != 4700 || e.count(c, "seed_carrot") != 3 {
		t.Fatalf("xu %d seeds %d", e.xu(c), e.count(c, "seed_carrot"))
	}
}

func TestT03SameRequestDifferentPayloadConflicts(t *testing.T) {
	e := newEnv(t)
	c := e.character("Conflict")
	offer := e.offerID("seed_carrot", "NPC_SELLS")
	a, _ := json.Marshal(map[string]any{"npc_code": "FARM_SHOP", "offer_id": offer, "quantity": 1, "config_version": e.cfgV})
	b, _ := json.Marshal(map[string]any{"npc_code": "FARM_SHOP", "offer_id": offer, "quantity": 2, "config_version": e.cfgV})
	if _, err := e.svc.Command(context.Background(), c, "request-xyz", "npc.buy", a); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.Command(context.Background(), c, "request-xyz", "npc.buy", b)
	e.wantCode(err, "IDEMPOTENCY_CONFLICT")
	_, err = e.svc.Command(context.Background(), c, "request-xyz", "npc.sell", a)
	e.wantCode(err, "IDEMPOTENCY_CONFLICT")
}

func TestRejectionIsReplayedNotReexecuted(t *testing.T) {
	e := newEnv(t)
	c := e.character("Replay")
	args, _ := json.Marshal(map[string]any{"plot_no": 1, "crop_code": "carrot"})
	_, err := e.svc.Command(context.Background(), c, "plant-once-1", "farm.plant", args)
	e.wantCode(err, "ITEM_INSUFFICIENT")
	e.buy(c, "seed_carrot", 1)
	// Same request id: the stored rejection comes back even though it would now succeed.
	_, err = e.svc.Command(context.Background(), c, "plant-once-1", "farm.plant", args)
	e.wantCode(err, "ITEM_INSUFFICIENT")
	if e.count(c, "seed_carrot") != 1 {
		t.Fatal("seed consumed by a replay")
	}
}

func TestLedgerAndRequestsAreAppendOnly(t *testing.T) {
	e := newEnv(t)
	c := e.character("Ledger")
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE asset_ledger SET amount_delta = 1 WHERE character_id = $1`, c); err == nil {
		t.Fatal("ledger UPDATE allowed")
	}
	if _, err := e.pool.Exec(ctx, `DELETE FROM asset_ledger WHERE character_id = $1`, c); err == nil {
		t.Fatal("ledger DELETE allowed")
	}
	if _, err := e.pool.Exec(ctx, `TRUNCATE asset_ledger CASCADE`); err == nil {
		t.Fatal("ledger TRUNCATE allowed")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE transaction_requests SET state = 'rejected' WHERE character_id = $1`, c); err == nil {
		t.Fatal("transaction_requests UPDATE allowed")
	}
}

func TestPublishedConfigIsImmutable(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE npc_offers SET unit_price = 1 WHERE config_version_id = $1`, e.cfgV); err == nil {
		t.Fatal("published offer price changed")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE crop_configs SET base_yield = 1 WHERE config_version_id = $1`, e.cfgV); err == nil {
		t.Fatal("published crop config changed")
	}
	if _, err := e.pool.Exec(ctx, `INSERT INTO game_config_values VALUES ($1, 'x', '1')`, e.cfgV); err == nil {
		t.Fatal("value added to a published version")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE game_config_versions SET state = 'draft' WHERE id = $1`, e.cfgV); err == nil {
		t.Fatal("published version went back to draft")
	}
}

func TestSellToNPCAndWalletCap(t *testing.T) {
	e := newEnv(t)
	c := e.character("Seller")
	testdb.Exec(t, e.pool, `INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity) VALUES ($1, 3, 'crop_carrot', 40)`, c)
	e.must(c, "npc.sell", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("crop_carrot", "NPC_BUYS"), "quantity": 40, "config_version": e.cfgV})
	if e.xu(c) != 5000+600 || e.count(c, "crop_carrot") != 0 {
		t.Fatalf("xu %d carrots %d", e.xu(c), e.count(c, "crop_carrot"))
	}
	testdb.Exec(t, e.pool, `UPDATE character_wallets SET balance = 999999990 WHERE character_id = $1 AND currency = 'XU'`, c)
	testdb.Exec(t, e.pool, `INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity) VALUES ($1, 3, 'crop_carrot', 1)`, c)
	_, err := e.do(c, "npc.sell", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("crop_carrot", "NPC_BUYS"), "quantity": 1, "config_version": e.cfgV})
	e.wantCode(err, "WALLET_CAP_EXCEEDED") // GAP-01: refused, never destroyed
	if e.count(c, "crop_carrot") != 1 {
		t.Fatal("crop lost on refused sale")
	}
}

// ---------------------------------------------------------------------------
// NPC price versions (M3b)
// ---------------------------------------------------------------------------

func TestT06DraftPricesAreIgnored(t *testing.T) {
	e := newEnv(t)
	c := e.character("Drafty")
	testdb.Draft(t, e.pool, testdb.Config{Label: "TEST-DRAFT", Offers: []testdb.Offer{
		{NPC: "FARM_SHOP", Item: "seed_carrot", Direction: "NPC_SELLS", Currency: "XU", Price: 1},
	}})
	view, err := e.svc.NPCOffers(context.Background(), c, "FARM_SHOP")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range view.Offers {
		if o.ItemCode == "seed_carrot" && o.UnitPrice != 100 {
			t.Fatalf("draft price leaked: %d", o.UnitPrice)
		}
	}
	e.buy(c, "seed_carrot", 1)
	if e.xu(c) != 4900 {
		t.Fatalf("xu %d", e.xu(c))
	}
}

func TestT07T11NewPublishedPriceAppliesOnlyAfterPublish(t *testing.T) {
	e := newEnv(t)
	c := e.character("Prices")
	e.buy(c, "seed_carrot", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	oldOffer := e.offerID("crop_carrot", "NPC_BUYS")
	before := e.xu(c)

	// Maintenance publishes v2: new sell price and new yield.
	v2 := baseConfig
	v2.Label = "TEST-2"
	v2.Crops = []testdb.Crop{{Code: "carrot", GrowthSeconds: 600, BaseYield: 3, PestProbability: 0}}
	v2.Offers = []testdb.Offer{{NPC: "FARM_SHOP", Item: "crop_carrot", Direction: "NPC_BUYS", Currency: "XU", Price: 20}}
	newV := testdb.Publish(t, e.pool, v2)

	// The client still shows v1: the server refuses instead of silently applying a new price.
	_, err := e.do(c, "npc.sell", map[string]any{"npc_code": "FARM_SHOP", "offer_id": oldOffer, "quantity": 1, "config_version": e.cfgV})
	e.wantCode(err, "CONFIG_VERSION_OUTDATED")

	e.clock.Advance(31 * time.Minute)
	res := e.must(c, "farm.harvest", map[string]any{"plot_no": 1})
	var h econ.HarvestResult
	_ = json.Unmarshal(res, &h)
	if h.Quantity != 120 { // F-02: yield snapshot from v1, not 3
		t.Fatalf("harvested %d", h.Quantity)
	}
	e.must(c, "npc.sell", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("crop_carrot", "NPC_BUYS"), "quantity": 10, "config_version": newV})
	if e.xu(c) != before+200 { // T-07: newest published price
		t.Fatalf("xu %d, want %d", e.xu(c), before+200)
	}
	// T-11: the earlier purchase's ledger row is untouched.
	if n := testdb.Int64(t, e.pool, `SELECT amount_delta FROM asset_ledger WHERE character_id = $1 AND reason_code = 'NPC_BUY' AND asset_kind = 'XU'`, c); n != -100 {
		t.Fatalf("old purchase changed: %d", n)
	}
}

func TestT08CannotSellFromPending(t *testing.T) {
	e := newEnv(t)
	c := e.character("PendSell")
	testdb.Exec(t, e.pool, `INSERT INTO pending_crops (character_id, crop_code, quantity, earliest_created_at) VALUES ($1, 'carrot', 50, now())`, c)
	_, err := e.do(c, "npc.sell", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("crop_carrot", "NPC_BUYS"), "quantity": 1, "config_version": e.cfgV})
	e.wantCode(err, "ITEM_INSUFFICIENT")
	biz, _ := apperr.As(err)
	if biz.Details["in_pending"] != int64(50) {
		t.Fatalf("details %+v", biz.Details)
	}
	if q, _ := e.pending(c, "carrot"); q != 50 {
		t.Fatalf("pending %d", q)
	}
}

// ---------------------------------------------------------------------------
// Farm (M4)
// ---------------------------------------------------------------------------

func TestF01PlantConsumesExactlyOneSeed(t *testing.T) {
	e := newEnv(t)
	c := e.character("Planter")
	e.buy(c, "seed_carrot", 2)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	if e.count(c, "seed_carrot") != 1 {
		t.Fatalf("seeds %d", e.count(c, "seed_carrot"))
	}
	st := e.state(c)
	p := st.Farm.Plots[0].Planting
	if p == nil || p.CropCode != "carrot" || !p.MaturesAt.Equal(e.clock.Now().Add(30*time.Minute)) {
		t.Fatalf("planting %+v", p)
	}
	_, err := e.do(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	e.wantCode(err, "PLOT_OCCUPIED")
	_, err = e.do(c, "farm.plant", map[string]any{"plot_no": 7, "crop_code": "carrot"})
	e.wantCode(err, "PLOT_LOCKED")
	_, err = e.do(c, "farm.plant", map[string]any{"plot_no": 2, "crop_code": "corn"}) // corn not configured in TEST-1
	e.wantCode(err, "FEATURE_NOT_CONFIGURED")
	if e.count(c, "seed_carrot") != 1 {
		t.Fatal("rejected plant consumed a seed")
	}
}

func TestF03MatureCropNeverWithers(t *testing.T) {
	e := newEnv(t)
	c := e.character("Patient")
	e.buy(c, "seed_carrot", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 2, "crop_code": "carrot"})
	_, err := e.do(c, "farm.harvest", map[string]any{"plot_no": 2})
	e.wantCode(err, "PLOT_NOT_READY")
	e.clock.Advance(30*time.Minute + 3*time.Hour)
	st := e.state(c)
	if !st.Farm.Plots[1].Planting.Mature {
		t.Fatal("not mature")
	}
	e.must(c, "farm.harvest", map[string]any{"plot_no": 2})
	if e.count(c, "crop_carrot") != 120 {
		t.Fatalf("carrots %d", e.count(c, "crop_carrot"))
	}
	_, err = e.do(c, "farm.harvest", map[string]any{"plot_no": 2}) // harvested once only
	e.wantCode(err, "PLOT_EMPTY")
}

func TestF04WholeLotGoesToPendingWhenItDoesNotFit(t *testing.T) {
	e := newEnv(t)
	c := e.character("Overflow")
	e.buy(c, "seed_carrot", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	e.fillBag(c, 1, 9, "crop_potato", 999)
	testdb.Exec(t, e.pool, `INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity) VALUES ($1, 10, 'crop_carrot', 949)`, c)
	e.clock.Advance(30 * time.Minute)
	res := e.must(c, "farm.harvest", map[string]any{"plot_no": 1})
	var h econ.HarvestResult
	_ = json.Unmarshal(res, &h)
	if h.Destination != "PENDING" || h.Quantity != 120 {
		t.Fatalf("result %+v", h)
	}
	if e.count(c, "crop_carrot") != 949 {
		t.Fatalf("bag got part of the lot: %d", e.count(c, "crop_carrot"))
	}
	if q, _ := e.pending(c, "carrot"); q != 120 {
		t.Fatalf("pending %d", q)
	}
}

func TestF05PendingLotsMergeKeepingEarliestTime(t *testing.T) {
	e := newEnv(t)
	c := e.character("Merger")
	// Two carrot plantings with yields 50 and 70, from two config versions.
	v := baseConfig
	v.Label = "TEST-50"
	v.Crops = []testdb.Crop{{Code: "carrot", GrowthSeconds: 1800, BaseYield: 50, PestProbability: 0}}
	e.cfgV = testdb.Publish(t, e.pool, v)
	e.buy(c, "seed_carrot", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	v.Label = "TEST-70"
	v.Crops = []testdb.Crop{{Code: "carrot", GrowthSeconds: 1800, BaseYield: 70, PestProbability: 0}}
	e.cfgV = testdb.Publish(t, e.pool, v)
	e.clock.Advance(time.Minute)
	e.buy(c, "seed_carrot", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 2, "crop_code": "carrot"})
	e.fillBag(c, 1, 10, "crop_potato", 999) // no room for carrots

	e.clock.Advance(30 * time.Minute)
	first := e.clock.Now()
	e.must(c, "farm.harvest", map[string]any{"plot_no": 1})
	e.clock.Advance(time.Minute)
	e.must(c, "farm.harvest", map[string]any{"plot_no": 2})
	q, ts := e.pending(c, "carrot")
	if q != 120 || !ts.Equal(first) {
		t.Fatalf("pending %d at %v, want 120 at %v", q, ts, first)
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM pending_crops WHERE character_id = $1`, c); n != 1 {
		t.Fatalf("%d pending lines", n)
	}
}

func TestF06F07ClaimWholeLinesFIFO(t *testing.T) {
	e := newEnv(t)
	c := e.character("Claimer")
	t0 := e.clock.Now()
	testdb.Exec(t, e.pool, `INSERT INTO pending_crops (character_id, crop_code, quantity, earliest_created_at) VALUES
		($1, 'carrot', 100, $2), ($1, 'potato', 2000, $3), ($1, 'tomato', 10, $4)`, c, t0, t0.Add(time.Minute), t0.Add(2*time.Minute))
	// Only one free slot: 999 units of any crop.
	e.fillBag(c, 2, 10, "seed_corn", 999)
	_, err := e.do(c, "pending.crop.claim", map[string]any{"crop_code": "potato"}) // F-06
	e.wantCode(err, "PENDING_WHOLE_BATCH_REQUIRED")
	if q, _ := e.pending(c, "potato"); q != 2000 {
		t.Fatal("refused claim changed the line")
	}
	res := e.must(c, "pending.crop.claim_all", nil) // F-07
	var r econ.ClaimResult
	_ = json.Unmarshal(res, &r)
	if len(r.Claimed) != 1 || r.Claimed[0].CropCode != "carrot" || r.StoppedAt == nil || *r.StoppedAt != "potato" {
		t.Fatalf("claim all %+v", r)
	}
	if q, _ := e.pending(c, "tomato"); q != 10 {
		t.Fatal("line after the stop was touched")
	}
	if e.count(c, "crop_carrot") != 100 {
		t.Fatal("carrots not in bag")
	}
}

func TestF08PartialClaimWhenLineExceedsMaxPossibleBag(t *testing.T) {
	e := newEnv(t)
	c := e.character("Huge")
	// max_slot_cap 30 * 999 = 29,970 < 40,000
	testdb.Exec(t, e.pool, `INSERT INTO pending_crops (character_id, crop_code, quantity, earliest_created_at) VALUES ($1, 'carrot', 40000, now())`, c)
	e.fillBag(c, 3, 10, "seed_corn", 999) // 2 free slots = 1998
	res := e.must(c, "pending.crop.claim", map[string]any{"crop_code": "carrot"})
	var r econ.ClaimResult
	_ = json.Unmarshal(res, &r)
	if len(r.Claimed) != 1 || r.Claimed[0].Quantity != 1998 || !r.Claimed[0].Partial || r.Claimed[0].Remaining != 38002 {
		t.Fatalf("claim %+v", r)
	}
	if q, _ := e.pending(c, "carrot"); q != 38002 {
		t.Fatalf("pending %d", q)
	}
	// Bag full now: the exception claims nothing and reports no success.
	_, err := e.do(c, "pending.crop.claim", map[string]any{"crop_code": "carrot"})
	e.wantCode(err, "PENDING_WHOLE_BATCH_REQUIRED")
}

func TestF09HarvestAllContinuesPastAnOverflowingPlot(t *testing.T) {
	e := newEnv(t)
	c := e.character("Harvester")
	e.buy(c, "seed_carrot", 1)
	e.buy(c, "seed_potato", 2)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "potato"}) // 70
	e.must(c, "farm.plant", map[string]any{"plot_no": 2, "crop_code": "carrot"}) // 120
	e.must(c, "farm.plant", map[string]any{"plot_no": 3, "crop_code": "potato"}) // 70
	// Free space: room for 899 more potatoes in slot 1 and nothing else.
	testdb.Exec(t, e.pool, `DELETE FROM inventory_slots WHERE character_id = $1`, c)
	testdb.Exec(t, e.pool, `INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity) VALUES ($1, 1, 'crop_potato', 100)`, c)
	e.fillBag(c, 2, 10, "seed_corn", 999)
	e.clock.Advance(2 * time.Hour)
	res := e.must(c, "farm.harvest_all", nil)
	var r econ.HarvestAllResult
	_ = json.Unmarshal(res, &r)
	if len(r.Harvests) != 3 {
		t.Fatalf("%+v", r)
	}
	want := []string{"BAG", "PENDING", "BAG"}
	for i, h := range r.Harvests {
		if h.PlotNo != i+1 || h.Destination != want[i] {
			t.Fatalf("harvest %d: %+v", i, h)
		}
	}
}

func TestF10UprootAllNeedsConfirmationAndTakesEverything(t *testing.T) {
	e := newEnv(t)
	c := e.character("Uprooter")
	e.buy(c, "seed_carrot", 1)
	e.buy(c, "seed_tomato", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	e.must(c, "farm.plant", map[string]any{"plot_no": 4, "crop_code": "tomato"})
	e.clock.Advance(40 * time.Minute) // carrot mature, tomato young
	_, err := e.do(c, "farm.uproot_all", map[string]any{"confirmed": false})
	e.wantCode(err, "CONFIRMATION_REQUIRED")
	_, err = e.do(c, "farm.uproot", map[string]any{"plot_no": 1})
	e.wantCode(err, "CONFIRMATION_REQUIRED")
	res := e.must(c, "farm.uproot_all", map[string]any{"confirmed": true})
	var r econ.UprootResult
	_ = json.Unmarshal(res, &r)
	if len(r.Uprooted) != 2 {
		t.Fatalf("%+v", r)
	}
	if e.count(c, "crop_carrot") != 0 || e.count(c, "seed_carrot") != 0 {
		t.Fatal("uproot refunded something")
	}
}

func TestF11PlantAllUsesIncreasingPlotsUntilSeedsRunOut(t *testing.T) {
	e := newEnv(t)
	c := e.character("PlantAll")
	e.buy(c, "seed_carrot", 1)
	e.buy(c, "seed_potato", 3)
	e.must(c, "farm.plant", map[string]any{"plot_no": 2, "crop_code": "carrot"})
	_, err := e.do(c, "farm.plant_all", map[string]any{"crop_code": "potato"})
	e.wantCode(err, "CONFIRMATION_REQUIRED")
	res := e.must(c, "farm.plant_all", map[string]any{"crop_code": "potato", "confirmed": true})
	var r econ.PlantAllResult
	_ = json.Unmarshal(res, &r)
	if r.SeedsUsed != 3 || r.Planted[0].PlotNo != 1 || r.Planted[1].PlotNo != 3 || r.Planted[2].PlotNo != 4 {
		t.Fatalf("%+v", r)
	}
	if e.count(c, "seed_potato") != 0 {
		t.Fatal("seeds left")
	}
}

func TestF12F13F14PestLifecycle(t *testing.T) {
	e := newEnv(t)
	c := e.character("Pesty")
	e.rand.float = 0          // pest
	e.rand.intValue = 1 << 62 // latest spawn (70%), penalty 10%
	e.buy(c, "seed_carrot", 2)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	planted := e.clock.Now()

	// Before the spawn the client sees nothing.
	if st := e.state(c); st.Farm.Plots[0].Planting.Pest != nil {
		t.Fatal("future pest leaked")
	}
	_, err := e.do(c, "farm.catch_pest", map[string]any{"plot_no": 1})
	e.wantCode(err, "PEST_NOT_ACTIVE")

	// F-14: a restart (new service instance, other randomness) does not roll again.
	svc2, _ := econ.New(context.Background(), e.pool)
	svc2.Now, svc2.Rand = e.clock.Now, &stubRand{float: 0.99}
	e.clock.Advance(21 * time.Minute)
	st, _ := svc2.State(context.Background(), c)
	pest := st.Farm.Plots[0].Planting.Pest
	if pest == nil || !pest.Active || !pest.SpawnAt.Equal(planted.Add(21*time.Minute)) || !pest.DeadlineAt.Equal(planted.Add(30*time.Minute)) {
		t.Fatalf("F-12 pest %+v", pest)
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM pest_incidents`); n != 1 {
		t.Fatalf("%d incidents", n)
	}

	// Missed: the harvest loses 10%: 120 - floor(12) = 108 (F-13).
	e.clock.Advance(9 * time.Minute)
	_, err = e.do(c, "farm.catch_pest", map[string]any{"plot_no": 1})
	e.wantCode(err, "PEST_NOT_ACTIVE")
	res := e.must(c, "farm.harvest", map[string]any{"plot_no": 1})
	var h econ.HarvestResult
	_ = json.Unmarshal(res, &h)
	if h.Quantity != 108 || h.PenaltyPercent != 10 {
		t.Fatalf("harvest %+v", h)
	}

	// Caught in time: full yield and one worm.
	e.rand.intValue = 0 // spawn right away, penalty 5
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	e.must(c, "farm.catch_pest", map[string]any{"plot_no": 1})
	if e.count(c, "worm") != 1 {
		t.Fatal("no worm")
	}
	_, err = e.do(c, "farm.catch_pest", map[string]any{"plot_no": 1})
	e.wantCode(err, "PEST_NOT_ACTIVE")
	e.clock.Advance(30 * time.Minute)
	res = e.must(c, "farm.harvest", map[string]any{"plot_no": 1})
	h = econ.HarvestResult{}
	_ = json.Unmarshal(res, &h)
	if h.Quantity != 120 || h.PenaltyPercent != 0 {
		t.Fatalf("caught harvest %+v", h)
	}
}

func TestF17CatchPestWithFullBagIsBlocked(t *testing.T) {
	e := newEnv(t)
	c := e.character("FullBag")
	e.rand.float, e.rand.intValue = 0, 0
	e.buy(c, "seed_carrot", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	e.fillBag(c, 1, 10, "crop_potato", 999)
	_, err := e.do(c, "farm.catch_pest", map[string]any{"plot_no": 1})
	e.wantCode(err, "INVENTORY_CAPACITY_INSUFFICIENT")
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM pest_incidents WHERE caught_at IS NULL`); n != 1 {
		t.Fatal("pest state changed")
	}
}

func TestUpgradesUseConfigAndLimits(t *testing.T) {
	e := newEnv(t)
	c := e.character("Upgrader")
	res := e.must(c, "farm.unlock_plot", nil)
	var u econ.UnlockPlotResult
	_ = json.Unmarshal(res, &u)
	if u.PlotNo != 7 || e.xu(c) != 3000 {
		t.Fatalf("unlock %+v xu %d", u, e.xu(c))
	}
	e.must(c, "inventory.upgrade", nil)
	if st := e.state(c); st.Inventory.SlotCap != 15 || e.xu(c) != 2000 {
		t.Fatalf("slot cap %d xu %d", st.Inventory.SlotCap, e.xu(c))
	}
	e.must(c, "farm.unlock_plot", nil) // plot 8 = max
	_, err := e.do(c, "farm.unlock_plot", nil)
	e.wantCode(err, "UPGRADE_LIMIT_REACHED")
}

func TestUnconfiguredFeaturesStayDisabled(t *testing.T) {
	e := newEnv(t)
	c := e.character("Unconf")
	testdb.Publish(t, e.pool, testdb.Config{Label: "TEST-BARE"}) // nothing configured
	_, err := e.do(c, "farm.unlock_plot", nil)
	e.wantCode(err, "FEATURE_NOT_CONFIGURED")
	_, err = e.do(c, "inventory.upgrade", nil)
	e.wantCode(err, "FEATURE_NOT_CONFIGURED")
	testdb.Exec(t, e.pool, `INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity) VALUES ($1, 1, 'seed_carrot', 1)`, c)
	_, err = e.do(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	e.wantCode(err, "FEATURE_NOT_CONFIGURED")
}

func TestPendingDiamondsFIFOWholeEntries(t *testing.T) {
	e := newEnv(t)
	c := e.character("Gems")
	// cap 1000, balance 900: room for 100.
	testdb.Exec(t, e.pool, `UPDATE character_wallets SET balance = 900 WHERE character_id = $1 AND currency = 'DIAMOND'`, c)
	t0 := e.clock.Now()
	testdb.Exec(t, e.pool, `INSERT INTO pending_diamond_awards (character_id, source_kind, source_ref, quantity, remaining, created_at) VALUES
		($1, 'quest', 'q1', 60, 60, $2), ($1, 'event', 'e1', 50, 50, $3)`, c, t0, t0.Add(time.Second))
	res := e.must(c, "pending.diamond.claim_all", nil)
	var r econ.DiamondClaimResult
	_ = json.Unmarshal(res, &r)
	if len(r.Claimed) != 1 || r.Claimed[0].Quantity != 60 || r.StoppedAt == nil {
		t.Fatalf("%+v", r)
	}
	if b := testdb.Int64(t, e.pool, `SELECT balance FROM character_wallets WHERE character_id = $1 AND currency = 'DIAMOND'`, c); b != 960 {
		t.Fatalf("balance %d", b)
	}
	_, err := e.do(c, "pending.diamond.claim_all", nil)
	e.wantCode(err, "WALLET_CAP_EXCEEDED")
}

func TestUnknownCommandArgumentsAreRejected(t *testing.T) {
	e := newEnv(t)
	c := e.character("Strict")
	_, err := e.do(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot", "base_yield": 999})
	e.wantCode(err, "VALIDATION_FAILED")
	_, err = e.svc.Command(context.Background(), c, "short", "farm.harvest_all", nil)
	e.wantCode(err, "VALIDATION_FAILED")
	_, err = e.svc.Command(context.Background(), "00000000-0000-4000-8000-000000000000", "valid-request", "farm.harvest_all", nil)
	if !errors.Is(err, apperr.NotFound) {
		t.Fatalf("unknown character: %v", err)
	}
}
