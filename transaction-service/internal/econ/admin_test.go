package econ_test

import (
	"context"
	"encoding/json"
	"testing"

	"example.com/avatar/transaction-service/internal/testdb"
)

func TestMaintenanceStopsPlayerCommandsWithoutCachingTheRefusal(t *testing.T) {
	e := newEnv(t)
	c := e.character("Waiter")
	testdb.Exec(t, e.pool, `UPDATE server_state SET value = '{"active": true, "message": "bảo trì"}' WHERE key = 'maintenance'`)
	args, _ := json.Marshal(map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("seed_carrot", "NPC_SELLS"), "quantity": 1, "config_version": e.cfgV})
	_, err := e.svc.Command(context.Background(), c, "buy-during-mnt", "npc.buy", args)
	e.wantCode(err, "MAINTENANCE")
	acc := testdb.CreateAccount(t, e.pool, "late_comer")
	_, _, err = e.svc.CreateCharacter(context.Background(), acc, "Đến Muộn", "create-late-01")
	e.wantCode(err, "MAINTENANCE")

	testdb.Exec(t, e.pool, `UPDATE server_state SET value = '{"active": false, "message": ""}' WHERE key = 'maintenance'`)
	// The same request id works afterwards: the refusal was not stored.
	if _, err := e.svc.Command(context.Background(), c, "buy-during-mnt", "npc.buy", args); err != nil {
		t.Fatalf("after maintenance: %v", err)
	}
}

func TestAdminAdjustmentsAreAtomicAndLedgered(t *testing.T) {
	e := newEnv(t)
	c := e.character("Compensated")
	adj := func(id string, a map[string]any) error {
		raw, _ := json.Marshal(a)
		_, err := e.svc.Command(context.Background(), c, id, "admin.adjust", raw)
		return err
	}
	if err := adj("adj-00000001", map[string]any{"asset_kind": "XU", "amount_delta": 250}); err != nil {
		t.Fatal(err)
	}
	// Replaying the same adjustment id never applies it twice (T-10).
	if err := adj("adj-00000001", map[string]any{"asset_kind": "XU", "amount_delta": 250}); err != nil {
		t.Fatal(err)
	}
	if e.xu(c) != 5250 {
		t.Fatalf("xu %d", e.xu(c))
	}
	// Diamonds above the cap (1000) wait in pending.
	if err := adj("adj-00000002", map[string]any{"asset_kind": "DIAMOND", "amount_delta": 1200}); err != nil {
		t.Fatal(err)
	}
	st := e.state(c)
	if st.Wallets["DIAMOND"].Balance != 1000 || len(st.PendingDiamonds) != 1 || st.PendingDiamonds[0].Remaining != 200 {
		t.Fatalf("diamonds %+v pending %+v", st.Wallets["DIAMOND"], st.PendingDiamonds)
	}
	// Taking more than held fails as a whole.
	err := adj("adj-00000003", map[string]any{"asset_kind": "XU", "amount_delta": -999999})
	e.wantCode(err, "INSUFFICIENT_FUNDS")
	if err := adj("adj-00000004", map[string]any{"asset_kind": "ITEM", "item_code": "seed_corn", "amount_delta": 7}); err != nil {
		t.Fatal(err)
	}
	if e.count(c, "seed_corn") != 7 {
		t.Fatal("item not granted")
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM asset_ledger WHERE character_id = $1 AND reason_code = 'ADMIN_ADJUSTMENT'`, c); n != 4 {
		t.Fatalf("%d ledger rows", n)
	}
}
