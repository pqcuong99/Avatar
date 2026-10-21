// Package migrate applies the numbered SQL files of database/migrations (and,
// for local development only, database/seeds/local) exactly once each.
//
// Every file runs inside its own transaction together with the bookkeeping row,
// so a failing file leaves no partial schema behind. An applied file whose
// content changed is an error: applied migrations are never edited, a new
// numbered file is added instead (docs/06 §11).
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"
)

// Set is one directory of numbered SQL files with its own bookkeeping table.
type Set struct {
	Dir   string
	Table string // "schema_migrations" or "schema_seeds"
}

var fileName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

// advisoryLockKey serializes concurrent runners (two containers starting).
const advisoryLockKey = 7_204_431_001

type file struct {
	version  string
	name     string
	sql      string
	checksum string
}

func load(dir string) ([]file, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []file
	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		m := fileName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("%s: name must look like 0001_snake_case.sql", e.Name())
		}
		if other, dup := seen[m[1]]; dup {
			return nil, fmt.Errorf("%s and %s share version %s", other, e.Name(), m[1])
		}
		seen[m[1]] = e.Name()
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(body)
		files = append(files, file{version: m[1], name: e.Name(), sql: string(body), checksum: hex.EncodeToString(sum[:])})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

// Apply runs every pending file of each set, in order. It returns the names of
// the files it applied.
func Apply(ctx context.Context, conn *pgx.Conn, sets ...Set) ([]string, error) {
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey); err != nil {
		return nil, err
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, advisoryLockKey) //nolint:errcheck

	var applied []string
	for _, set := range sets {
		names, err := applySet(ctx, conn, set)
		applied = append(applied, names...)
		if err != nil {
			return applied, err
		}
	}
	return applied, nil
}

func applySet(ctx context.Context, conn *pgx.Conn, set Set) ([]string, error) {
	if set.Table != "schema_migrations" && set.Table != "schema_seeds" {
		return nil, fmt.Errorf("unexpected bookkeeping table %q", set.Table)
	}
	files, err := load(set.Dir)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+set.Table+` (
		version TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return nil, err
	}
	done := map[string]string{}
	rows, err := conn.Query(ctx, `SELECT version, checksum FROM `+set.Table)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v, c string
		if err := rows.Scan(&v, &c); err != nil {
			rows.Close()
			return nil, err
		}
		done[v] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var applied []string
	for _, f := range files {
		if sum, ok := done[f.version]; ok {
			if sum != f.checksum {
				return applied, fmt.Errorf("%s was already applied with different content; add a new numbered file instead of editing it", f.name)
			}
			continue
		}
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, f.sql); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO `+set.Table+` (version, name, checksum) VALUES ($1, $2, $3)`, f.version, f.name, f.checksum)
			return err
		})
		if err != nil {
			return applied, fmt.Errorf("%s: %w", f.name, err)
		}
		applied = append(applied, f.name)
	}
	return applied, nil
}
