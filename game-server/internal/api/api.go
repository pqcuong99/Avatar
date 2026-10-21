// Package api is the public HTTP API of docs/07 served by the Game Server.
// Account and session routes are handled here; every asset route is checked
// (session, character, NPC proximity) and then forwarded to the Transaction
// Service, which alone changes assets (P0-165). The client never sends prices,
// owners or yields: only intents.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"example.com/project-thanh/game-server/internal/authn"
	"example.com/project-thanh/game-server/internal/gamedb"
	"example.com/project-thanh/game-server/internal/httpx"
	"example.com/project-thanh/game-server/internal/mapdata"
	"example.com/project-thanh/game-server/internal/social"
	"example.com/project-thanh/game-server/internal/txclient"
	"example.com/project-thanh/game-server/internal/world"
)

type API struct {
	Auth           *authn.Store
	Sec            *authn.Security
	Limiter        *authn.Limiter
	RegisterLimit  *authn.Limiter
	DB             *gamedb.Store
	Social         *social.Service
	Tx             *txclient.Client
	World          *world.World
	Maps           *mapdata.Registry
	Log            *slog.Logger
	CookieName     string
	CookieSecure   bool
	AllowedOrigins []string // full origins, e.g. http://localhost:5173
	TrustProxy     bool
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/logout", a.logout)
	mux.HandleFunc("GET /api/v1/auth/me", a.me)
	mux.HandleFunc("GET /api/v1/auth/health", a.health)

	mux.HandleFunc("POST /api/v1/characters", a.withAccount(a.createCharacter))
	mux.HandleFunc("GET /api/v1/characters/me", a.withCharacter(a.characterMe))
	mux.HandleFunc("GET /api/v1/catalog", a.withCharacter(a.forwardGet("/internal/v1/catalog")))
	mux.HandleFunc("GET /api/v1/state", a.withCharacter(a.state))
	mux.HandleFunc("GET /api/v1/npc/{npcCode}/offers", a.withCharacter(a.offers))
	mux.HandleFunc("POST /api/v1/npc/{npcCode}/buy", a.withCharacter(a.trade("npc.buy")))
	mux.HandleFunc("POST /api/v1/npc/{npcCode}/sell", a.withCharacter(a.trade("npc.sell")))

	mux.HandleFunc("POST /api/v1/farm/plots/{plotNo}/plant", a.withCharacter(a.command("farm.plant", plotArg, bodyArgs("crop_code"))))
	mux.HandleFunc("POST /api/v1/farm/plots/{plotNo}/harvest", a.withCharacter(a.command("farm.harvest", plotArg)))
	mux.HandleFunc("POST /api/v1/farm/plots/{plotNo}/uproot", a.withCharacter(a.command("farm.uproot", plotArg, bodyArgs("confirmed"))))
	mux.HandleFunc("POST /api/v1/farm/plots/{plotNo}/catch-pest", a.withCharacter(a.command("farm.catch_pest", plotArg)))
	mux.HandleFunc("POST /api/v1/farm/plant-all", a.withCharacter(a.command("farm.plant_all", bodyArgs("crop_code", "confirmed"))))
	mux.HandleFunc("POST /api/v1/farm/harvest-all", a.withCharacter(a.command("farm.harvest_all")))
	mux.HandleFunc("POST /api/v1/farm/uproot-all", a.withCharacter(a.command("farm.uproot_all", bodyArgs("confirmed"))))
	mux.HandleFunc("POST /api/v1/farm/unlock-plot", a.withCharacter(a.command("farm.unlock_plot")))
	mux.HandleFunc("POST /api/v1/inventory/upgrade", a.withCharacter(a.command("inventory.upgrade")))
	mux.HandleFunc("POST /api/v1/pending/crops/{cropCode}/claim", a.withCharacter(a.command("pending.crop.claim", pathArg("cropCode", "crop_code"))))
	mux.HandleFunc("POST /api/v1/pending/crops/claim-all", a.withCharacter(a.command("pending.crop.claim_all")))
	mux.HandleFunc("POST /api/v1/pending/diamonds/{awardId}/claim", a.withCharacter(a.command("pending.diamond.claim", pathArg("awardId", "award_id"))))
	mux.HandleFunc("POST /api/v1/pending/diamonds/claim-all", a.withCharacter(a.command("pending.diamond.claim_all")))
	mux.HandleFunc("POST /api/v1/fashion/equip", a.withCharacter(a.appearanceCommand("fashion.equip", bodyArgs("instance_id"))))
	mux.HandleFunc("POST /api/v1/fashion/unequip", a.withCharacter(a.appearanceCommand("fashion.unequip", bodyArgs("equip_slot"))))
	mux.HandleFunc("POST /api/v1/characters/rename", a.withCharacter(a.rename))
	mux.HandleFunc("GET /api/v1/quests/daily", a.withCharacter(a.quests))
	mux.HandleFunc("POST /api/v1/quests/daily/{questCode}/claim", a.withCharacter(a.command("quests.claim", pathArg("questCode", "quest_code"))))
	a.socialRoutes(mux)
	a.securityRoutes(mux)
}

