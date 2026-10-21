package econ

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/avatar/transaction-service/internal/apperr"
	"example.com/avatar/transaction-service/internal/farm"
)

// Crop pending claims (P0-077..085, docs/02 §4).

type pendingLine struct {
	crop     string
	qty      int64
	earliest time.Time
}

func (w *Work) pendingLines(onlyCrop string) ([]pendingLine, error) {
	sql := `SELECT crop_code, quantity, earliest_created_at FROM pending_crops WHERE character_id = $1`
	args := []any{w.CharacterID}
	if onlyCrop != "" {
		sql += ` AND crop_code = $2`
		args = append(args, onlyCrop)
	}
	sql += ` ORDER BY earliest_created_at, crop_code FOR UPDATE`
	rows, err := w.tx.Query(w.ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (pendingLine, error) {
		var l pendingLine
		err := r.Scan(&l.crop, &l.qty, &l.earliest)
		return l, err
	})
}

type ClaimedLine struct {
	CropCode  string `json:"crop_code"`
	ItemCode  string `json:"item_code"`
	Quantity  int64  `json:"quantity"`
	Remaining int64  `json:"remaining"`
	Partial   bool   `json:"partial"`
}

type ClaimResult struct {
	Claimed []ClaimedLine `json:"claimed"`
	// StoppedAt is the first line Claim All could not take whole (P0-079).
	StoppedAt *string `json:"stopped_at,omitempty"`
}

// maxPossibleCapacity is the largest quantity of an item the bag could ever
// hold: the configured maximum slot count times the stack limit. 0 = unknown.
func (w *Work) maxPossibleCapacity(itemCode string) int64 {
	slots := w.Config.MaxPossibleSlots()
	if slots <= 0 {
		return 0
	}
	return int64(slots) * int64(w.Catalog().Items[itemCode].StackLimit)
}

// claimLine applies DecideClaim to one line and moves what it allows.
func (w *Work) claimLine(l pendingLine) (farm.ClaimOutcome, ClaimedLine, error) {
	crop := w.Catalog().Crops[l.crop]
	item := w.Catalog().Items[crop.CropItemCode]
	bag, err := w.Bag()
	if err != nil {
		return farm.ClaimRefused, ClaimedLine{}, err
	}
	free := bag.FreeCapacity(item.inv())
	outcome, take := farm.DecideClaim(l.qty, free, w.maxPossibleCapacity(crop.CropItemCode))
	cl := ClaimedLine{CropCode: l.crop, ItemCode: crop.CropItemCode, Remaining: l.qty}
	if outcome == farm.ClaimRefused {
		return outcome, cl, nil
	}
	if err := w.AddItem(crop.CropItemCode, take, "PENDING_CLAIM"); err != nil {
		return outcome, cl, err
	}
	remaining := l.qty - take
	if remaining == 0 {
		err = w.Exec(`DELETE FROM pending_crops WHERE character_id = $1 AND crop_code = $2`, w.CharacterID, l.crop)
	} else {
		err = w.Exec(`UPDATE pending_crops SET quantity = $3, updated_at = $4 WHERE character_id = $1 AND crop_code = $2`,
			w.CharacterID, l.crop, remaining, w.Now)
	}
	if err != nil {
		return outcome, cl, err
	}
	w.LogItem(crop.CropItemCode, "PENDING", -take, remaining, "PENDING_CLAIM")
	cl.Quantity, cl.Remaining, cl.Partial = take, remaining, outcome == farm.ClaimPartial
	return outcome, cl, nil
}

func refusal(w *Work, l pendingLine) error {
	crop := w.Catalog().Crops[l.crop]
	bag, err := w.Bag()
	if err != nil {
		return err
	}
	free := bag.FreeCapacity(w.Catalog().Items[crop.CropItemCode].inv())
	return apperr.PendingWholeBatch.With(map[string]any{"crop_code": l.crop, "required": l.qty, "free_capacity": free})
}

// ClaimPendingCrop takes one whole line into the bag, or nothing.
func (w *Work) ClaimPendingCrop(cropCode string) (any, error) {
	if _, ok := w.Catalog().Crops[cropCode]; !ok {
		return nil, apperr.Validation.With(map[string]any{"field": "crop_code"})
	}
	lines, err := w.pendingLines(cropCode)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, apperr.NotFound.With(map[string]any{"crop_code": cropCode})
	}
	outcome, cl, err := w.claimLine(lines[0])
	if err != nil {
		return nil, err
	}
	if outcome == farm.ClaimRefused {
		return nil, refusal(w, lines[0])
	}
	return ClaimResult{Claimed: []ClaimedLine{cl}}, nil
}

// ClaimAllPendingCrops goes through lines oldest first and stops at the first
// line it cannot take whole; under the P0-083 exception it takes as much as
// fits and then stops (P0-085). Nothing claimed at all is an error, not a
// "success" with zero items.
func (w *Work) ClaimAllPendingCrops() (any, error) {
	lines, err := w.pendingLines("")
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, apperr.NotFound.With(map[string]any{"reason": "no_pending"})
	}
	res := ClaimResult{Claimed: []ClaimedLine{}}
	for _, l := range lines {
		outcome, cl, err := w.claimLine(l)
		if err != nil {
			return nil, err
		}
		if outcome == farm.ClaimRefused {
			if len(res.Claimed) == 0 {
				return nil, refusal(w, l)
			}
			stopped := l.crop
			res.StoppedAt = &stopped
			break
		}
		res.Claimed = append(res.Claimed, cl)
		if outcome == farm.ClaimPartial {
			stopped := l.crop
			res.StoppedAt = &stopped
			break
		}
	}
	return res, nil
}

