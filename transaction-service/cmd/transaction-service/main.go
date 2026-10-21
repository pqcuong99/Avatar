// Command transaction-service is the only writer of player assets (P0-165).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // quest reset time zones on minimal images

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-thanh/transaction-service/internal/econ"
	"example.com/project-thanh/transaction-service/internal/httpapi"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	addr := envOr("TRANSACTION_SERVICE_ADDR", ":8082")
	dsn := os.Getenv("DATABASE_URL")
	token := os.Getenv("INTERNAL_API_TOKEN")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	if len(token) < 16 {
		return errors.New("INTERNAL_API_TOKEN is required (at least 16 characters)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	svc, err := econ.New(ctx, pool)
	if err != nil {
		return err
	}
	svc.RequireSyncStandby = os.Getenv("REQUIRE_SYNC_STANDBY") == "true"

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(svc, pool, token, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("transaction-service listening", "addr", addr, "require_sync_standby", svc.RequireSyncStandby)
		errc <- server.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
