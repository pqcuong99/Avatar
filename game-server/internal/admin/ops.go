package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/avatar/game-server/internal/textkey"
)

// ---------------------------------------------------------------------------
// Two-person asset adjustments (P0-144)
// ---------------------------------------------------------------------------

type Adjustment struct {
	ID          string          `json:"id"`
	TargetID    *string         `json:"target_character_id"`
	Target      json.RawMessage `json:"target"`
	AssetKind   string          `json:"asset_kind"`
	ItemCode    *string         `json:"item_code"`
	AmountDelta int64           `json:"amount_delta"`
	Reason      string          `json:"reason"`
	Proposer    string          `json:"proposer"`
	Approver    *string         `json:"approver"`
	State       string          `json:"state"`
	Note        *string         `json:"decision_note"`
	Result      json.RawMessage `json:"result"`
	CreatedAt   time.Time       `json:"created_at"`
	DecidedAt   *time.Time      `json:"decided_at"`
}

type Proposal struct {
	CharacterID string `json:"character_id"`
	AssetKind   string `json:"asset_kind"`
	ItemCode    string `json:"item_code"`
	AmountDelta int64  `json:"amount_delta"`
	Reason      string `json:"reason"`
}

func (s *Store) Propose(ctx context.Context, p *Principal, in Proposal) (*Adjustment, error) {
	if !p.Can(PermPropose) {
		return nil, ErrForbidden
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.AmountDelta == 0 || len([]rune(in.Reason)) < 5 || len([]rune(in.Reason)) > 500 ||
		(in.AssetKind != "XU" && in.AssetKind != "DIAMOND" && in.AssetKind != "ITEM") || (in.AssetKind == "ITEM") != (in.ItemCode != "") {
		return nil, ErrInvalidArgument
	}
	var name, account string
	if err := s.Pool.QueryRow(ctx, `SELECT c.display_name, a.username FROM characters c JOIN accounts a ON a.id = c.account_id WHERE c.id = $1`,
		in.CharacterID).Scan(&name, &account); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	snap, _ := json.Marshal(map[string]string{"character_id": in.CharacterID, "display_name": name, "username": account})
	var item any
	if in.ItemCode != "" {
		item = in.ItemCode
	}
	var id string
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO admin_asset_adjustments (target_character_id, target_snapshot, asset_kind, item_code, amount_delta, reason, proposer_id, state, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8) RETURNING id::text`,
			in.CharacterID, snap, in.AssetKind, item, in.AmountDelta, in.Reason, p.ID, s.Now().UTC()).Scan(&id); err != nil {
			return err
		}
		return s.audit(ctx, tx, &p.ID, "asset_adjustment.propose", "asset_adjustment", id, map[string]any{
			"character_id": in.CharacterID, "asset_kind": in.AssetKind, "item_code": in.ItemCode, "amount_delta": in.AmountDelta, "reason": in.Reason})
	})
	if err != nil {
		return nil, err
	}
	return s.Adjustment(ctx, id)
}

func (s *Store) Adjustment(ctx context.Context, id string) (*Adjustment, error) {
	list, err := s.adjustments(ctx, `a.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

func (s *Store) Adjustments(ctx context.Context, state string) ([]Adjustment, error) {
	if state == "" {
		return s.adjustments(ctx, `true`)
	}
	return s.adjustments(ctx, `a.state = $1`, state)
}

func (s *Store) adjustments(ctx context.Context, where string, args ...any) ([]Adjustment, error) {
	rows, err := s.Pool.Query(ctx, `SELECT a.id::text, a.target_character_id::text, a.target_snapshot, a.asset_kind, a.item_code, a.amount_delta,
			a.reason, p.login_name, ap.login_name, a.state, a.decision_note, COALESCE(a.result, 'null'::jsonb), a.created_at, a.decided_at
		FROM admin_asset_adjustments a JOIN admin_users p ON p.id = a.proposer_id LEFT JOIN admin_users ap ON ap.id = a.approver_id
		WHERE `+where+` ORDER BY a.created_at DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Adjustment])
}

// Executor runs an approved adjustment in the Transaction Service. It returns
// the service's JSON answer and whether the change was committed.
type Executor func(ctx context.Context, a *Adjustment) (json.RawMessage, bool, error)

// Decide approves (and executes) or rejects a pending adjustment. The
// approver must be another Admin with the approve permission, even a Super
// Admin cannot approve their own request (T-09). The row lock makes two
// approvals of the same request execute once (T-10).
func (s *Store) Decide(ctx context.Context, p *Principal, id string, approve bool, note string, exec Executor) (*Adjustment, error) {
	if !p.Can(PermApprove) {
		return nil, ErrForbidden
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var proposer, state string
		err := tx.QueryRow(ctx, `SELECT proposer_id::text, state FROM admin_asset_adjustments WHERE id = $1 FOR UPDATE`, id).Scan(&proposer, &state)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if state != "pending" {
			return ErrNotPending
		}
		if proposer == p.ID {
			return ErrSelfApproval
		}
		now := s.Now().UTC()
		if !approve {
			if _, err := tx.Exec(ctx, `UPDATE admin_asset_adjustments SET state = 'rejected', approver_id = $2, decision_note = $3, decided_at = $4 WHERE id = $1`,
				id, p.ID, note, now); err != nil {
				return err
			}
			return s.audit(ctx, tx, &p.ID, "asset_adjustment.reject", "asset_adjustment", id, map[string]any{"note": note})
		}
		adj, err := s.adjustmentTx(ctx, tx, id)
		if err != nil {
			return err
		}
		// Executed while the row is locked; the Transaction Service is
		// idempotent on the adjustment id, so a retry never doubles it.
		result, ok, err := exec(ctx, adj)
		if err != nil {
			return err
		}
		newState := "executed"
		if !ok {
			newState = "failed"
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_asset_adjustments SET state = $2, approver_id = $3, decision_note = $4, result = $5, decided_at = $6 WHERE id = $1`,
			id, newState, p.ID, note, result, now); err != nil {
			return err
		}
		return s.audit(ctx, tx, &p.ID, "asset_adjustment."+newState, "asset_adjustment", id, map[string]any{"note": note, "result": result})
	})
	if err != nil {
		return nil, err
	}
	return s.Adjustment(ctx, id)
}