func (a *API) quests(w http.ResponseWriter, r *http.Request) {
	status, raw, err := a.Tx.Do(r.Context(), http.MethodGet, "/internal/v1/characters/"+character(r).ID+"/quests", nil)
	a.relay(w, status, raw, err)
}

// appearanceCommand runs an outfit change and, once it is committed, shows
// the new look to the character's room (character.appearance.changed).
func (a *API) appearanceCommand(op string, fns ...argFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := map[string]json.RawMessage{}
		if err := decode(r, &body); err != nil {
			httpx.Fail(w, httpx.Validation)
			return
		}
		args := map[string]any{}
		for _, fn := range fns {
			if e := fn(r, body, args); e != nil {
				httpx.Fail(w, e)
				return
			}
		}
		c := character(r)
		path := "/internal/v1/characters/" + c.ID + "/commands/" + op
		status, raw, err := a.Tx.Do(r.Context(), http.MethodPost, path, map[string]any{"request_id": requestID(r, body), "args": args})
		if err == nil && status == http.StatusOK {
			if look, lerr := a.DB.Appearance(r.Context(), c.ID); lerr == nil {
				a.World.SetAppearance(c.ID, look)
			} else {
				a.Log.Error("reload appearance", "err", lerr)
			}
		}
		a.relay(w, status, raw, err)
	}
}

// rename spends a Rename Card (P0-026..029) and, once committed, shows the
// new name to the character's room (character.renamed).
func (a *API) rename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"display_name"`
		RequestID   string `json:"request_id"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	reqID := body.RequestID
	if k := r.Header.Get("Idempotency-Key"); k != "" {
		reqID = k
	}
	c := character(r)
	path := "/internal/v1/characters/" + c.ID + "/commands/character.rename"
	status, raw, err := a.Tx.Do(r.Context(), http.MethodPost, path, map[string]any{"request_id": reqID, "args": map[string]any{"display_name": body.DisplayName}})
	if err == nil && status == http.StatusOK {
		var res struct {
			Result struct {
				NewName string `json:"new_name"`
			} `json:"result"`
		}
		if json.Unmarshal(raw, &res) == nil && res.Result.NewName != "" {
			a.World.SetDisplayName(c.ID, res.Result.NewName)
		}
	}
	a.relay(w, status, raw, err)
}

// ---------------------------------------------------------------------------
// Request plumbing
// ---------------------------------------------------------------------------

func (a *API) clientIP(r *http.Request) string {
	if a.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// checkOrigin is the CSRF defence for cookie-authenticated writes: the
// browser's Origin must be one of the game's origins, and the body must be
// JSON (which a cross-site form cannot send without a CORS preflight).
func (a *API) checkOrigin(r *http.Request) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients (tests, tools) send no Origin; browsers always
		// send it on POST, so a missing Origin is not a cross-site request.
		return r.Header.Get("Sec-Fetch-Site") == "" || r.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	for _, o := range a.AllowedOrigins {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

func decode(r *http.Request, dst any) error {
	body := http.MaxBytesReader(nil, r.Body, 16<<10)
	dec := json.NewDecoder(body)
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

type ctxKey int

const (
	keyPrincipal ctxKey = iota
	keyCharacter
	keyToken
)

func (a *API) principal(r *http.Request) (*authn.Principal, string, *httpx.Error) {
	c, err := r.Cookie(a.CookieName)
	if err != nil {
		return nil, "", httpx.AuthRequired
	}
	p, err := a.Auth.Authenticate(r.Context(), c.Value)
	switch {
	case errors.Is(err, authn.ErrBanned):
		return nil, "", httpx.AccountBanned
	case err != nil && !errors.Is(err, authn.ErrNoSession):
		a.Log.Error("authenticate", "err", err)
		return nil, "", httpx.Internal
	case err != nil:
		return nil, "", httpx.AuthRequired
	}
	return p, c.Value, nil
}

func (a *API) withAccount(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && !a.checkOrigin(r) {
			httpx.Fail(w, httpx.BadOrigin)
			return
		}
		p, token, e := a.principal(r)
		if e != nil {
			httpx.Fail(w, e)
			return
		}
		ctx := context.WithValue(r.Context(), keyPrincipal, p)
		ctx = context.WithValue(ctx, keyToken, token)
		next(w, r.WithContext(ctx))
	}
}

func (a *API) withCharacter(next http.HandlerFunc) http.HandlerFunc {
	return a.withAccount(func(w http.ResponseWriter, r *http.Request) {
		p := r.Context().Value(keyPrincipal).(*authn.Principal)
		c, err := a.DB.CharacterByAccount(r.Context(), p.AccountID)
		if errors.Is(err, gamedb.ErrNoCharacter) {
			httpx.Fail(w, httpx.CharacterRequired)
			return
		}
		if err != nil {
			a.Log.Error("load character", "err", err)
			httpx.Fail(w, httpx.Internal)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), keyCharacter, c)))
	})
}

func character(r *http.Request) *gamedb.Character {
	return r.Context().Value(keyCharacter).(*gamedb.Character)
}

