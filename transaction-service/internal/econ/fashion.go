package econ

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"example.com/avatar/transaction-service/internal/apperr"
)

// Fashion (MVP-06): one instance per item, one bag slot per instance
// (TH-02-004). Equipping keeps the item in its slot (see migration 0006).

type FashionDef struct {
	ItemCode  string          `json:"item_code"`
	EquipSlot string          `json:"equip_slot"`
	Style     json.RawMessage `json:"style"`
}

func loadFashion(ctx context.Context, q querier) (map[string]FashionDef, error) {
	rows, err := q.Query(ctx, `SELECT item_code, equip_slot, style FROM fashion_items`)
	if err != nil {
		return nil, err
	}
	defs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[FashionDef])
	if err != nil {
		return nil, err
	}
	m := map[string]FashionDef{}
	for _, d := range defs {
		m[d.ItemCode] = d
	}
	return m, nil
}

// buyFashion creates one instance per unit; all units need a free slot or the
// purchase is refused as a whole.
func (w *Work) buyFashion(o tradeOffer, qty int64) (any, error) {
	if qty > 20 {
		return nil, apperr.Validation.With(map[string]any{"field": "quantity", "max": 20})
	}
	def := w.Catalog().Items[o.itemCode]
	bag, err := w.Bag()
	if err != nil {
		return nil, err
	}
	if free := bag.FreeCapacity(def.inv()); free < qty {
		return nil, apperr.InventoryCapacity.With(map[string]any{"item_code": o.itemCode, "required": qty, "free_capacity": free})
	}
	total := o.price * qty
	if err := w.Debit(o.currency, total, "NPC_BUY"); err != nil {
		return nil, err
	}
	if err := w.grantFashion(def, qty, "NPC_BUY"); err != nil {
		return nil, err
	}
	return map[string]any{"item_code": o.itemCode, "quantity": qty, "currency": o.currency, "unit_price": o.price, "total": total}, nil
}

// grantFashion creates qty instances, each in its own bag slot, all or none.
func (w *Work) grantFashion(def ItemDef, qty int64, reason string) error {
	bag, err := w.Bag()
	if err != nil {
		return err
	}
	if free := bag.FreeCapacity(def.inv()); free < qty {
		return apperr.InventoryCapacity.With(map[string]any{"item_code": def.Code, "required": qty, "free_capacity": free})
	}
	for i := int64(0); i < qty; i++ {
		id := newUUID()
		if err := w.Exec(`INSERT INTO item_instances (id, item_code, owner_character_id, bound, acquired_at) VALUES ($1, $2, $3, $4, $5)`,
			id, def.Code, w.CharacterID, def.TradePolicy == "bound", w.Now); err != nil {
			return err
		}
		if _, err := bag.AddInstance(def.inv(), id); err != nil {
			return err
		}
	}
	w.ledger = append(w.ledger, ledgerEntry{kind: "ITEM", ref: def.Code, location: "BAG", reason: reason, delta: qty, after: bag.Count(def.Code)})
	return nil
}

type EquipResult struct {
	Equipment map[string]string `json:"equipment"` // equip_slot -> instance id
}

func (w *Work) equipment() (map[string]string, error) {
	rows, err := w.tx.Query(w.ctx, `SELECT equip_slot, item_instance_id::text FROM character_equipment WHERE character_id = $1`, w.CharacterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var slot, id string
		if err := rows.Scan(&slot, &id); err != nil {
			return nil, err
		}
		out[slot] = id
	}
	return out, rows.Err()
}

func (w *Work) bumpAppearance() error {
	return w.Exec(`UPDATE characters SET appearance_version = appearance_version + 1, updated_at = $2 WHERE id = $1`, w.CharacterID, w.Now)
}

// Equip wears an owned fashion instance from the bag, replacing whatever was
// in its slot.
func (w *Work) Equip(instanceID string) (any, error) {
	if !IsUUID(instanceID) {
		return nil, apperr.NotFound
	}
	bag, err := w.Bag()
	if err != nil {
		return nil, err
	}
	slot, ok := bag.FindInstance(instanceID)
	if !ok {
		return nil, apperr.NotFound.With(map[string]any{"instance_id": instanceID})
	}
	f, ok := w.s.fashion()[slot.ItemCode]
	if !ok {
		return nil, apperr.Validation.With(map[string]any{"reason": "not_fashion"})
	}
	if err := w.Exec(`INSERT INTO character_equipment (character_id, equip_slot, item_instance_id, equipped_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (character_id, equip_slot) DO UPDATE SET item_instance_id = EXCLUDED.item_instance_id, equipped_at = EXCLUDED.equipped_at`,
		w.CharacterID, f.EquipSlot, instanceID, w.Now); err != nil {
		return nil, err
	}
	if err := w.bumpAppearance(); err != nil {
		return nil, err
	}
	eq, err := w.equipment()
	return EquipResult{Equipment: eq}, err
}

func (w *Work) Unequip(slot string) (any, error) {
	if slot != "hat" && slot != "top" && slot != "bottom" {
		return nil, apperr.Validation.With(map[string]any{"field": "equip_slot"})
	}
	if err := w.Exec(`DELETE FROM character_equipment WHERE character_id = $1 AND equip_slot = $2`, w.CharacterID, slot); err != nil {
		return nil, err
	}
	if err := w.bumpAppearance(); err != nil {
		return nil, err
	}
	eq, err := w.equipment()
	return EquipResult{Equipment: eq}, err
}

// fashion is the fashion part of the catalog (loaded with it at start; item
// definitions only change through migrations or content seeds).
func (s *Service) fashion() map[string]FashionDef { return s.catalog.Fashion }
