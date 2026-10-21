package econ_test

import (
	"testing"

	"example.com/project-thanh/transaction-service/internal/testdb"
)

func TestA07RenameConsumesTheCardOnlyOnSuccess(t *testing.T) {
	e := newEnv(t)
	c := e.character("Tên Cũ")
	e.character("Đã Có")
	_, err := e.do(c, "character.rename", map[string]any{"display_name": "Tên Mới"})
	e.wantCode(err, "ITEM_INSUFFICIENT") // no card

	testdb.Exec(t, e.pool, `INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity) VALUES ($1, 1, 'rename_card', 2)`, c)
	_, err = e.do(c, "character.rename", map[string]any{"display_name": "đã có"}) // taken (case-insensitive)
	e.wantCode(err, "CHARACTER_NAME_TAKEN")
	_, err = e.do(c, "character.rename", map[string]any{"display_name": "x"})
	e.wantCode(err, "CHARACTER_NAME_INVALID")
	if e.count(c, "rename_card") != 2 {
		t.Fatal("a failed rename consumed a card")
	}
	e.must(c, "character.rename", map[string]any{"display_name": "Tên Mới"})
	e.must(c, "character.rename", map[string]any{"display_name": "Tên Mới ★"}) // no cooldown (P0-029)
	if e.count(c, "rename_card") != 0 {
		t.Fatalf("cards left %d", e.count(c, "rename_card"))
	}
	if n := testdb.Int64(t, e.pool, `SELECT count(*) FROM character_renames WHERE character_id = $1`, c); n != 2 {
		t.Fatalf("%d rename records", n)
	}
	st := e.state(c)
	if st.Character.DisplayName != "Tên Mới ★" {
		t.Fatalf("name %q", st.Character.DisplayName)
	}
}

func TestT05NoDurableStandbyStopsAssetWrites(t *testing.T) {
	e := newEnv(t)
	c := e.character("Durable")
	e.svc.RequireSyncStandby = true // the test database has no synchronous standby
	_, err := e.do(c, "npc.buy", map[string]any{"npc_code": "FARM_SHOP", "offer_id": e.offerID("seed_carrot", "NPC_SELLS"), "quantity": 1, "config_version": e.cfgV})
	e.wantCode(err, "ASSET_WRITES_DISABLED")
	if e.xu(c) != 5000 {
		t.Fatal("money moved without durability")
	}
}
