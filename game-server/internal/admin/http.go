package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"example.com/avatar/game-server/internal/authn"
	"example.com/avatar/game-server/internal/httpx"
	"example.com/avatar/game-server/internal/txclient"
	"example.com/avatar/game-server/internal/world"
)

const cookieName = "thanh_admin"
const cookiePath = "/admin/api"

// Handler serves /admin/api/v1. Every route except login needs an Admin
// session, which only exists after the TOTP step (A-11).
type Handler struct {
	Store          *Store
	Tx             *txclient.Client
	World          *world.World
	Limiter        *authn.Limiter
	Log            *slog.Logger
	CookieSecure   bool
	AllowedOrigins []string
	// RefreshRuntime reloads published config and maintenance into the world.
	RefreshRuntime func(ctx context.Context)
}

var (
	errForbidden    = &httpx.Error{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: "Bạn không có quyền thực hiện thao tác này."}
	errSelfApprove  = &httpx.Error{Status: http.StatusForbidden, Code: "SELF_APPROVAL_FORBIDDEN", Message: "Không thể tự duyệt yêu cầu do chính mình tạo."}
	errNotPending   = &httpx.Error{Status: http.StatusConflict, Code: "ALREADY_DECIDED", Message: "Yêu cầu đã được xử lý."}
	errMaintenance  = &httpx.Error{Status: http.StatusConflict, Code: "MAINTENANCE_REQUIRED", Message: "Chỉ phát hành cấu hình trong lúc bảo trì."}
	errInvalidLogin = &httpx.Error{Status: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "Sai thông tin đăng nhập hoặc mã TOTP."}
)

func (h *Handler) Routes(mux *http.ServeMux) {
	p := "/admin/api/v1"
	mux.HandleFunc("POST "+p+"/login", h.login)
	mux.HandleFunc("POST "+p+"/login/totp", h.loginTOTP)
	mux.HandleFunc("POST "+p+"/logout", h.auth(h.logout))
	mux.HandleFunc("GET "+p+"/me", h.auth(h.me))
	mux.HandleFunc("GET "+p+"/catalog", h.auth(h.catalog))
	mux.HandleFunc("GET "+p+"/players", h.auth(h.players))
	mux.HandleFunc("GET "+p+"/players/{characterID}", h.auth(h.player))
	mux.HandleFunc("POST "+p+"/accounts/{accountID}/{action}", h.auth(h.ban))
	mux.HandleFunc("GET "+p+"/adjustments", h.auth(h.adjustments))
	mux.HandleFunc("POST "+p+"/adjustments", h.auth(h.propose))
	mux.HandleFunc("POST "+p+"/adjustments/{id}/{action}", h.auth(h.decide))
	mux.HandleFunc("GET "+p+"/audit", h.auth(h.audit))
	mux.HandleFunc("GET "+p+"/maintenance", h.auth(h.maintenance))
	mux.HandleFunc("POST "+p+"/maintenance", h.auth(h.setMaintenance))
	mux.HandleFunc("GET "+p+"/config/versions", h.auth(h.versions))
	mux.HandleFunc("GET "+p+"/config/versions/{id}", h.auth(h.version))
	mux.HandleFunc("POST "+p+"/config/drafts", h.auth(h.saveDraft))
	mux.HandleFunc("POST "+p+"/config/versions/{id}/publish", h.auth(h.publish))
}

type ctxKey struct{}

func principal(r *http.Request) *Principal { return r.Context().Value(ctxKey{}).(*Principal) }

