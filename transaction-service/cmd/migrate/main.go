// Command migrate applies database/migrations, plus database/seeds/local when
// THANH_ENV=local. It never drops or rewrites anything that already exists.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/avatar/transaction-service/internal/migrate"
)

func main() {
	migrations := flag.String("migrations", "database/migrations", "directory of numbered migration files")
	seeds := flag.String("local-seeds", "database/seeds/local", "directory of DEV SAMPLE seed files (THANH_ENV=local only)")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := connectWithRetry(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(context.Background())

	sets := []migrate.Set{{Dir: *migrations, Table: "schema_migrations"}}
	if os.Getenv("THANH_ENV") == "local" {
		sets = append(sets, migrate.Set{Dir: *seeds, Table: "schema_seeds"})
		log.Printf("THANH_ENV=local: DEV SAMPLE seeds from %s will be applied", *seeds)
	}
	applied, err := migrate.Apply(ctx, conn, sets...)
	for _, name := range applied {
		log.Printf("applied %s", name)
	}
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("database is up to date (%d file(s) applied now)", len(applied))
}

// The database container may still be starting when this runs.
func connectWithRetry(ctx context.Context, dsn string) (*pgx.Conn, error) {
	for {
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			return conn, nil
		}
		log.Printf("database not reachable yet: %v", err)
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(2 * time.Second):
		}
	}
}