// Pending Diamonds (P0-034, 036..039). Entries are per source and FIFO. Taking
// part of an entry is not decided (GAP-08), so an entry is claimed whole or
// not at all and Claim All stops at the first entry that does not fit.

type DiamondClaim struct {
	ID       string `json:"id"`
	Quantity int64  `json:"quantity"`
}

type DiamondClaimResult struct {
	Claimed   []DiamondClaim `json:"claimed"`
	StoppedAt *string        `json:"stopped_at,omitempty"`
}

type diamondEntry struct {
	id        string
	remaining int64
}

func (w *Work) diamondEntries(onlyID string) ([]diamondEntry, error) {
	sql := `SELECT id::text, remaining FROM pending_diamond_awards WHERE character_id = $1 AND remaining > 0`
	args := []any{w.CharacterID}
	if onlyID != "" {
		if !IsUUID(onlyID) {
			return nil, apperr.NotFound
		}
		sql += ` AND id = $2`
		args = append(args, onlyID)
	}
	sql += ` ORDER BY created_at, id FOR UPDATE`
	rows, err := w.tx.Query(w.ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (diamondEntry, error) {
		var e diamondEntry
		err := r.Scan(&e.id, &e.remaining)
		return e, err
	})
}

func (w *Work) claimDiamondEntry(e diamondEntry) error {
	if err := w.Credit("DIAMOND", e.remaining, "PENDING_DIAMOND_CLAIM"); err != nil {
		return err
	}
	var left int64
	if err := w.tx.QueryRow(w.ctx, `SELECT COALESCE(SUM(remaining), 0) FROM pending_diamond_awards
		WHERE character_id = $1 AND remaining > 0 AND id <> $2`, w.CharacterID, e.id).Scan(&left); err != nil {
		return err
	}
	w.LogDiamondPending(-e.remaining, left, "PENDING_DIAMOND_CLAIM")
	return w.Exec(`UPDATE pending_diamond_awards SET remaining = 0, claimed_at = $2 WHERE id = $1`, e.id, w.Now)
}

func (w *Work) ClaimPendingDiamond(id string) (any, error) {
	entries, err := w.diamondEntries(id)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, apperr.NotFound
	}
	if err := w.claimDiamondEntry(entries[0]); err != nil {
		return nil, err
	}
	return DiamondClaimResult{Claimed: []DiamondClaim{{ID: entries[0].id, Quantity: entries[0].remaining}}}, nil
}

func (w *Work) ClaimAllPendingDiamonds() (any, error) {
	entries, err := w.diamondEntries("")
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, apperr.NotFound.With(map[string]any{"reason": "no_pending"})
	}
	res := DiamondClaimResult{Claimed: []DiamondClaim{}}
	for _, e := range entries {
		err := w.claimDiamondEntry(e)
		var biz *apperr.Error
		if errors.As(err, &biz) && biz.Code == apperr.WalletCapExceeded.Code {
			if len(res.Claimed) == 0 {
				return nil, err
			}
			id := e.id
			res.StoppedAt = &id
			break
		}
		if err != nil {
			return nil, err
		}
		res.Claimed = append(res.Claimed, DiamondClaim{ID: e.id, Quantity: e.remaining})
	}
	return res, nil
}

// GrantDiamonds credits a reward; the part above the cap becomes one pending
// entry for this source (P0-034). Used by quests and approved Admin changes.
func (w *Work) GrantDiamonds(amount int64, sourceKind, sourceRef, reason string) (credited, pending int64, err error) {
	if amount <= 0 {
		return 0, 0, nil
	}
	balance, err := w.Balance("DIAMOND")
	if err != nil {
		return 0, 0, err
	}
	limit, err := w.WalletCap("DIAMOND")
	if err != nil {
		return 0, 0, err
	}
	room := int64(0)
	if limit > balance {
		room = limit - balance
	}
	credited = min(room, amount)
	if credited > 0 {
		if err := w.Credit("DIAMOND", credited, reason); err != nil {
			return 0, 0, err
		}
	}
	pending = amount - credited
	if pending > 0 {
		if err := w.Exec(`INSERT INTO pending_diamond_awards (id, character_id, source_kind, source_ref, quantity, remaining, created_at)
			VALUES ($1, $2, $3, $4, $5, $5, $6)`, newUUID(), w.CharacterID, sourceKind, sourceRef, pending, w.Now); err != nil {
			return 0, 0, err
		}
		var total int64
		if err := w.tx.QueryRow(w.ctx, `SELECT COALESCE(SUM(remaining), 0) FROM pending_diamond_awards
			WHERE character_id = $1 AND remaining > 0`, w.CharacterID).Scan(&total); err != nil {
			return 0, 0, err
		}
		w.LogDiamondPending(pending, total, reason)
	}
	return credited, pending, nil
}
