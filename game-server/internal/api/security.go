package api

import (
	"errors"
	"net/http"
	"strings"

	"example.com/avatar/game-server/internal/authn"
	"example.com/avatar/game-server/internal/httpx"
	"example.com/avatar/game-server/internal/world"
)

var (
	errBadCode       = &httpx.Error{Status: http.StatusUnprocessableEntity, Code: "INVALID_CODE", Message: "Mã không đúng hoặc đã hết hạn."}
	errTOTPOn        = &httpx.Error{Status: http.StatusConflict, Code: "TOTP_ALREADY_ENABLED", Message: "Xác thực 2 lớp đang bật."}
	errTOTPOff       = &httpx.Error{Status: http.StatusConflict, Code: "TOTP_NOT_ENABLED", Message: "Xác thực 2 lớp chưa bật."}
	errNoEnroll      = &httpx.Error{Status: http.StatusConflict, Code: "TOTP_NOT_ENROLLED", Message: "Hãy tạo mã bí mật trước."}
	errMailOff       = &httpx.Error{Status: http.StatusServiceUnavailable, Code: "FEATURE_NOT_CONFIGURED", Message: "Chức năng email chưa được cấu hình. Liên hệ Admin để được hỗ trợ."}
	errBadEmail      = &httpx.Error{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_FAILED", Message: "Email không hợp lệ."}
	errAuth2FAPrompt = &httpx.Error{Status: http.StatusUnauthorized, Code: "AUTH_2FA_REQUIRED", Message: "Nhập mã xác thực 2 lớp."}
)

func (a *API) securityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login/totp", a.loginTOTP)
	mux.HandleFunc("POST /api/v1/auth/password/reset-request", a.resetRequest)
	mux.HandleFunc("POST /api/v1/auth/password/reset-confirm", a.resetConfirm)
	mux.HandleFunc("GET /api/v1/auth/security", a.withAccount(a.securityStatus))
	mux.HandleFunc("POST /api/v1/auth/totp/enroll", a.withAccount(a.totpEnroll))
	mux.HandleFunc("POST /api/v1/auth/totp/enable", a.withAccount(a.totpEnable))
	mux.HandleFunc("POST /api/v1/auth/totp/disable", a.withAccount(a.totpDisable))
	mux.HandleFunc("POST /api/v1/auth/email", a.withAccount(a.setEmail))
	mux.HandleFunc("POST /api/v1/auth/email/verify", a.withAccount(a.verifyEmail))
}

func (a *API) securityFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authn.ErrBadCode):
		httpx.Fail(w, errBadCode)
	case errors.Is(err, authn.ErrTOTPAlreadyOn):
		httpx.Fail(w, errTOTPOn)
	case errors.Is(err, authn.ErrTOTPNotOn):
		httpx.Fail(w, errTOTPOff)
	case errors.Is(err, authn.ErrNoEnrollment):
		httpx.Fail(w, errNoEnroll)
	case errors.Is(err, authn.ErrMailNotConfigured):
		httpx.Fail(w, errMailOff)
	case errors.Is(err, authn.ErrBadEmail):
		httpx.Fail(w, errBadEmail)
	case errors.Is(err, authn.ErrInvalidCredentials):
		httpx.Fail(w, httpx.InvalidCredentials)
	default:
		a.Log.Error("security", "err", err)
		httpx.Fail(w, httpx.Internal)
	}
}

