// Package testdb gives integration tests a fresh, fully migrated PostgreSQL
// database of their own. Set TEST_DATABASE_URL to a role that may CREATE
// DATABASE (the local compose postgres works); without it the tests skip.
// The databases are named thanh_test_* and dropped after the test; nothing
// touches the development database itself.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-thanh/transaction-service/internal/migrate"
)

// New creates and migrates a database and returns a pool connected to it.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	admin := os.Getenv("TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("TEST_DATABASE_URL not set: PostgreSQL integration test skipped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var b [6]byte
	_, _ = rand.Read(b[:])
	name := "thanh_test_" + hex.EncodeToString(b[:])

	adminConn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := adminConn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	adminConn.Close(ctx)

	u, err := url.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dsn := u.String()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if _, err := migrate.Apply(ctx, conn, migrate.Set{Dir: MigrationsDir(t), Table: "schema_migrations"}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	conn.Close(ctx)

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 40
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if c, err := pgx.Connect(ctx, admin); err == nil {
			_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			c.Close(ctx)
		}
	})
	return pool
}

// MigrationsDir finds database/migrations from the test's working directory.
func MigrationsDir(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "database", "migrations")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("database/migrations not found above the test directory")
		}
		dir = parent
	}
}

// CreateAccount inserts an account row the way the auth service would.
func CreateAccount(t testing.TB, pool *pgxpool.Pool, username string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO accounts (username, username_key, password_hash)
		VALUES ($1, $2, 'test-not-a-real-hash') RETURNING id::text`, username, strings.ToLower(username)).Scan(&id)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return id
}

// Crop is one crop_configs row of a test config version.
type Crop struct {
	Code            string
	GrowthSeconds   int
	BaseYield       int
	PestProbability float64
}

// Offer is one npc_offers row of a test config version.
type Offer struct {
	NPC, Item, Direction, Currency string
	Price                          int64
}

// Quest is one quest_definitions row of a test config version.
type Quest struct {
	Code, EventType, Currency string
	Target                    int
	Reward                    int64
}

// Config describes a whole config version for tests.
type Config struct {
	Label  string
	Values map[string]any
	Crops  []Crop
	Offers []Offer
	Quests []Quest
}

// Publish inserts the version as draft, fills it, retires the currently
// published version and publishes this one — what maintenance would do.
// It returns the new version id.
func Publish(t testing.TB, pool *pgxpool.Pool, c Config) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO npc_definitions (npc_code, display_name, functions)
			VALUES ('FARM_SHOP', 'Test Farm Shop', '{buy_seeds,sell_crops}') ON CONFLICT DO NOTHING`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO game_config_versions (label, state) VALUES ($1, 'draft') RETURNING id`, c.Label).Scan(&id); err != nil {
			return err
		}
		for k, v := range c.Values {
			raw, _ := json.Marshal(v)
			if _, err := tx.Exec(ctx, `INSERT INTO game_config_values (config_version_id, key, value) VALUES ($1, $2, $3)`, id, k, raw); err != nil {
				return err
			}
		}
		for _, cr := range c.Crops {
			if _, err := tx.Exec(ctx, `INSERT INTO crop_configs (config_version_id, crop_code, growth_seconds, base_yield, pest_probability)
				VALUES ($1, $2, $3, $4, $5)`, id, cr.Code, cr.GrowthSeconds, cr.BaseYield, cr.PestProbability); err != nil {
				return err
			}
		}
		for i, o := range c.Offers {
			if _, err := tx.Exec(ctx, `INSERT INTO npc_offers (config_version_id, npc_code, item_code, direction, currency, unit_price, sort_order)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, o.NPC, o.Item, o.Direction, o.Currency, o.Price, i); err != nil {
				return err
			}
		}
		for i, q := range c.Quests {
			if _, err := tx.Exec(ctx, `INSERT INTO quest_definitions (config_version_id, quest_code, title, event_type, target_quantity, reward_currency, reward_amount, sort_order)
				VALUES ($1, $2, $2, $3, $4, $5, $6, $7)`, id, q.Code, q.EventType, q.Target, q.Currency, q.Reward, i); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE game_config_versions SET state = 'retired' WHERE state = 'published'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE game_config_versions SET state = 'published', published_at = now() WHERE id = $1`, id)
		return err
	})
	if err != nil {
		t.Fatalf("publish config %s: %v", c.Label, err)
	}
	return id
}

// Draft inserts a version that stays a draft (never effective).
func Draft(t testing.TB, pool *pgxpool.Pool, c Config) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO game_config_versions (label, state) VALUES ($1, 'draft') RETURNING id`, c.Label).Scan(&id); err != nil {
			return err
		}
		for _, o := range c.Offers {
			if _, err := tx.Exec(ctx, `INSERT INTO npc_offers (config_version_id, npc_code, item_code, direction, currency, unit_price)
				VALUES ($1, $2, $3, $4, $5, $6)`, id, o.NPC, o.Item, o.Direction, o.Currency, o.Price); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("draft config %s: %v", c.Label, err)
	}
	return id
}

// Exec runs a statement for test setup and fails the test on error.
func Exec(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// Int64 runs a single-value query.
func Int64(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) int64 {
	t.Helper()
	var v int64
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

// Sprintf is a tiny convenience for building ids in tests.
func Sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }
