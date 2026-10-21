// Command game-server runs accounts/sessions, the public HTTP API gateway to
// the Transaction Service and the realtime world over WebSocket.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/avatar/game-server/internal/admin"
	"example.com/avatar/game-server/internal/api"
	"example.com/avatar/game-server/internal/authn"
	"example.com/avatar/game-server/internal/gamedb"
	"example.com/avatar/game-server/internal/httpx"
	"example.com/avatar/game-server/internal/mail"
	"example.com/avatar/game-server/internal/mapdata"
	"example.com/avatar/game-server/internal/realtime"
	"example.com/avatar/game-server/internal/secretbox"
	"example.com/avatar/game-server/internal/social"
	"example.com/avatar/game-server/internal/txclient"
	"example.com/avatar/game-server/internal/world"
)

const cookieName = "thanh_session"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	addr := envOr("GAME_SERVER_ADDR", ":8081")
	dsn := os.Getenv("DATABASE_URL")
	token := os.Getenv("INTERNAL_API_TOKEN")
	txURL := envOr("TRANSACTION_SERVICE_URL", "http://localhost:8082")
	mapDir := envOr("MAP_DIR", "../shared/maps")
	origins := splitList(envOr("ALLOWED_ORIGINS", "http://localhost:5173"))
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	if len(token) < 16 {
		return errors.New("INTERNAL_API_TOKEN is required (at least 16 characters)")
	}
	cookieSecure := os.Getenv("COOKIE_SECURE") == "true"
	if os.Getenv("THANH_ENV") != "local" && !cookieSecure {
		return errors.New("COOKIE_SECURE=true is required outside THANH_ENV=local (HTTPS only)")
	}

	maps, err := mapdata.Load(mapDir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	db := &gamedb.Store{Pool: pool}

	w := world.New(world.DefaultConfig(), maps, func(ps []gamedb.Position) {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.SavePositions(c, ps); err != nil {
			log.Error("save positions", "err", err, "count", len(ps))
		}
	}, log)
	worldCtx, stopWorld := context.WithCancel(context.Background())
	defer stopWorld()
	worldDone := make(chan struct{})
	go func() { w.Run(worldCtx); close(worldDone) }()
	go refreshRuntime(ctx, db, w, log)

	auth := authn.NewStore(pool)
	sec, err := securityFromEnv(log)
	if err != nil {
		return err
	}
	limiter, registerLimit := authn.NewLimiter(), authn.NewLimiter()
	registerLimit.PerIPPerMinute = 5 // [ĐỀ XUẤT] Closed Beta allows free sign-up (P0-178)
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				limiter.Sweep()
				registerLimit.Sweep()
			}
		}
	}()

	soc := social.New(pool)
	go purgeExpiredMessages(ctx, soc, log)

	tx := txclient.New(txURL, token)
	a := &api.API{Auth: auth, Sec: sec, Limiter: limiter, RegisterLimit: registerLimit, DB: db, Social: soc, Tx: tx, World: w, Maps: maps,
		Log: log, CookieName: cookieName, CookieSecure: cookieSecure, AllowedOrigins: origins,
		TrustProxy: os.Getenv("TRUST_PROXY_HEADERS") == "true"}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(rw http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(rw, http.StatusOK, map[string]string{"service": "game-server", "status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(rw http.ResponseWriter, r *http.Request) {
		c, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(c); err != nil {
			httpx.WriteJSON(rw, http.StatusServiceUnavailable, map[string]string{"service": "game-server", "status": "database-unavailable"})
			return
		}
		if status, _, err := tx.Do(c, http.MethodGet, "/readyz", nil); err != nil || status != http.StatusOK {
			httpx.WriteJSON(rw, http.StatusServiceUnavailable, map[string]string{"service": "game-server", "status": "transaction-service-unavailable"})
			return
		}
		httpx.WriteJSON(rw, http.StatusOK, map[string]string{"service": "game-server", "status": "ready"})
	})
	a.Routes(mux)
	adminLimiter := authn.NewLimiter()
	adminLimiter.FreeFailures = 3
	(&admin.Handler{
		Store: admin.New(pool, sec.Box), Tx: tx, World: w, Limiter: adminLimiter, Log: log,
		CookieSecure: cookieSecure, AllowedOrigins: origins,
		RefreshRuntime: func(c context.Context) {
			if r, err := db.LoadRuntime(c); err == nil {
				w.SetRuntime(r)
			}
		},
	}).Routes(mux)
	mux.Handle("GET /ws", &realtime.Handler{World: w, Auth: auth, DB: db, Social: soc, Tx: tx, CookieName: cookieName,
		OriginPatterns: hostsOf(origins), Log: log})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("game-server listening", "addr", addr, "maps", len(maps.Maps), "origins", origins)
		errc <- server.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = server.Shutdown(shutdown)
	stopWorld() // saves every position before returning
	<-worldDone
	return err
}

// refreshRuntime reloads the published config the world uses. It only
// changes through maintenance, so a slow poll is enough.
func refreshRuntime(ctx context.Context, db *gamedb.Store, w *world.World, log *slog.Logger) {
	load := func() {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		r, err := db.LoadRuntime(c)
		if err != nil {
			log.Error("load runtime config", "err", err)
			return
		}
		if r.RoomCapacity <= 0 {
			log.Warn("world.room_capacity is not configured: players cannot enter the map until an Admin publishes it")
		}
		w.SetRuntime(r)
	}
	load()
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			load()
		}
	}
}

// securityFromEnv sets up TOTP secret encryption and email. Outside local
// development a real key is required, and email stays off until a provider
// is configured (GAP-16).
func securityFromEnv(log *slog.Logger) (*authn.Security, error) {
	sec := &authn.Security{Issuer: "Avatar"}
	local := os.Getenv("THANH_ENV") == "local"
	if k := os.Getenv("TOTP_ENCRYPTION_KEY"); k != "" {
		box, err := secretbox.New(k)
		if err != nil {
			return nil, err
		}
		sec.Box = box
	} else if local {
		log.Warn("TOTP_ENCRYPTION_KEY not set: using the fixed DEVELOPMENT key (THANH_ENV=local)")
		sec.Box = secretbox.DevBox()
	} else {
		return nil, errors.New("TOTP_ENCRYPTION_KEY is required (32 random bytes, base64)")
	}
	if local {
		sec.Mail = mail.LogSender{Log: log}
	}
	return sec, nil
}

// purgeExpiredMessages reclaims space from private messages past their
// 30 days; reads already ignore them (TH-05-001).
func purgeExpiredMessages(ctx context.Context, soc *social.Service, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		c, cancel := context.WithTimeout(ctx, time.Minute)
		if n, err := soc.PurgeExpired(c); err != nil {
			log.Error("purge expired private messages", "err", err)
		} else if n > 0 {
			log.Info("purged expired private messages", "count", n)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// hostsOf turns origins into the host patterns the WebSocket library checks.
func hostsOf(origins []string) []string {
	var out []string
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
}
