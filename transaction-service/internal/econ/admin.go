package econ

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"example.com/project-thanh/transaction-service/internal/apperr"
)

// Maintenance (P0-152): while the switch is on, players' asset commands are
// refused so that configuration can be published and backups taken without
// transactions in flight. Admin adjustments still run.
var errMaintenance = &apperr.Error{Code: "MAINTENANCE", Message: "Máy chủ đang bảo trì, giao dịch tạm dừng.", Status: 503, Retryable: true}

func maintenanceActive(ctx context.Context, q querier) (bool, error) {
	var raw json.RawMessage
	err := q.QueryRow(ctx, `SELECT value FROM server_state WHERE key = 'maintenance'`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var m struct {
		Active bool `json:"active"`
	}
	_ = json.Unmarshal(raw, &m)
	return m.Active, nil
}

func (s *Service) checkMaintenance(ctx context.Context, tx pgx.Tx, op string) error {
	if strings.HasPrefix(op, "admin.") || op == "quest.event" {
		return nil
	}
	on, err := maintenanceActive(ctx, tx)
	if err != nil {
		return err
	}
	if on {
		return errMaintenance
	}
	return nil
}

type AdminAdjustResult struct {
	AssetKind string `json:"asset_kind"`
	ItemCode  string `json:"item_code,omitempty"`
	Delta     int64  `json:"amount_delta"`
	Pending   int64  `json:"pending_diamonds,omitempty"`
}

// AdminAdjust applies an approved two-person adjustment (P0-144). It is only
// reachable through the internal API, which the Game Server calls after the
// approval; players have no route to it. Removing more than a character has
// fails without a partial change.
func (w *Work) AdminAdjust(kind, itemCode string, delta int64) (any, error) {
	res := AdminAdjustResult{AssetKind: kind, ItemCode: itemCode, Delta: delta}
	switch kind {
	case "XU":
		if delta > 0 {
			return res, w.Credit("XU", delta, "ADMIN_ADJUSTMENT")
		}
		return res, w.Debit("XU", -delta, "ADMIN_ADJUSTMENT")
	case "DIAMOND":
		if delta > 0 {
			_, pending, err := w.GrantDiamonds(delta, "admin_adjustment", w.requestID, "ADMIN_ADJUSTMENT")
			res.Pending = pending
			return res, err
		}
		return res, w.Debit("DIAMOND", -delta, "ADMIN_ADJUSTMENT")
	case "ITEM":
		def, ok := w.Catalog().Items[itemCode]
		if !ok {
			return nil, apperr.Validation.With(map[string]any{"field": "item_code"})
		}
		if def.Category == "fashion" {
			if delta <= 0 || delta > 20 {
				return nil, apperr.Validation.With(map[string]any{"reason": "fashion items can only be granted, 1..20"})
			}
			return res, w.grantFashion(def, delta, "ADMIN_ADJUSTMENT")
		}
		if delta > 0 {
			return res, w.AddItem(itemCode, delta, "ADMIN_ADJUSTMENT")
		}
		return res, w.RemoveItem(itemCode, -delta, "ADMIN_ADJUSTMENT")
	}
	return nil, apperr.Validation.With(map[string]any{"field": "asset_kind"})
}

type adminAdjustCmd struct {
	AssetKind   string `json:"asset_kind"`
	ItemCode    string `json:"item_code"`
	AmountDelta int64  `json:"amount_delta"`
}

func (c *adminAdjustCmd) exec(w *Work) (any, error) {
	if c.AmountDelta == 0 {
		return nil, apperr.Validation.With(map[string]any{"field": "amount_delta"})
	}
	return w.AdminAdjust(c.AssetKind, c.ItemCode, c.AmountDelta)
}