func (s *Store) adjustmentTx(ctx context.Context, tx pgx.Tx, id string) (*Adjustment, error) {
	a := &Adjustment{ID: id}
	err := tx.QueryRow(ctx, `SELECT target_character_id::text, asset_kind, item_code, amount_delta FROM admin_asset_adjustments WHERE id = $1`, id).
		Scan(&a.TargetID, &a.AssetKind, &a.ItemCode, &a.AmountDelta)
	if err != nil {
		return nil, err
	}
	if a.TargetID == nil {
		return nil, ErrNotFound // the character was deleted (Closed Beta reset)
	}
	return a, nil
}

// ---------------------------------------------------------------------------
// Players
// ---------------------------------------------------------------------------

type PlayerRow struct {
	CharacterID string    `json:"character_id"`
	DisplayName string    `json:"display_name"`
	AccountID   string    `json:"account_id"`
	Username    string    `json:"username"`
	Status      string    `json:"status"`
	BanReason   *string   `json:"ban_reason"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Store) SearchPlayers(ctx context.Context, p *Principal, q string) ([]PlayerRow, error) {
	if !p.Can(PermPlayerRead) {
		return nil, ErrForbidden
	}
	q = strings.TrimSpace(q)
	key := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(textkey.Search(q))
	rows, err := s.Pool.Query(ctx, `SELECT c.id::text, c.display_name, a.id::text, a.username, a.status, a.ban_reason, c.created_at
		FROM characters c JOIN accounts a ON a.id = c.account_id
		WHERE $1 = '' OR c.id::text = $2 OR a.username_key = lower($2) OR c.search_key LIKE '%' || $1 || '%'
		ORDER BY c.created_at DESC LIMIT 50`, key, q)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[PlayerRow])
}

type LedgerRow struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"asset_kind"`
	Ref       *string   `json:"asset_ref"`
	Location  string    `json:"location"`
	Delta     int64     `json:"amount_delta"`
	After     int64     `json:"balance_after"`
	Reason    string    `json:"reason_code"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) Ledger(ctx context.Context, p *Principal, characterID string) ([]LedgerRow, error) {
	if !p.Can(PermPlayerRead) {
		return nil, ErrForbidden
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, asset_kind, asset_ref, location, amount_delta, balance_after, reason_code, created_at
		FROM asset_ledger WHERE character_id = $1 ORDER BY id DESC LIMIT 100`, characterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[LedgerRow])
}

// SetBan bans (with a reason shown to the player, TH-14) or unbans an
// account. Ban types and durations are not decided; this is indefinite until
// lifted. [ĐỀ XUẤT]
func (s *Store) SetBan(ctx context.Context, p *Principal, accountID string, ban bool, reason string) error {
	if !p.Can(PermPlayerBan) {
		return ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if ban && len([]rune(reason)) < 5 {
		return ErrInvalidArgument
	}
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		status, r := "active", any(nil)
		if ban {
			status, r = "banned", reason
		}
		tag, err := tx.Exec(ctx, `UPDATE accounts SET status = $2, ban_reason = $3, updated_at = $4 WHERE id = $1`, accountID, status, r, s.Now().UTC())
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		action := "player.unban"
		if ban {
			action = "player.ban"
		}
		return s.audit(ctx, tx, &p.ID, action, "account", accountID, map[string]any{"reason": reason})
	})
}