func (h *Handler) originOK(r *http.Request) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return false
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return r.Header.Get("Sec-Fetch-Site") == "" || r.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	for _, a := range h.AllowedOrigins {
		if strings.EqualFold(a, o) {
			return true
		}
	}
	return false
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && !h.originOK(r) {
			httpx.Fail(w, httpx.BadOrigin)
			return
		}
		c, err := r.Cookie(cookieName)
		if err != nil {
			httpx.Fail(w, httpx.AuthRequired)
			return
		}
		p, err := h.Store.Authenticate(r.Context(), c.Value)
		if err != nil {
			if !errors.Is(err, ErrNoSession) {
				h.Log.Error("admin auth", "err", err)
			}
			httpx.Fail(w, httpx.AuthRequired)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, p)))
	}
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpx.Fail(w, errForbidden)
	case errors.Is(err, ErrSelfApproval):
		httpx.Fail(w, errSelfApprove)
	case errors.Is(err, ErrNotPending):
		httpx.Fail(w, errNotPending)
	case errors.Is(err, ErrMaintenanceOff):
		httpx.Fail(w, errMaintenance)
	case errors.Is(err, ErrNotFound):
		httpx.Fail(w, httpx.NotFound)
	case errors.Is(err, ErrInvalidArgument):
		httpx.Fail(w, httpx.Validation)
	case errors.Is(err, ErrInvalidDraft):
		httpx.Fail(w, httpx.Validation.With(map[string]any{"reason": err.Error()}))
	case errors.Is(err, ErrInvalidLogin):
		httpx.Fail(w, errInvalidLogin)
	default:
		h.Log.Error("admin", "err", err)
		httpx.Fail(w, httpx.Internal)
	}
}

func readJSON(r *http.Request, dst any) error {
	body := http.MaxBytesReader(nil, r.Body, 512<<10)
	if err := json.NewDecoder(body).Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	var b struct{ Login, Password string }
	if err := readJSON(r, &b); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	key := "admin:" + strings.ToLower(b.Login)
	if ok, wait := h.Limiter.Allow(clientIP(r), key); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	ch, err := h.Store.StartLogin(r.Context(), b.Login, b.Password)
	if err != nil {
		h.Limiter.Failure(key)
		h.fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"challenge": ch})
}

func (h *Handler) loginTOTP(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	var b struct{ Challenge, Code string }
	if err := readJSON(r, &b); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if ok, wait := h.Limiter.Allow(clientIP(r), "admin-totp"); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	s, err := h.Store.FinishLogin(r.Context(), b.Challenge, b.Code)
	if err != nil {
		h.fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.Token, Path: cookiePath, Expires: s.ExpiresAt,
		HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteStrictMode})
	httpx.OK(w, map[string]any{"expires_at": s.ExpiresAt})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		_ = h.Store.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: cookiePath, MaxAge: -1, HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteStrictMode})
	httpx.OK(w, map[string]any{"logged_out": true})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) { httpx.OK(w, principal(r)) }

func (h *Handler) relay(w http.ResponseWriter, status int, raw []byte, err error) {
	if err != nil {
		httpx.Fail(w, httpx.TxUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	status, raw, err := h.Tx.Do(r.Context(), http.MethodGet, "/internal/v1/catalog", nil)
	h.relay(w, status, raw, err)
}

func (h *Handler) players(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) > 64 {
		httpx.Fail(w, httpx.Validation)
		return
	}
	list, err := h.Store.SearchPlayers(r.Context(), principal(r), q)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"players": list})
}

func (h *Handler) player(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := r.PathValue("characterID")
	ledger, err := h.Store.Ledger(r.Context(), p, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	_ = h.Store.Audit(r.Context(), p, "player.view", "character", id, nil)
	status, raw, err := h.Tx.Do(r.Context(), http.MethodGet, "/internal/v1/characters/"+id+"/state", nil)
	if err != nil || status != http.StatusOK {
		httpx.Fail(w, httpx.NotFound)
		return
	}
	var st struct {
		Result json.RawMessage `json:"result"`
	}
	_ = json.Unmarshal(raw, &st)
	httpx.OK(w, map[string]any{"state": st.Result, "ledger": ledger})
}

func (h *Handler) ban(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if action != "ban" && action != "unban" {
		httpx.Fail(w, httpx.NotFound)
		return
	}
	var b struct{ Reason string }
	if err := readJSON(r, &b); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	accountID := r.PathValue("accountID")
	if err := h.Store.SetBan(r.Context(), principal(r), accountID, action == "ban", b.Reason); err != nil {
		h.fail(w, err)
		return
	}
	if action == "ban" {
		h.World.KickAccount(accountID, world.CloseLogout, "banned", true)
	}
	httpx.OK(w, map[string]any{"action": action})
}

func (h *Handler) adjustments(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if !p.Can(PermPropose) && !p.Can(PermApprove) {
		h.fail(w, ErrForbidden)
		return
	}
	list, err := h.Store.Adjustments(r.Context(), r.URL.Query().Get("state"))
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"adjustments": list})
}

