// Package betareset wipes every player at the end of the Closed Beta while
// keeping Admins, configuration, catalogs and the audit log (P0-160, 177..179,
// docs/09 §7). It is an operator tool, never an HTTP endpoint, and refuses to
// run unless every safeguard holds:
//
//   - THANH_ENV is closed_beta (or local for rehearsal); open_beta and
//     production are refused outright (O-05);
//   - the server is in maintenance (no transactions in flight);
//   - the operator typed the confirmation phrase "RESET <database name>";
//   - a backup reference is given (the backup itself is taken before, see
//     deploy/RUNBOOK_CLOSED_BETA_RESET.md).
//
// Without Execute it only counts what would be deleted (dry run).
package betareset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// PlayerTables are counted before and after. Deleting accounts cascades to
// all of them; the ledger and request history are append-only and accept the
// cascade only with thanh.allow_player_data_purge = 'on' in this transaction.
var PlayerTables = []string{
	"accounts", "auth_sessions", "characters", "character_wallets", "inventory_slots", "item_instances",
	"character_equipment", "transaction_requests", "asset_ledger", "farm_plots", "crop_plantings", "pest_incidents",
	"pending_crops", "pending_diamond_awards", "public_chat_messages", "private_chat_messages", "private_chat_blocks",
	"friend_requests", "friendships", "character_daily_quests", "account_codes", "account_recovery_codes", "login_challenges",
	"character_renames",
}

// KeptTables must be untouched by a reset.
var KeptTables = []string{
	"admin_users", "admin_user_roles", "admin_audit_logs", "game_config_versions", "game_config_values",
	"crop_configs", "npc_offers", "npc_definitions", "item_definitions", "crop_definitions", "quest_definitions", "fashion_items",
}

var (
	ErrEnvironment  = errors.New("betareset: refused: THANH_ENV must be closed_beta (or local)")
	ErrMaintenance  = errors.New("betareset: refused: put the server in maintenance first")
	ErrConfirmation = errors.New("betareset: refused: confirmation phrase does not match")
	ErrBackup       = errors.New("betareset: refused: give the reference of the backup taken before the reset")
)

type Options struct {
	Env          string
	Confirmation string
	BackupRef    string
	Operator     string
	Execute      bool
}

type Report struct {
	Database string           `json:"database"`
	Executed bool             `json:"executed"`
	Before   map[string]int64 `json:"before"`
	After    map[string]int64 `json:"after,omitempty"`
	Kept     map[string]int64 `json:"kept"`
}

func count(ctx context.Context, q pgx.Tx, tables []string) (map[string]int64, error) {
	out := map[string]int64{}
	for _, t := range tables {
		var n int64
		if err := q.QueryRow(ctx, `SELECT count(*) FROM `+t).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", t, err)
		}
		out[t] = n
	}
	return out, nil
}

// Run performs the dry run or the reset in one transaction.
func Run(ctx context.Context, conn *pgx.Conn, o Options) (*Report, error) {
	if o.Env != "closed_beta" && o.Env != "local" {
		return nil, ErrEnvironment
	}
	rep := &Report{}
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT current_database()`).Scan(&rep.Database); err != nil {
			return err
		}
		var m struct {
			Active bool `json:"active"`
		}
		var raw json.RawMessage
		if err := tx.QueryRow(ctx, `SELECT value FROM server_state WHERE key = 'maintenance' FOR UPDATE`).Scan(&raw); err != nil {
			return err
		}
		_ = json.Unmarshal(raw, &m)
		var err error
		if rep.Before, err = count(ctx, tx, PlayerTables); err != nil {
			return err
		}
		if rep.Kept, err = count(ctx, tx, KeptTables); err != nil {
			return err
		}
		if !o.Execute {
			return nil
		}
		if !m.Active {
			return ErrMaintenance
		}
		if strings.TrimSpace(o.Confirmation) != "RESET "+rep.Database {
			return ErrConfirmation
		}
		if strings.TrimSpace(o.BackupRef) == "" {
			return ErrBackup
		}
		if _, err := tx.Exec(ctx, `SET LOCAL thanh.allow_player_data_purge = 'on'`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM accounts`); err != nil {
			return fmt.Errorf("delete player data: %w", err)
		}
		if rep.After, err = count(ctx, tx, PlayerTables); err != nil {
			return err
		}
		for t, n := range rep.After {
			if n != 0 {
				return fmt.Errorf("betareset: %s still has %d rows; rolled back", t, n)
			}
		}
		kept, err := count(ctx, tx, KeptTables)
		if err != nil {
			return err
		}
		for t, n := range rep.Kept {
			if kept[t] != n {
				return fmt.Errorf("betareset: kept table %s changed (%d -> %d); rolled back", t, n, kept[t])
			}
		}
		details, _ := json.Marshal(map[string]any{"operator": o.Operator, "backup_ref": o.BackupRef, "before": rep.Before})
		if _, err := tx.Exec(ctx, `INSERT INTO admin_audit_logs (admin_id, action, object_type, object_id, details) VALUES (NULL, 'closed_beta.reset', 'database', $1, $2)`,
			rep.Database, details); err != nil {
			return err
		}
		rep.Executed = true
		return nil
	})
	return rep, err
}
