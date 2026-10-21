package econ_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"example.com/project-thanh/transaction-service/internal/econ"
	"example.com/project-thanh/transaction-service/internal/testdb"
)

// withFashionAndQuests adds sample fashion items (catalog rows are loaded
// when the service starts, so a fresh service is built) and publishes a
// version with fashion offers and daily quests.
func withFashionAndQuests(t *testing.T) *env {
	e := newEnv(t)
	testdb.Exec(t, e.pool, `INSERT INTO npc_definitions (npc_code, display_name) VALUES ('FASHION_SHOP', 'Test Tailor')`)
	testdb.Exec(t, e.pool, `INSERT INTO item_definitions (item_code, display_name, category, stack_limit, trade_policy) VALUES
		('hat_a', 'Hat A', 'fashion', 1, 'unbound'), ('top_a', 'Top A', 'fashion', 1, 'bound')`)
	testdb.Exec(t, e.pool, `INSERT INTO fashion_items (item_code, equip_slot, style) VALUES ('hat_a', 'hat', '{"hat":"cap"}'), ('top_a', 'top', '{"shirt":"#f00"}')`)
	cfg := baseConfig
	cfg.Label = "TEST-M6"
	cfg.Values = map[string]any{}
	for k, v := range baseConfig.Values {
		cfg.Values[k] = v
	}
	cfg.Values["quests.daily"] = map[string]any{"timezone": "Asia/Ho_Chi_Minh"}
	cfg.Offers = append(append([]testdb.Offer{}, baseConfig.Offers...),
		testdb.Offer{NPC: "FASHION_SHOP", Item: "hat_a", Direction: "NPC_SELLS", Currency: "XU", Price: 300},
		testdb.Offer{NPC: "FASHION_SHOP", Item: "top_a", Direction: "NPC_SELLS", Currency: "XU", Price: 500})
	cfg.Quests = []testdb.Quest{
		{Code: "plant3", EventType: "plant_crop", Currency: "XU", Target: 3, Reward: 200},
		{Code: "pest1", EventType: "catch_pest", Currency: "DIAMOND", Target: 1, Reward: 50},
		{Code: "chat1", EventType: "public_chat", Currency: "XU", Target: 2, Reward: 10},
	}
	e.cfgV = testdb.Publish(t, e.pool, cfg)
	svc, err := econ.New(context.Background(), e.pool)
	if err != nil {
		t.Fatal(err)
	}
	svc.Now, svc.Rand = e.clock.Now, e.rand
	e.svc = svc
	return e
}

func (e *env) buyFrom(char, npc, item string, qty int64) (json.RawMessage, error) {
	var id string
	_ = e.pool.QueryRow(context.Background(), `SELECT o.id::text FROM npc_offers o JOIN game_config_versions v ON v.id = o.config_version_id
		WHERE v.state = 'published' AND o.item_code = $1`, item).Scan(&id)
	return e.do(char, "npc.buy", map[string]any{"npc_code": npc, "offer_id": id, "quantity": qty, "config_version": e.cfgV})
}

