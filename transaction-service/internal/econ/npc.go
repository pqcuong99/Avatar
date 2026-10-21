package econ

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"

	"example.com/avatar/transaction-service/internal/apperr"
)

// NPC trade (docs/02 §7). Prices come only from the published config version;
// the client sends the config_version it was shown and is rejected with
// CONFIG_VERSION_OUTDATED if that is no longer the published one (T-06/T-11).
// Proximity to the NPC is checked by the Game Server before forwarding.

type Offer struct {
	ID           string `json:"offer_id"`
	ItemCode     string `json:"item_code"`
	ItemName     string `json:"item_name"`
	Category     string `json:"category"`
	Direction    string `json:"direction"`
	Currency     string `json:"currency"`
	UnitPrice    int64  `json:"unit_price"`
	HeldQuantity int64  `json:"held_quantity"`
}

type OffersView struct {
	NPCCode       string  `json:"npc_code"`
	DisplayName   string  `json:"display_name"`
	ConfigVersion int64   `json:"config_version"`
	Offers        []Offer `json:"offers"`
}

// MaxTradeQuantity bounds one order; it only protects against absurd input.
const MaxTradeQuantity = 9999

func (s *Service) NPCOffers(ctx context.Context, characterID, npcCode string) (*OffersView, error) {
	npc, ok := s.catalog.NPCs[npcCode]
	if !ok {
		if err := s.catalog.ReloadNPCs(ctx, s.pool); err != nil {
			return nil, err
		}
		if npc, ok = s.catalog.NPCs[npcCode]; !ok {
			return nil, apperr.NotFound
		}
	}
	if !IsUUID(characterID) {
		return nil, apperr.NotFound
	}
	cfg, err := loadPublishedConfig(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	view := &OffersView{NPCCode: npc.Code, DisplayName: npc.DisplayName, ConfigVersion: cfg.VersionID, Offers: []Offer{}}
	if cfg.VersionID == 0 {
		return view, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT o.id::text, o.item_code, d.display_name, d.category, o.direction, o.currency, o.unit_price,
			COALESCE((SELECT SUM(quantity) FROM inventory_slots s WHERE s.character_id = $3 AND s.item_code = o.item_code), 0)
		FROM npc_offers o JOIN item_definitions d USING (item_code)
		WHERE o.config_version_id = $1 AND o.npc_code = $2
		ORDER BY o.direction DESC, o.sort_order, o.item_code`, cfg.VersionID, npcCode, characterID)
	if err != nil {
		return nil, err
	}
	view.Offers, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Offer])
	return view, err
}

type tradeOffer struct {
	itemCode, direction, currency string
	price                         int64
}

func (w *Work) offer(npcCode, offerID string, clientVersion int64) (tradeOffer, error) {
	if w.Config.VersionID == 0 {
		return tradeOffer{}, apperr.NotConfigured.With(map[string]any{"config": "npc_offers"})
	}
	if clientVersion != w.Config.VersionID {
		return tradeOffer{}, apperr.ConfigOutdated.With(map[string]any{"config_version": w.Config.VersionID})
	}
	if !IsUUID(offerID) {
		return tradeOffer{}, apperr.ConfigOutdated.With(map[string]any{"config_version": w.Config.VersionID})
	}
	var o tradeOffer
	err := w.tx.QueryRow(w.ctx, `SELECT item_code, direction, currency, unit_price FROM npc_offers
		WHERE id = $1 AND npc_code = $2 AND config_version_id = $3`, offerID, npcCode, w.Config.VersionID).
		Scan(&o.itemCode, &o.direction, &o.currency, &o.price)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, apperr.ConfigOutdated.With(map[string]any{"config_version": w.Config.VersionID})
	}
	return o, err
}

type TradeResult struct {
	ItemCode  string `json:"item_code"`
	Quantity  int64  `json:"quantity"`
	Currency  string `json:"currency"`
	UnitPrice int64  `json:"unit_price"`
	Total     int64  `json:"total"`
}

func checkQuantity(q int64) error {
	if q < 1 || q > MaxTradeQuantity {
		return apperr.Validation.With(map[string]any{"field": "quantity", "min": 1, "max": MaxTradeQuantity})
	}
	return nil
}

// Buy: the NPC sells to the player. Money and bag space are both checked, and
// the debit and the item grant commit together or not at all (T-01, T-04).
func (w *Work) Buy(npcCode, offerID string, qty, clientVersion int64) (any, error) {
	if err := checkQuantity(qty); err != nil {
		return nil, err
	}
	o, err := w.offer(npcCode, offerID, clientVersion)
	if err != nil {
		return nil, err
	}
	if o.direction != "NPC_SELLS" {
		return nil, apperr.Validation.With(map[string]any{"field": "offer_id"})
	}
	if w.Catalog().Items[o.itemCode].Category == "fashion" {
		return w.buyFashion(o, qty)
	}
	if o.price > math.MaxInt64/qty {
		return nil, apperr.Validation.With(map[string]any{"field": "quantity"})
	}
	total := o.price * qty
	if err := w.Debit(o.currency, total, "NPC_BUY"); err != nil {
		return nil, err
	}
	if err := w.AddItem(o.itemCode, qty, "NPC_BUY"); err != nil {
		return nil, err
	}
	return TradeResult{ItemCode: o.itemCode, Quantity: qty, Currency: o.currency, UnitPrice: o.price, Total: total}, nil
}

// Sell: the NPC buys from the player, from the bag only. Crops still in
// pending cannot be sold (P0-080, T-08).
func (w *Work) Sell(npcCode, offerID string, qty, clientVersion int64) (any, error) {
	if err := checkQuantity(qty); err != nil {
		return nil, err
	}
	o, err := w.offer(npcCode, offerID, clientVersion)
	if err != nil {
		return nil, err
	}
	if o.direction != "NPC_BUYS" {
		return nil, apperr.Validation.With(map[string]any{"field": "offer_id"})
	}
	if err := w.RemoveItem(o.itemCode, qty, "NPC_SELL"); err != nil {
		if biz, ok := apperr.As(err); ok && biz.Code == apperr.ItemInsufficient.Code {
			if crop, isCrop := w.Catalog().CropByItem(o.itemCode); isCrop {
				var pending int64
				_ = w.tx.QueryRow(w.ctx, `SELECT COALESCE(SUM(quantity), 0) FROM pending_crops
					WHERE character_id = $1 AND crop_code = $2`, w.CharacterID, crop.Code).Scan(&pending)
				if pending > 0 {
					d := map[string]any{}
					for k, v := range biz.Details {
						d[k] = v
					}
					d["in_pending"] = pending
					return nil, apperr.ItemInsufficient.With(d)
				}
			}
		}
		return nil, err
	}
	if o.price > math.MaxInt64/qty {
		return nil, apperr.Validation.With(map[string]any{"field": "quantity"})
	}
	total := o.price * qty
	if err := w.Credit(o.currency, total, "NPC_SELL"); err != nil {
		return nil, err
	}
	if w.Catalog().Items[o.itemCode].Category == "crop" {
		if err := w.QuestEvent("sell_crop_to_npc", int(qty)); err != nil {
			return nil, err
		}
	}
	return TradeResult{ItemCode: o.itemCode, Quantity: qty, Currency: o.currency, UnitPrice: o.price, Total: total}, nil
}
