# PROMPT TASK — M1 Auth + Character

1. Đọc `reference/M1_AUTH_PROTOTYPE_STATUS.md`, `reference/0001_auth_ACTUAL.sql`, kiểm tra code Go/Auth và React thật.
2. Hoàn thiện Auth: register/login/logout/me, single-session, bảo vệ CSRF/rate limits, no secret logs; recovery email tùy chọn, reset TTL10m và revoke; TOTP player tùy chọn theo phạm vi phát hành.
3. Thêm tạo một Character/account, tên 3–16 grapheme hợp lệ có dấu Việt, so sánh casefold nhưng phân biệt dấu. Không có xóa/recreate API.
4. Tạo ví 5.000 Xu/0 Kim Cương, 10 ô túi và 6 ô đất khi tạo character đúng transaction; chuẩn bị ledger cho starter asset, không ghi từ React.
5. React có loading/validation/errors/duplicate name/create successful; update Session state.
6. Tests: race create Character x2, Unicode composed/decomposed, login A then B invalidates A, password reset expires.

Cho biết rõ chức năng nào đã làm, chức năng nào đề xuất theo thứ tự thực hiện, không nhận “M1 hoàn thành” nếu recovery/2FA thiết kế MVP chưa có.