func TestFashionInstancesEquipAndOwnership(t *testing.T) {
	e := withFashionAndQuests(t)
	c := e.character("Fashionista")
	if _, err := e.buyFrom(c, "FASHION_SHOP", "hat_a", 2); err != nil {
		t.Fatal(err)
	}
	st := e.state(c)
	var inst []string
	for _, s := range st.Inventory.Slots {
		if s.ItemCode == "hat_a" {
			if s.InstanceID == nil || s.Quantity != 1 {
				t.Fatalf("fashion slot %+v", s)
			}
			inst = append(inst, *s.InstanceID)
		}
	}
	if len(inst) != 2 || inst[0] == inst[1] { // F-16 through the real purchase flow
		t.Fatalf("instances %v", inst)
	}
	if e.xu(c) != 5000-600 {
		t.Fatalf("xu %d", e.xu(c))
	}
	e.must(c, "fashion.equip", map[string]any{"instance_id": inst[1]})
	st = e.state(c)
	if st.Equipment["hat"] != inst[1] || st.Character.AppearanceVersion != 1 {
		t.Fatalf("equipment %+v version %d", st.Equipment, st.Character.AppearanceVersion)
	}
	// Equipping keeps the item in the bag.
	if len(st.Inventory.Slots) != 2 {
		t.Fatalf("slots %+v", st.Inventory.Slots)
	}
	other := e.character("Thief")
	_, err := e.do(other, "fashion.equip", map[string]any{"instance_id": inst[0]})
	e.wantCode(err, "NOT_FOUND")
	e.must(c, "fashion.unequip", map[string]any{"equip_slot": "hat"})
	if st := e.state(c); len(st.Equipment) != 0 {
		t.Fatalf("still equipped %+v", st.Equipment)
	}
	// Database guard: equipment cannot point at someone else's instance.
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO character_equipment (character_id, equip_slot, item_instance_id) VALUES ($1, 'hat', $2)`, other, inst[0]); err == nil {
		t.Fatal("foreign instance equipped")
	}
	// No room for a fashion item: refused as a whole, money untouched.
	e.fillBag(c, 3, 10, "crop_potato", 999)
	before := e.xu(c)
	_, err = e.buyFrom(c, "FASHION_SHOP", "top_a", 1)
	e.wantCode(err, "INVENTORY_CAPACITY_INSUFFICIENT")
	if e.xu(c) != before {
		t.Fatal("money taken for a refused purchase")
	}
}

func TestDailyQuestProgressClaimOnceAndReset(t *testing.T) {
	e := withFashionAndQuests(t)
	c := e.character("Quester")
	e.buy(c, "seed_carrot", 4)
	e.must(c, "farm.plant_all", map[string]any{"crop_code": "carrot", "confirmed": true}) // 4 plots, progress capped at 3
	q, _ := e.svc.Quests(context.Background(), c)
	var plant econ.QuestView
	for _, v := range q.Quests {
		if v.Code == "plant3" {
			plant = v
		}
	}
	if !q.Enabled || plant.Progress != 3 || plant.CompletedAt == nil {
		t.Fatalf("quests %+v", q)
	}
	before := e.xu(c)
	e.must(c, "quests.claim", map[string]any{"quest_code": "plant3"})
	if e.xu(c) != before+200 {
		t.Fatalf("reward not paid: %d -> %d", before, e.xu(c))
	}
	_, err := e.do(c, "quests.claim", map[string]any{"quest_code": "plant3"})
	e.wantCode(err, "QUEST_ALREADY_CLAIMED")
	_, err = e.do(c, "quests.claim", map[string]any{"quest_code": "pest1"})
	e.wantCode(err, "QUEST_NOT_COMPLETE")
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM asset_ledger WHERE character_id = $1 AND reason_code = 'QUEST_REWARD'`, c); n != 1 {
		t.Fatalf("%d reward ledger rows", n)
	}
	// Next local day: a fresh quest day.
	e.clock.Advance(24 * time.Hour)
	q, _ = e.svc.Quests(context.Background(), c)
	for _, v := range q.Quests {
		if v.Progress != 0 || v.ClaimedAt != nil {
			t.Fatalf("not reset: %+v", v)
		}
	}
}

func TestDiamondQuestRewardOverTheCapGoesToPending(t *testing.T) {
	e := withFashionAndQuests(t)
	c := e.character("Gemmy")
	testdb.Exec(t, e.pool, `UPDATE character_wallets SET balance = 980 WHERE character_id = $1 AND currency = 'DIAMOND'`, c) // cap 1000
	e.rand.float, e.rand.intValue = 0, 0
	e.buy(c, "seed_carrot", 1)
	e.must(c, "farm.plant", map[string]any{"plot_no": 1, "crop_code": "carrot"})
	e.must(c, "farm.catch_pest", map[string]any{"plot_no": 1})
	res := e.must(c, "quests.claim", map[string]any{"quest_code": "pest1"})
	var r econ.ClaimQuestResult
	_ = json.Unmarshal(res, &r)
	if r.Credited != 20 || r.Pending != 30 {
		t.Fatalf("claim %+v", r)
	}
	st := e.state(c)
	if st.Wallets["DIAMOND"].Balance != 1000 || len(st.PendingDiamonds) != 1 || st.PendingDiamonds[0].Remaining != 30 {
		t.Fatalf("wallet %+v pending %+v", st.Wallets["DIAMOND"], st.PendingDiamonds)
	}
}

func TestExternalChatEventCountsOncePerMessage(t *testing.T) {
	e := withFashionAndQuests(t)
	c := e.character("Chatty")
	args, _ := json.Marshal(map[string]any{"event_type": "public_chat", "amount": 1})
	for i := 0; i < 2; i++ { // the same message reported twice
		if _, err := e.svc.Command(context.Background(), c, "chat-msg-0001", "quest.event", args); err != nil {
			t.Fatal(err)
		}
	}
	q, _ := e.svc.Quests(context.Background(), c)
	for _, v := range q.Quests {
		if v.Code == "chat1" && v.Progress != 1 {
			t.Fatalf("progress %d", v.Progress)
		}
	}
	bad, _ := json.Marshal(map[string]any{"event_type": "plant_crop", "amount": 5})
	_, err := e.svc.Command(context.Background(), c, "chat-msg-0002", "quest.event", bad)
	e.wantCode(err, "VALIDATION_FAILED") // gameplay events can only come from this service
}