func (h *Handler) propose(w http.ResponseWriter, r *http.Request) {
	var in Proposal
	if err := readJSON(r, &in); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	a, err := h.Store.Propose(r.Context(), principal(r), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, a)
}

// execute runs an approved adjustment through the Transaction Service with
// the adjustment id as request id, so it can never be applied twice.
func (h *Handler) execute(ctx context.Context, a *Adjustment) (json.RawMessage, bool, error) {
	args := map[string]any{"asset_kind": a.AssetKind, "amount_delta": a.AmountDelta}
	if a.ItemCode != nil {
		args["item_code"] = *a.ItemCode
	}
	status, raw, err := h.Tx.Do(ctx, http.MethodPost, "/internal/v1/characters/"+*a.TargetID+"/commands/admin.adjust",
		map[string]any{"request_id": "adj-" + a.ID, "args": args})
	if err != nil {
		return nil, false, err
	}
	if status >= 500 {
		return nil, false, errors.New("transaction service error " + strconv.Itoa(status))
	}
	var env struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, false, err
	}
	if env.OK {
		return env.Result, true, nil
	}
	return env.Error, false, nil
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if action != "approve" && action != "reject" {
		httpx.Fail(w, httpx.NotFound)
		return
	}
	var b struct{ Note string }
	if err := readJSON(r, &b); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	a, err := h.Store.Decide(r.Context(), principal(r), r.PathValue("id"), action == "approve", b.Note, h.execute)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, a)
}

func (h *Handler) audit(w http.ResponseWriter, r *http.Request) {
	if !principal(r).Can(PermAudit) {
		h.fail(w, ErrForbidden)
		return
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	list, err := h.Store.AuditLog(r.Context(), before, 100)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"entries": list})
}

func (h *Handler) maintenance(w http.ResponseWriter, r *http.Request) {
	m, err := h.Store.Maintenance(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, m)
}

func (h *Handler) setMaintenance(w http.ResponseWriter, r *http.Request) {
	var m Maintenance
	if err := readJSON(r, &m); err != nil || len(m.Message) > 500 {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if err := h.Store.SetMaintenance(r.Context(), principal(r), m); err != nil {
		h.fail(w, err)
		return
	}
	if h.RefreshRuntime != nil {
		h.RefreshRuntime(r.Context())
	}
	httpx.OK(w, m)
}

func (h *Handler) versions(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.Versions(r.Context(), principal(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"versions": list})
}

func (h *Handler) version(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	d, err := h.Store.ReadVersion(r.Context(), principal(r), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, d)
}

func (h *Handler) saveDraft(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ID      int64  `json:"id"`
		Label   string `json:"label"`
		Notes   string `json:"notes"`
		Content *Draft `json:"content"`
	}
	if err := readJSON(r, &b); err != nil || b.Content == nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	id, err := h.Store.SaveDraft(r.Context(), principal(r), b.ID, b.Label, b.Notes, b.Content)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"id": id})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := h.Store.Publish(r.Context(), principal(r), id); err != nil {
		h.fail(w, err)
		return
	}
	if h.RefreshRuntime != nil {
		h.RefreshRuntime(r.Context())
	}
	httpx.OK(w, map[string]any{"published": id})
}