// ---------------------------------------------------------------------------
// Maintenance and config publishing (P0-152, TH-04)
// ---------------------------------------------------------------------------

type Maintenance struct {
	Active  bool   `json:"active"`
	Message string `json:"message"`
}

func (s *Store) Maintenance(ctx context.Context) (Maintenance, error) {
	var m Maintenance
	err := s.Pool.QueryRow(ctx, `SELECT value FROM server_state WHERE key = 'maintenance'`).Scan(&m)
	return m, err
}

func (s *Store) SetMaintenance(ctx context.Context, p *Principal, m Maintenance) error {
	if !p.Can(PermMaintenance) {
		return ErrForbidden
	}
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		raw, _ := json.Marshal(m)
		if _, err := tx.Exec(ctx, `UPDATE server_state SET value = $1, updated_at = $2 WHERE key = 'maintenance'`, raw, s.Now().UTC()); err != nil {
			return err
		}
		return s.audit(ctx, tx, &p.ID, "maintenance.set", "server_state", "maintenance", map[string]any{"active": m.Active, "message": m.Message})
	})
}

type Version struct {
	ID          int64      `json:"id"`
	Label       string     `json:"label"`
	State       string     `json:"state"`
	Notes       string     `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at"`
}

func (s *Store) Versions(ctx context.Context, p *Principal) ([]Version, error) {
	if !p.Can(PermPublish) {
		return nil, ErrForbidden
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, label, state, notes, created_at, published_at FROM game_config_versions ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Version])
}

// Draft is the editable content of a config version.
type Draft struct {
	Values map[string]json.RawMessage `json:"values"`
	Crops  []struct {
		CropCode        string  `json:"crop_code"`
		GrowthSeconds   int     `json:"growth_seconds"`
		BaseYield       int     `json:"base_yield"`
		PestProbability float64 `json:"pest_probability"`
	} `json:"crops"`
	Offers []struct {
		NPCCode   string `json:"npc_code"`
		ItemCode  string `json:"item_code"`
		Direction string `json:"direction"`
		Currency  string `json:"currency"`
		UnitPrice int64  `json:"unit_price"`
		SortOrder int    `json:"sort_order"`
	} `json:"offers"`
	Quests []struct {
		QuestCode      string `json:"quest_code"`
		Title          string `json:"title"`
		EventType      string `json:"event_type"`
		TargetQuantity int    `json:"target_quantity"`
		RewardCurrency string `json:"reward_currency"`
		RewardAmount   int64  `json:"reward_amount"`
		SortOrder      int    `json:"sort_order"`
	} `json:"quests"`
}

// ReadVersion returns any version's content as a Draft document.
func (s *Store) ReadVersion(ctx context.Context, p *Principal, id int64) (*Draft, error) {
	if !p.Can(PermPublish) {
		return nil, ErrForbidden
	}
	d := &Draft{Values: map[string]json.RawMessage{}}
	rows, err := s.Pool.Query(ctx, `SELECT key, value FROM game_config_values WHERE config_version_id = $1 ORDER BY key`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k string
		var v json.RawMessage
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return nil, err
		}
		d.Values[k] = v
	}
	rows.Close()
	if err := scanInto(ctx, s, &d.Crops, `SELECT crop_code, growth_seconds, base_yield, pest_probability::float8 FROM crop_configs WHERE config_version_id = $1 ORDER BY crop_code`, id); err != nil {
		return nil, err
	}
	if err := scanInto(ctx, s, &d.Offers, `SELECT npc_code, item_code, direction, currency, unit_price, sort_order FROM npc_offers WHERE config_version_id = $1 ORDER BY npc_code, direction, sort_order`, id); err != nil {
		return nil, err
	}
	if err := scanInto(ctx, s, &d.Quests, `SELECT quest_code, title, event_type, target_quantity, reward_currency, reward_amount, sort_order FROM quest_definitions WHERE config_version_id = $1 ORDER BY sort_order`, id); err != nil {
		return nil, err
	}
	return d, nil
}

func scanInto[T any](ctx context.Context, s *Store, dst *[]T, sql string, args ...any) error {
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[T])
	*dst = out
	return err
}

