package econ

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/avatar/transaction-service/internal/apperr"
)

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// State is everything the client shows about its own assets. Times are server
// UTC; the client never uses its own clock as the truth (docs/04 §4).
type State struct {
	ServerTime      time.Time        `json:"server_time"`
	ConfigVersion   int64            `json:"config_version"`
	Character       Character        `json:"character"`
	Wallets         map[string]Money `json:"wallets"`
	Inventory       InventoryView    `json:"inventory"`
	Farm            FarmView         `json:"farm"`
	PendingCrops    []PendingCrop    `json:"pending_crops"`
	PendingDiamonds []PendingDiamond `json:"pending_diamonds"`
	// Equipment maps equip slot -> item instance id (the item stays in its bag slot).
	Equipment map[string]string `json:"equipment"`
}

type Money struct {
	Balance int64  `json:"balance"`
	Cap     *int64 `json:"cap"`
}

type InventoryView struct {
	SlotCap int        `json:"slot_cap"`
	Slots   []SlotView `json:"slots"`
}

type SlotView struct {
	SlotNo     int     `json:"slot_no"`
	ItemCode   string  `json:"item_code"`
	Quantity   int     `json:"quantity"`
	InstanceID *string `json:"instance_id,omitempty"`
}

type FarmView struct {
	Plots []PlotView `json:"plots"`
}

type PlotView struct {
	PlotNo   int           `json:"plot_no"`
	Planting *PlantingView `json:"planting"`
}

type PlantingView struct {
	CropCode   string    `json:"crop_code"`
	PlantedAt  time.Time `json:"planted_at"`
	MaturesAt  time.Time `json:"matures_at"`
	BaseYield  int       `json:"base_yield"`
	Mature     bool      `json:"mature"`
	Pest       *PestView `json:"pest"`
	ConfigVers int64     `json:"config_version"`
}

// PestView is only present once the pest has appeared: the client does not
// learn a future spawn time in advance.
type PestView struct {
	SpawnAt    time.Time  `json:"spawn_at"`
	DeadlineAt time.Time  `json:"deadline_at"`
	CaughtAt   *time.Time `json:"caught_at"`
	// active: can be caught now. missed: deadline passed, penalty applies.
	Active         bool `json:"active"`
	Missed         bool `json:"missed"`
	PenaltyPercent *int `json:"penalty_percent,omitempty"`
}

type PendingCrop struct {
	CropCode          string    `json:"crop_code"`
	ItemCode          string    `json:"item_code"`
	Quantity          int64     `json:"quantity"`
	EarliestCreatedAt time.Time `json:"earliest_created_at"`
}

type PendingDiamond struct {
	ID         string    `json:"id"`
	SourceKind string    `json:"source_kind"`
	SourceRef  string    `json:"source_ref"`
	Quantity   int64     `json:"quantity"`
	Remaining  int64     `json:"remaining"`
	CreatedAt  time.Time `json:"created_at"`
}

