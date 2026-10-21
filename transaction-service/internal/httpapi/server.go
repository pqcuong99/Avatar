// Package httpapi exposes the Transaction Service to the Game Server only.
// Every /internal route requires the shared X-Internal-Token; players never
// reach this service directly (deploy keeps its port off the public network).
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-thanh/transaction-service/internal/apperr"
	"example.com/project-thanh/transaction-service/internal/econ"
)

type Server struct {
	svc   *econ.Service
	pool  *pgxpool.Pool
	token string
	log   *slog.Logger
}

func New(svc *econ.Service, pool *pgxpool.Pool, token string, log *slog.Logger) *Server {
	return &Server{svc: svc, pool: pool, token: token, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "transaction-service", "status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"service": "transaction-service", "status": "database-unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"service": "transaction-service", "status": "ready"})
	})

	internal := http.NewServeMux()
	internal.HandleFunc("GET /internal/v1/catalog", s.catalog)
	internal.HandleFunc("POST /internal/v1/characters", s.createCharacter)
	internal.HandleFunc("GET /internal/v1/accounts/{accountID}/character", s.characterByAccount)
	internal.HandleFunc("GET /internal/v1/characters/{characterID}/state", s.state)
	internal.HandleFunc("GET /internal/v1/characters/{characterID}/npc/{npcCode}/offers", s.offers)
	internal.HandleFunc("GET /internal/v1/characters/{characterID}/quests", s.quests)
	internal.HandleFunc("POST /internal/v1/characters/{characterID}/commands/{op}", s.command)
	mux.Handle("/internal/", s.requireToken(internal))
	return mux
}

func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Internal-Token")
		if s.token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeError(w, apperr.Unauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		next.ServeHTTP(w, r)
	})
}

type okEnvelope struct {
	OK            bool   `json:"ok"`
	Result        any    `json:"result,omitempty"`
	Replayed      bool   `json:"replayed,omitempty"`
	State         any    `json:"state,omitempty"`
	ServerTimeUTC string `json:"server_time_utc"`
}

func writeOK(w http.ResponseWriter, result any, replayed bool, state any) {
	writeJSON(w, http.StatusOK, okEnvelope{OK: true, Result: result, Replayed: replayed, State: state,
		ServerTimeUTC: time.Now().UTC().Format(time.RFC3339Nano)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, e *apperr.Error) {
	writeJSON(w, e.Status, map[string]any{"ok": false, "error": e})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := apperr.As(err); ok {
		writeError(w, e)
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	s.log.Error("internal error", "path", r.URL.Path, "err", err)
	writeError(w, apperr.Internal)
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	cat, err := s.svc.PublicCatalog(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeOK(w, cat, false, nil)
}

func (s *Server) createCharacter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID   string `json:"account_id"`
		DisplayName string `json:"display_name"`
		RequestID   string `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperr.Validation)
		return
	}
	c, replayed, err := s.svc.CreateCharacter(r.Context(), body.AccountID, body.DisplayName, body.RequestID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeOK(w, c, replayed, nil)
}

func (s *Server) characterByAccount(w http.ResponseWriter, r *http.Request) {
	c, err := s.svc.CharacterByAccount(r.Context(), r.PathValue("accountID"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeOK(w, c, false, nil)
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.State(r.Context(), r.PathValue("characterID"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeOK(w, st, false, nil)
}

func (s *Server) offers(w http.ResponseWriter, r *http.Request) {
	v, err := s.svc.NPCOffers(r.Context(), r.PathValue("characterID"), r.PathValue("npcCode"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeOK(w, v, false, nil)
}

func (s *Server) quests(w http.ResponseWriter, r *http.Request) {
	v, err := s.svc.Quests(r.Context(), r.PathValue("characterID"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeOK(w, v, false, nil)
}

// command runs one asset operation and answers with its outcome plus a fresh
// state snapshot read after COMMIT (replays get a fresh snapshot too).
func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	op := r.PathValue("op")
	if !econ.IsCommand(op) {
		writeError(w, apperr.NotFound)
		return
	}
	var body struct {
		RequestID string          `json:"request_id"`
		Args      json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperr.Validation)
		return
	}
	charID := r.PathValue("characterID")
	out, err := s.svc.Command(r.Context(), charID, body.RequestID, op, body.Args)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	st, err := s.svc.State(r.Context(), charID)
	if err != nil {
		// The command is committed; report it even if the snapshot failed.
		s.log.Error("state after command", "op", op, "err", err)
		st = nil
	}
	writeOK(w, out.Result, out.Replayed, st)
}
