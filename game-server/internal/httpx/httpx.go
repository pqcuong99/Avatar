// Package httpx holds the JSON envelope and the error codes the Game Server
// itself produces (docs/07 §1, §7). Errors from the Transaction Service are
// passed through unchanged.
package httpx

import (
	"encoding/json"
	"net/http"
	"time"
)

type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Retryable bool           `json:"retryable"`
	Status    int            `json:"-"`
}

func (e *Error) Error() string { return e.Code }

func (e *Error) With(details map[string]any) *Error {
	c := *e
	c.Details = details
	return &c
}

func newErr(status int, code, msg string, retryable bool) *Error {
	return &Error{Status: status, Code: code, Message: msg, Retryable: retryable}
}

var (
	AuthRequired        = newErr(http.StatusUnauthorized, "AUTH_REQUIRED", "Phiên đăng nhập đã hết hạn, hãy đăng nhập lại.", false)
	InvalidCredentials  = newErr(http.StatusUnauthorized, "INVALID_CREDENTIALS", "Sai tên đăng nhập hoặc mật khẩu.", false)
	AccountBanned       = newErr(http.StatusForbidden, "ACCOUNT_BANNED", "Tài khoản đang bị khóa.", false)
	Forbidden           = newErr(http.StatusForbidden, "FORBIDDEN", "Bạn không có quyền thực hiện thao tác này.", false)
	UsernameTaken       = newErr(http.StatusConflict, "USERNAME_TAKEN", "Tên đăng nhập đã được dùng.", false)
	Validation          = newErr(http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Dữ liệu gửi lên không hợp lệ.", false)
	RateLimited         = newErr(http.StatusTooManyRequests, "RATE_LIMITED", "Thao tác quá nhanh, hãy thử lại sau.", true)
	CharacterRequired   = newErr(http.StatusConflict, "CHARACTER_REQUIRED", "Tài khoản chưa có nhân vật.", false)
	NotInWorld          = newErr(http.StatusConflict, "NOT_IN_WORLD", "Hãy vào bản đồ trước khi giao dịch.", false)
	NPCTooFar           = newErr(http.StatusConflict, "NPC_TOO_FAR", "Hãy lại gần NPC để giao dịch.", false)
	NotFound            = newErr(http.StatusNotFound, "NOT_FOUND", "Không tìm thấy.", false)
	BadOrigin           = newErr(http.StatusForbidden, "BAD_ORIGIN", "Yêu cầu không đến từ trang game.", false)
	TxUnavailable       = newErr(http.StatusServiceUnavailable, "TRANSACTION_SERVICE_UNAVAILABLE", "Máy chủ giao dịch đang bận; thử lại với cùng thao tác.", true)
	Internal            = newErr(http.StatusInternalServerError, "INTERNAL", "Lỗi máy chủ, hãy thử lại sau.", true)
	MaintenanceRequired = newErr(http.StatusServiceUnavailable, "MAINTENANCE", "Máy chủ đang bảo trì.", true)
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, result any) {
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result, "server_time_utc": time.Now().UTC().Format(time.RFC3339Nano)})
}

func Fail(w http.ResponseWriter, e *Error) {
	WriteJSON(w, e.Status, map[string]any{"ok": false, "error": e})
}