// relay writes a Transaction Service answer through unchanged.
func (a *API) relay(w http.ResponseWriter, status int, body []byte, err error) {
	if err != nil {
		a.Log.Error("transaction service", "err", err)
		httpx.Fail(w, httpx.TxUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// ---------------------------------------------------------------------------
// Auth (M1)
// ---------------------------------------------------------------------------

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if !a.checkOrigin(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	var c credentials
	if err := decode(r, &c); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if ok, wait := a.RegisterLimit.Allow(a.clientIP(r), ""); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	if !authn.ValidUsername(c.Username) {
		httpx.Fail(w, httpx.Validation.With(map[string]any{"field": "username", "rule": "3-32 ký tự A-Z, a-z, 0-9, _"}))
		return
	}
	if !authn.ValidPassword(c.Password) {
		httpx.Fail(w, httpx.Validation.With(map[string]any{"field": "password", "min_bytes": authn.MinPasswordBytes, "max_bytes": authn.MaxPasswordBytes}))
		return
	}
	id, err := a.Auth.Register(r.Context(), c.Username, c.Password)
	if errors.Is(err, authn.ErrUsernameTaken) {
		httpx.Fail(w, httpx.UsernameTaken)
		return
	}
	if err != nil {
		a.Log.Error("register", "err", err)
		httpx.Fail(w, httpx.Internal)
		return
	}
	httpx.OK(w, map[string]any{"account_id": id, "username": c.Username})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.checkOrigin(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	var c credentials
	if err := decode(r, &c); err != nil || c.Username == "" || len(c.Password) > authn.MaxPasswordBytes {
		httpx.Fail(w, httpx.Validation)
		return
	}
	key := strings.ToLower(c.Username)
	if ok, wait := a.Limiter.Allow(a.clientIP(r), key); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	sess, err := a.Auth.Login(r.Context(), c.Username, c.Password)
	var challenge *authn.TOTPChallenge
	switch {
	case errors.As(err, &challenge):
		// Password right, second factor needed (A-10): no session yet.
		a.Limiter.Success(key)
		httpx.OK(w, map[string]any{"totp_required": true, "challenge": challenge.Token})
		return
	case errors.Is(err, authn.ErrInvalidCredentials):
		a.Limiter.Failure(key)
		a.Log.Info("login failed", "username_key", key, "ip", a.clientIP(r))
		httpx.Fail(w, httpx.InvalidCredentials)
		return
	case errors.Is(err, authn.ErrBanned):
		httpx.Fail(w, httpx.AccountBanned.With(map[string]any{"reason": a.Auth.BanReason(r.Context(), c.Username)}))
		return
	case err != nil:
		a.Log.Error("login", "err", err)
		httpx.Fail(w, httpx.Internal)
		return
	}
	a.Limiter.Success(key)
	// P0-006: the older session's live connection ends now; its character
	// waits in the grace period for this new session to resume it.
	a.setSessionCookie(w, sess)
	httpx.OK(w, map[string]any{"account_id": sess.AccountID, "expires_at": sess.ExpiresAt})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if !a.checkOrigin(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	if c, err := r.Cookie(a.CookieName); err == nil {
		if p, err := a.Auth.Authenticate(r.Context(), c.Value); err == nil {
			a.World.KickAccount(p.AccountID, world.CloseLogout, "logout", true)
		}
		if err := a.Auth.Logout(r.Context(), c.Value); err != nil {
			a.Log.Error("logout", "err", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: a.CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode})
	httpx.OK(w, map[string]any{"logged_out": true})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	p, _, e := a.principal(r)
	if e != nil {
		httpx.Fail(w, e)
		return
	}
	out := map[string]any{"account": map[string]any{"id": p.AccountID, "username": p.Username}, "character": nil}
	c, err := a.DB.CharacterByAccount(r.Context(), p.AccountID)
	switch {
	case err == nil:
		out["character"] = map[string]any{"id": c.ID, "display_name": c.DisplayName}
	case !errors.Is(err, gamedb.ErrNoCharacter):
		a.Log.Error("me", "err", err)
		httpx.Fail(w, httpx.Internal)
		return
	}
	httpx.OK(w, out)
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.DB.Pool.Ping(ctx); err != nil {
		httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"service": "auth", "status": "database-unavailable"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"service": "auth", "status": "ok"})
}

// ---------------------------------------------------------------------------
// Character (M1b)
// ---------------------------------------------------------------------------

func (a *API) createCharacter(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(keyPrincipal).(*authn.Principal)
	var body struct {
		DisplayName string `json:"display_name"`
		RequestID   string `json:"request_id"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if k := r.Header.Get("Idempotency-Key"); k != "" {
		body.RequestID = k
	}
	status, raw, err := a.Tx.Do(r.Context(), http.MethodPost, "/internal/v1/characters",
		map[string]string{"account_id": p.AccountID, "display_name": body.DisplayName, "request_id": body.RequestID})
	a.relay(w, status, raw, err)
}

func (a *API) characterMe(w http.ResponseWriter, r *http.Request) {
	c := character(r)
	httpx.OK(w, map[string]any{"id": c.ID, "display_name": c.DisplayName})
}

func (a *API) forwardGet(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, raw, err := a.Tx.Do(r.Context(), http.MethodGet, path, nil)
		a.relay(w, status, raw, err)
	}
}

func (a *API) state(w http.ResponseWriter, r *http.Request) {
	status, raw, err := a.Tx.Do(r.Context(), http.MethodGet, "/internal/v1/characters/"+character(r).ID+"/state", nil)
	a.relay(w, status, raw, err)
}

func (a *API) offers(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("npcCode")
	status, raw, err := a.Tx.Do(r.Context(), http.MethodGet,
		"/internal/v1/characters/"+character(r).ID+"/npc/"+url.PathEscape(code)+"/offers", nil)
	a.relay(w, status, raw, err)
}

// ---------------------------------------------------------------------------
// Asset commands
// ---------------------------------------------------------------------------

// argFn adds arguments taken from the path or the JSON body.
type argFn func(r *http.Request, body map[string]json.RawMessage, args map[string]any) *httpx.Error

func plotArg(r *http.Request, _ map[string]json.RawMessage, args map[string]any) *httpx.Error {
	n, err := strconv.Atoi(r.PathValue("plotNo"))
	if err != nil || n < 1 || n > 1000 {
		return httpx.Validation.With(map[string]any{"field": "plot_no"})
	}
	args["plot_no"] = n
	return nil
}

func pathArg(param, name string) argFn {
	return func(r *http.Request, _ map[string]json.RawMessage, args map[string]any) *httpx.Error {
		v := r.PathValue(param)
		if v == "" || len(v) > 64 {
			return httpx.Validation.With(map[string]any{"field": name})
		}
		args[name] = v
		return nil
	}
}

// bodyArgs copies whitelisted JSON fields; anything else the client sends is
// ignored here and could not reach the Transaction Service anyway.
func bodyArgs(names ...string) argFn {
	return func(_ *http.Request, body map[string]json.RawMessage, args map[string]any) *httpx.Error {
		for _, n := range names {
			if v, ok := body[n]; ok {
				args[n] = v
			}
		}
		return nil
	}
}

func requestID(r *http.Request, body map[string]json.RawMessage) string {
	if k := r.Header.Get("Idempotency-Key"); k != "" {
		return k
	}
	var id string
	if raw, ok := body["request_id"]; ok {
		_ = json.Unmarshal(raw, &id)
	}
	return id
}

func (a *API) command(op string, fns ...argFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := map[string]json.RawMessage{}
		if err := decode(r, &body); err != nil {
			httpx.Fail(w, httpx.Validation)
			return
		}
		args := map[string]any{}
		for _, fn := range fns {
			if e := fn(r, body, args); e != nil {
				httpx.Fail(w, e)
				return
			}
		}
		a.forwardCommand(w, r, op, requestID(r, body), args)
	}
}

func (a *API) forwardCommand(w http.ResponseWriter, r *http.Request, op, reqID string, args map[string]any) {
	path := "/internal/v1/characters/" + character(r).ID + "/commands/" + op
	status, raw, err := a.Tx.Do(r.Context(), http.MethodPost, path, map[string]any{"request_id": reqID, "args": args})
	a.relay(w, status, raw, err)
}

// trade checks that the character stands near the NPC in the world before
// buying or selling (docs/05 §6: the Game Server verifies proximity).
func (a *API) trade(op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		npcCode := r.PathValue("npcCode")
		var body struct {
			OfferID       string `json:"offer_id"`
			Quantity      int64  `json:"quantity"`
			ConfigVersion int64  `json:"config_version"`
			RequestID     string `json:"request_id"`
		}
		if err := decode(r, &body); err != nil {
			httpx.Fail(w, httpx.Validation)
			return
		}
		if e := a.nearNPC(r.Context(), character(r).ID, npcCode); e != nil {
			httpx.Fail(w, e)
			return
		}
		reqID := body.RequestID
		if k := r.Header.Get("Idempotency-Key"); k != "" {
			reqID = k
		}
		a.forwardCommand(w, r, op, reqID, map[string]any{
			"npc_code": npcCode, "offer_id": body.OfferID, "quantity": body.Quantity, "config_version": body.ConfigVersion,
		})
	}
}

func (a *API) nearNPC(ctx context.Context, characterID, npcCode string) *httpx.Error {
	loc, ok := a.World.Locate(ctx, characterID)
	if !ok || !loc.Connected {
		return httpx.NotInWorld
	}
	m := a.Maps.Maps[loc.MapID]
	npc, found := m.NPCs[npcCode]
	if !found {
		return httpx.NPCTooFar.With(map[string]any{"npc_code": npcCode})
	}
	dx, dy := int64(loc.X-npc.X), int64(loc.Y-npc.Y)
	if dx*dx+dy*dy > int64(npc.InteractRadius)*int64(npc.InteractRadius) {
		return httpx.NPCTooFar.With(map[string]any{"npc_code": npcCode})
	}
	return nil
}
