package betareset_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/avatar/transaction-service/internal/betareset"
	"example.com/avatar/transaction-service/internal/econ"
	"example.com/avatar/transaction-service/internal/testdb"
)

func TestO04O05ClosedBetaReset(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	testdb.Publish(t, pool, testdb.Config{Label: "CFG", Values: map[string]any{"wallet.diamond_cap": 100}})
	svc, err := econ.New(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	// Two players with assets, history and social data; one Admin.
	var chars []string
	for _, n := range []string{"alpha", "beta"} {
		acc := testdb.CreateAccount(t, pool, n)
		c, _, err := svc.CreateCharacter(ctx, acc, "Nhân "+n, "create-"+n+"-0001")
		if err != nil {
			t.Fatal(err)
		}
		chars = append(chars, c.ID)
	}
	testdb.Exec(t, pool, `INSERT INTO friendships (character_a, character_b) SELECT LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid)`, chars[0], chars[1])
	testdb.Exec(t, pool, `INSERT INTO private_chat_messages (sender_id, recipient_id, request_id, body, expires_at) VALUES ($1, $2, 'req-00000001', 'hi', now() + interval '30 days')`, chars[0], chars[1])
	testdb.Exec(t, pool, `INSERT INTO admin_users (login_name, password_hash, totp_secret_enc) VALUES ('ops', 'x', '\x00')`)
	testdb.Exec(t, pool, `INSERT INTO admin_asset_adjustments (target_character_id, target_snapshot, asset_kind, amount_delta, reason, proposer_id, state)
		SELECT $1, '{}', 'XU', 10, 'test reason', id, 'pending' FROM admin_users`, chars[0])

	conn := connect(t, pool)
	var db string
	_ = conn.QueryRow(ctx, `SELECT current_database()`).Scan(&db)

	// O-05: never on Open Beta / production.
	for _, env := range []string{"open_beta", "production", ""} {
		if _, err := betareset.Run(ctx, conn, betareset.Options{Env: env, Execute: true}); !errors.Is(err, betareset.ErrEnvironment) {
			t.Fatalf("env %q: %v", env, err)
		}
	}
	// Dry run deletes nothing.
	rep, err := betareset.Run(ctx, conn, betareset.Options{Env: "closed_beta"})
	if err != nil || rep.Executed || rep.Before["accounts"] != 2 || rep.Before["asset_ledger"] < 2 {
		t.Fatalf("dry run %+v %v", rep, err)
	}
	opts := betareset.Options{Env: "closed_beta", Execute: true, Confirmation: "RESET " + db, BackupRef: "backup-test.dump", Operator: "test"}
	if _, err := betareset.Run(ctx, conn, opts); !errors.Is(err, betareset.ErrMaintenance) {
		t.Fatalf("without maintenance: %v", err)
	}
	testdb.Exec(t, pool, `UPDATE server_state SET value = '{"active": true, "message": "reset"}' WHERE key = 'maintenance'`)
	bad := opts
	bad.Confirmation = "RESET something_else"
	if _, err := betareset.Run(ctx, conn, bad); !errors.Is(err, betareset.ErrConfirmation) {
		t.Fatalf("wrong phrase: %v", err)
	}
	noBackup := opts
	noBackup.BackupRef = ""
	if _, err := betareset.Run(ctx, conn, noBackup); !errors.Is(err, betareset.ErrBackup) {
		t.Fatalf("no backup: %v", err)
	}

	// O-04: player data gone, Admin and config kept.
	rep, err = betareset.Run(ctx, conn, opts)
	if err != nil || !rep.Executed {
		t.Fatalf("reset: %+v %v", rep, err)
	}
	for _, tbl := range betareset.PlayerTables {
		if n := testdb.Int64(t, pool, `SELECT count(*) FROM `+tbl); n != 0 {
			t.Fatalf("%s has %d rows", tbl, n)
		}
	}
	if n := testdb.Int64(t, pool, `SELECT count(*) FROM admin_users`); n != 1 {
		t.Fatal("admin lost")
	}
	if n := testdb.Int64(t, pool, `SELECT count(*) FROM game_config_versions WHERE state = 'published'`); n != 1 {
		t.Fatal("config lost")
	}
	if n := testdb.Int64(t, pool, `SELECT count(*) FROM admin_asset_adjustments WHERE target_character_id IS NULL`); n != 1 {
		t.Fatal("adjustment record must survive with its snapshot")
	}
	if n := testdb.Int64(t, pool, `SELECT count(*) FROM admin_audit_logs WHERE action = 'closed_beta.reset'`); n != 1 {
		t.Fatal("reset not audited")
	}
	// The purge switch was local to that transaction: the ledger is append-only again.
	testdb.Exec(t, pool, `UPDATE server_state SET value = '{"active": false, "message": ""}' WHERE key = 'maintenance'`)
	acc := testdb.CreateAccount(t, pool, "gamma")
	c, _, err := svc.CreateCharacter(ctx, acc, "Nhân gamma", "create-gamma-01")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM asset_ledger WHERE character_id = $1`, c.ID)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("ledger delete after reset: %v", err)
	}
}

func connect(t *testing.T, pool *pgxpool.Pool) *pgx.Conn {
	t.Helper()
	conn, err := pgx.ConnectConfig(context.Background(), pool.Config().ConnConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}
