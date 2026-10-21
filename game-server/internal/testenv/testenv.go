// Package testenv gives Game Server integration tests a fresh PostgreSQL
// database with every migration of database/migrations applied in order.
// Set TEST_DATABASE_URL to a role that may CREATE DATABASE; without it the
// tests skip. Databases are named thanh_test_* and dropped afterwards.
package testenv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepoPath finds a path relative to the repository root.
func RepoPath(t testing.TB, parts ...string) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		p := filepath.Join(append([]string{dir}, parts...)...)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%v not found above the test directory", parts)
		}
		dir = parent
	}
}

func DB(t testing.TB) *pgxpool.Pool {
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
	c, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := c.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	c.Close(ctx)
	u, _ := url.Parse(admin)
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
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
	dir := RepoPath(t, "database", "migrations")
	entries, _ := os.ReadDir(dir)
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	return pool
}

// Character inserts an account and its character directly (what the auth
// and transaction services would have done) and returns their ids.
func Character(t testing.TB, pool *pgxpool.Pool, accountID, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO characters (account_id, display_name, display_name_key, search_key)
		VALUES ($1, $2, lower($2::text), lower($2::text)) RETURNING id::text`, accountID, name).Scan(&id)
	if err != nil {
		t.Fatalf("insert character: %v", err)
	}
	return id
}
