// Package apperr holds the stable error codes of docs/07 §7. Codes are part of
// the API contract; messages are Vietnamese text the client may show as is.
package apperr

import (
	"errors"
	"net/http"
)

// Error is a business rejection: nothing was changed and the outcome is final
// for this request_id (it is stored for idempotent replays).
type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Retryable bool           `json:"retryable"`
	Status    int            `json:"-"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// With returns a copy carrying extra details.
func (e *Error) With(details map[string]any) *Error {
	c := *e
	c.Details = details
	return &c
}

func newErr(status int, code, msg string) *Error {
	return &Error{Code: code, Message: msg, Status: status}
}

var (
	Validation           = newErr(http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Dữ liệu gửi lên không hợp lệ.")
	NotFound             = newErr(http.StatusNotFound, "NOT_FOUND", "Không tìm thấy dữ liệu yêu cầu.")
	CharacterRequired    = newErr(http.StatusConflict, "CHARACTER_REQUIRED", "Tài khoản chưa có nhân vật.")
	CharacterNameInvalid = newErr(http.StatusUnprocessableEntity, "CHARACTER_NAME_INVALID", "Tên nhân vật không hợp lệ.")
	CharacterNameTaken   = newErr(http.StatusConflict, "CHARACTER_NAME_TAKEN", "Tên nhân vật đã có người dùng.")
	CharacterExists      = newErr(http.StatusConflict, "CHARACTER_ALREADY_EXISTS", "Mỗi tài khoản chỉ có một nhân vật.")
	IdempotencyConflict  = newErr(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Mã yêu cầu đã được dùng cho một thao tác khác.")
	InsufficientFunds    = newErr(http.StatusConflict, "INSUFFICIENT_FUNDS", "Không đủ tiền.")
	WalletCapExceeded    = newErr(http.StatusConflict, "WALLET_CAP_EXCEEDED", "Ví đã chạm giới hạn, không thể nhận thêm.")
	InventoryCapacity    = newErr(http.StatusConflict, "INVENTORY_CAPACITY_INSUFFICIENT", "Túi không đủ chỗ.")
	ItemInsufficient     = newErr(http.StatusConflict, "ITEM_INSUFFICIENT", "Không đủ vật phẩm.")
	PlotLocked           = newErr(http.StatusConflict, "PLOT_LOCKED", "Ô đất chưa được mở.")
	PlotOccupied         = newErr(http.StatusConflict, "PLOT_OCCUPIED", "Ô đất đã có cây.")
	PlotEmpty            = newErr(http.StatusConflict, "PLOT_EMPTY", "Ô đất đang trống.")
	PlotNotReady         = newErr(http.StatusConflict, "PLOT_NOT_READY", "Cây chưa đến lúc thu hoạch.")
	PestNotActive        = newErr(http.StatusConflict, "PEST_NOT_ACTIVE", "Không có sâu để bắt ở ô này.")
	ConfirmationRequired = newErr(http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED", "Thao tác này cần xác nhận.")
	PendingWholeBatch    = newErr(http.StatusConflict, "PENDING_WHOLE_BATCH_REQUIRED", "Túi phải đủ chỗ cho toàn bộ dòng hàng chờ.")
	ConfigOutdated       = newErr(http.StatusConflict, "CONFIG_VERSION_OUTDATED", "Bảng giá đã thay đổi, hãy tải lại.")
	NotConfigured        = newErr(http.StatusConflict, "FEATURE_NOT_CONFIGURED", "Chức năng chưa được Admin cấu hình.")
	LimitReached         = newErr(http.StatusConflict, "UPGRADE_LIMIT_REACHED", "Đã đạt giới hạn nâng cấp.")
	AssetWritesDisabled  = &Error{Code: "ASSET_WRITES_DISABLED", Message: "Tạm dừng giao dịch tài sản để bảo toàn dữ liệu.", Status: http.StatusServiceUnavailable, Retryable: true}
	Unauthorized         = newErr(http.StatusUnauthorized, "UNAUTHORIZED", "Yêu cầu nội bộ không hợp lệ.")
	Internal             = &Error{Code: "INTERNAL", Message: "Lỗi máy chủ, hãy thử lại sau.", Status: http.StatusInternalServerError, Retryable: true}
)

// As extracts a business error.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