// State reads a consistent snapshot (one REPEATABLE READ transaction).
func (s *Service) State(ctx context.Context, characterID string) (*State, error) {
	if !IsUUID(characterID) {
		return nil, apperr.NotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck
	now := s.now()
	st := &State{ServerTime: now, Wallets: map[string]Money{}}

	c := &st.Character
	err = tx.QueryRow(ctx, `SELECT id, account_id, display_name, inventory_slot_cap, appearance_version, created_at
		FROM characters WHERE id = $1`, characterID).
		Scan(&c.ID, &c.AccountID, &c.DisplayName, &c.InventorySlotCap, &c.AppearanceVersion, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound
	}
	if err != nil {
		return nil, err
	}
	cfg, err := loadPublishedConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	st.ConfigVersion = cfg.VersionID

	rows, err := tx.Query(ctx, `SELECT currency, balance, wallet_cap FROM character_wallets WHERE character_id = $1`, characterID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var cur string
		var m Money
		if err := rows.Scan(&cur, &m.Balance, &m.Cap); err != nil {
			rows.Close()
			return nil, err
		}
		if m.Cap == nil {
			switch {
			case cur == "XU":
				v := cfg.XuCap
				m.Cap = &v
			case cfg.DiamondCap > 0:
				v := cfg.DiamondCap
				m.Cap = &v
			}
		}
		st.Wallets[cur] = m
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	st.Inventory.SlotCap = c.InventorySlotCap
	rows, err = tx.Query(ctx, `SELECT slot_no, item_code, quantity, item_instance_id::text
		FROM inventory_slots WHERE character_id = $1 ORDER BY slot_no`, characterID)
	if err != nil {
		return nil, err
	}
	st.Inventory.Slots, err = pgx.CollectRows(rows, pgx.RowToStructByPos[SlotView])
	if err != nil {
		return nil, err
	}

	if st.Farm, err = s.farmView(ctx, tx, characterID, now); err != nil {
		return nil, err
	}

	rows, err = tx.Query(ctx, `SELECT p.crop_code, d.crop_item_code, p.quantity, p.earliest_created_at
		FROM pending_crops p JOIN crop_definitions d USING (crop_code)
		WHERE p.character_id = $1 ORDER BY p.earliest_created_at, p.crop_code`, characterID)
	if err != nil {
		return nil, err
	}
	st.PendingCrops, err = pgx.CollectRows(rows, pgx.RowToStructByPos[PendingCrop])
	if err != nil {
		return nil, err
	}

	rows, err = tx.Query(ctx, `SELECT id::text, source_kind, source_ref, quantity, remaining, created_at
		FROM pending_diamond_awards WHERE character_id = $1 AND remaining > 0 ORDER BY created_at, id`, characterID)
	if err != nil {
		return nil, err
	}
	st.PendingDiamonds, err = pgx.CollectRows(rows, pgx.RowToStructByPos[PendingDiamond])
	if err != nil {
		return nil, err
	}

	st.Equipment = map[string]string{}
	rows, err = tx.Query(ctx, `SELECT equip_slot, item_instance_id::text FROM character_equipment WHERE character_id = $1`, characterID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var slot, id string
		if err := rows.Scan(&slot, &id); err != nil {
			rows.Close()
			return nil, err
		}
		st.Equipment[slot] = id
	}
	rows.Close()
	return st, rows.Err()
}

func (s *Service) farmView(ctx context.Context, q querier, characterID string, now time.Time) (FarmView, error) {
	var fv FarmView
	rows, err := q.Query(ctx, `SELECT f.plot_no, p.crop_code, p.planted_at, p.matures_at, p.base_yield_snapshot,
			p.config_version_id, i.spawn_at, i.deadline_at, i.caught_at, i.penalty_percent
		FROM farm_plots f
		LEFT JOIN crop_plantings p ON p.character_id = f.character_id AND p.plot_no = f.plot_no AND p.state = 'growing'
		LEFT JOIN pest_incidents i ON i.planting_id = p.id
		WHERE f.character_id = $1 ORDER BY f.plot_no`, characterID)
	if err != nil {
		return fv, err
	}
	defer rows.Close()
	for rows.Next() {
		var pv PlotView
		var crop *string
		var planted, matures, spawn, deadline, caught *time.Time
		var baseYield *int
		var cfgVer *int64
		var penalty *int16
		if err := rows.Scan(&pv.PlotNo, &crop, &planted, &matures, &baseYield, &cfgVer, &spawn, &deadline, &caught, &penalty); err != nil {
			return fv, err
		}
		if crop != nil {
			p := &PlantingView{CropCode: *crop, PlantedAt: planted.UTC(), MaturesAt: matures.UTC(), BaseYield: *baseYield,
				Mature: !now.Before(*matures), ConfigVers: *cfgVer}
			if spawn != nil && !now.Before(*spawn) {
				pest := &PestView{SpawnAt: spawn.UTC(), DeadlineAt: deadline.UTC(), CaughtAt: caught}
				pest.Active = caught == nil && now.Before(*deadline)
				pest.Missed = caught == nil && !now.Before(*deadline)
				if pest.Missed {
					v := int(*penalty)
					pest.PenaltyPercent = &v
				}
				p.Pest = pest
			}
			pv.Planting = p
		}
		fv.Plots = append(fv.Plots, pv)
	}
	return fv, rows.Err()
}