func (a *API) setSessionCookie(w http.ResponseWriter, s *authn.Session) {
	a.World.KickAccount(s.AccountID, world.CloseSessionReplaced, "session_replaced", false)
	http.SetCookie(w, &http.Cookie{
		Name: a.CookieName, Value: s.Token, Path: "/", Expires: s.ExpiresAt,
		HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *API) loginTOTP(w http.ResponseWriter, r *http.Request) {
	if !a.checkOrigin(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	var body struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if err := decode(r, &body); err != nil || body.Challenge == "" || len(body.Code) > 20 {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if ok, wait := a.Limiter.Allow(a.clientIP(r), "totp"); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	sess, err := a.Auth.CompleteTOTP(r.Context(), a.Sec, body.Challenge, body.Code)
	if err != nil {
		a.securityFail(w, err)
		return
	}
	a.setSessionCookie(w, sess)
	httpx.OK(w, map[string]any{"account_id": sess.AccountID, "expires_at": sess.ExpiresAt})
}

func (a *API) securityStatus(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(keyPrincipal).(*authn.Principal)
	st, err := a.Auth.Status(r.Context(), a.Sec, p.AccountID)
	if err != nil {
		a.securityFail(w, err)
		return
	}
	httpx.OK(w, st)
}

func (a *API) totpEnroll(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(keyPrincipal).(*authn.Principal)
	e, err := a.Auth.EnrollTOTP(r.Context(), a.Sec, p.AccountID, p.Username)
	if err != nil {
		a.securityFail(w, err)
		return
	}
	httpx.OK(w, e)
}

func (a *API) totpEnable(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(keyPrincipal).(*authn.Principal)
	var body struct {
		Code string `json:"code"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	codes, err := a.Auth.EnableTOTP(r.Context(), a.Sec, p.AccountID, body.Code)
	if err != nil {
		a.securityFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"recovery_codes": codes})
}

func (a *API) totpDisable(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(keyPrincipal).(*authn.Principal)
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if ok, wait := a.Limiter.Allow(a.clientIP(r), "disable:"+p.AccountID); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	if err := a.Auth.DisableTOTP(r.Context(), a.Sec, p.AccountID, body.Password, body.Code); err != nil {
		a.Limiter.Failure("disable:" + p.AccountID)
		a.securityFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"totp_enabled": false})
}

func (a *API) setEmail(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(keyPrincipal).(*authn.Principal)
	var body struct {
		Email string `json:"email"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if ok, wait := a.RegisterLimit.Allow(a.clientIP(r), "email:"+p.AccountID); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	if err := a.Auth.SetEmail(r.Context(), a.Sec, p.AccountID, body.Email); err != nil {
		a.securityFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"sent": true})
}

func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(keyPrincipal).(*authn.Principal)
	var body struct {
		Code string `json:"code"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if err := a.Auth.VerifyEmail(r.Context(), p.AccountID, body.Code); err != nil {
		a.securityFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"email_verified": true})
}

// resetRequest answers the same whether or not the account exists or has a
// verified email, so it cannot be used to discover accounts.
func (a *API) resetRequest(w http.ResponseWriter, r *http.Request) {
	if !a.checkOrigin(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if err := decode(r, &body); err != nil || !authn.ValidUsername(body.Username) {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if ok, wait := a.RegisterLimit.Allow(a.clientIP(r), "reset:"+strings.ToLower(body.Username)); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	if err := a.Auth.RequestReset(r.Context(), a.Sec, body.Username); err != nil {
		if errors.Is(err, authn.ErrMailNotConfigured) {
			httpx.Fail(w, errMailOff)
			return
		}
		a.Log.Error("reset request", "err", err)
	}
	httpx.OK(w, map[string]any{"message": "Nếu tài khoản có email đã xác minh, mã đặt lại mật khẩu đã được gửi (hiệu lực 10 phút)."})
}

func (a *API) resetConfirm(w http.ResponseWriter, r *http.Request) {
	if !a.checkOrigin(r) {
		httpx.Fail(w, httpx.BadOrigin)
		return
	}
	var body struct {
		Username    string `json:"username"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := decode(r, &body); err != nil || !authn.ValidUsername(body.Username) {
		httpx.Fail(w, httpx.Validation)
		return
	}
	if !authn.ValidPassword(body.NewPassword) {
		httpx.Fail(w, httpx.Validation.With(map[string]any{"field": "new_password", "min_bytes": authn.MinPasswordBytes, "max_bytes": authn.MaxPasswordBytes}))
		return
	}
	if ok, wait := a.Limiter.Allow(a.clientIP(r), "reset:"+strings.ToLower(body.Username)); !ok {
		httpx.Fail(w, httpx.RateLimited.With(map[string]any{"retry_after_seconds": int(wait.Seconds()) + 1}))
		return
	}
	accountID, err := a.Auth.ConfirmReset(r.Context(), body.Username, body.Code, body.NewPassword)
	if err != nil {
		a.Limiter.Failure("reset:" + strings.ToLower(body.Username))
		a.securityFail(w, err)
		return
	}
	// Every session is revoked (P0-015); live connections end now.
	a.World.KickAccount(accountID, world.CloseSessionReplaced, "password_reset", true)
	httpx.OK(w, map[string]any{"password_changed": true})
}