// SaveDraft creates (id 0) or replaces a draft version's content. Published
// and retired versions can never change (database triggers enforce it too).
func (s *Store) SaveDraft(ctx context.Context, p *Principal, id int64, label, notes string, d *Draft) (int64, error) {
	if !p.Can(PermPublish) {
		return 0, ErrForbidden
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if id == 0 {
			if strings.TrimSpace(label) == "" {
				return fmt.Errorf("%w: label required", ErrInvalidDraft)
			}
			if err := tx.QueryRow(ctx, `INSERT INTO game_config_versions (label, state, notes) VALUES ($1, 'draft', $2) RETURNING id`, label, notes).Scan(&id); err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidDraft, err)
			}
		} else {
			var state string
			if err := tx.QueryRow(ctx, `SELECT state FROM game_config_versions WHERE id = $1 FOR UPDATE`, id).Scan(&state); err != nil {
				return ErrNotFound
			}
			if state != "draft" {
				return fmt.Errorf("%w: version %d is %s", ErrInvalidDraft, id, state)
			}
			for _, t := range []string{"game_config_values", "crop_configs", "npc_offers", "quest_definitions"} {
				if _, err := tx.Exec(ctx, `DELETE FROM `+t+` WHERE config_version_id = $1`, id); err != nil {
					return err
				}
			}
		}
		for k, v := range d.Values {
			if _, err := tx.Exec(ctx, `INSERT INTO game_config_values (config_version_id, key, value) VALUES ($1, $2, $3)`, id, k, v); err != nil {
				return fmt.Errorf("%w: value %s: %v", ErrInvalidDraft, k, err)
			}
		}
		for _, c := range d.Crops {
			if _, err := tx.Exec(ctx, `INSERT INTO crop_configs (config_version_id, crop_code, growth_seconds, base_yield, pest_probability) VALUES ($1, $2, $3, $4, $5)`,
				id, c.CropCode, c.GrowthSeconds, c.BaseYield, c.PestProbability); err != nil {
				return fmt.Errorf("%w: crop %s: %v", ErrInvalidDraft, c.CropCode, err)
			}
		}
		for _, o := range d.Offers {
			if _, err := tx.Exec(ctx, `INSERT INTO npc_offers (config_version_id, npc_code, item_code, direction, currency, unit_price, sort_order) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				id, o.NPCCode, o.ItemCode, o.Direction, o.Currency, o.UnitPrice, o.SortOrder); err != nil {
				return fmt.Errorf("%w: offer %s/%s: %v", ErrInvalidDraft, o.NPCCode, o.ItemCode, err)
			}
		}
		for _, q := range d.Quests {
			if _, err := tx.Exec(ctx, `INSERT INTO quest_definitions (config_version_id, quest_code, title, event_type, target_quantity, reward_currency, reward_amount, sort_order)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, id, q.QuestCode, q.Title, q.EventType, q.TargetQuantity, q.RewardCurrency, q.RewardAmount, q.SortOrder); err != nil {
				return fmt.Errorf("%w: quest %s: %v", ErrInvalidDraft, q.QuestCode, err)
			}
		}
		return s.audit(ctx, tx, &p.ID, "config.save_draft", "config_version", fmt.Sprint(id), map[string]any{"label": label})
	})
	return id, err
}

// Publish makes a draft the live configuration. It is refused unless the
// server is in maintenance (O-06): new prices never appear mid-session.
func (s *Store) Publish(ctx context.Context, p *Principal, id int64) error {
	if !p.Can(PermPublish) {
		return ErrForbidden
	}
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var m Maintenance
		if err := tx.QueryRow(ctx, `SELECT value FROM server_state WHERE key = 'maintenance' FOR UPDATE`).Scan(&m); err != nil {
			return err
		}
		if !m.Active {
			return ErrMaintenanceOff
		}
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM game_config_versions WHERE id = $1 FOR UPDATE`, id).Scan(&state); err != nil {
			return ErrNotFound
		}
		if state != "draft" {
			return fmt.Errorf("%w: version %d is %s", ErrInvalidDraft, id, state)
		}
		var previous *int64
		_ = tx.QueryRow(ctx, `SELECT id FROM game_config_versions WHERE state = 'published'`).Scan(&previous)
		if _, err := tx.Exec(ctx, `UPDATE game_config_versions SET state = 'retired' WHERE state = 'published'`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE game_config_versions SET state = 'published', published_at = $2 WHERE id = $1`, id, s.Now().UTC()); err != nil {
			return err
		}
		return s.audit(ctx, tx, &p.ID, "config.publish", "config_version", fmt.Sprint(id), map[string]any{"previous": previous})
	})
}
