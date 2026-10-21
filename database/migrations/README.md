# Database migrations

Migration PostgreSQL 16 theo thứ tự số. Lệnh `migrate` (image `transaction-service`, dịch vụ `db-migrate` trong `deploy/compose.local.yml`) áp dụng từng file **một lần**, trong transaction, có advisory lock và lưu checksum vào `schema_migrations`: sửa một file đã áp dụng sẽ bị từ chối — hãy thêm file mới.

| File | Nội dung | Quyết định chính |
|---|---|---|
| `0001_auth.sql` | `accounts`, `auth_sessions` (M1, giữ nguyên bản tham chiếu) | P0-010..016 |
| `0002_characters_economy.sql` | nhân vật, cấu hình phiên bản (draft/published/retired, published bất biến), item, túi, ví, `transaction_requests` + `asset_ledger` append-only, KC chờ nhận | P0-017..025, P0-080..090, P0-148..152 |
| `0003_farm_npc.sql` | 5 cây MVP, ô đất, cây trồng (snapshot yield/thời gian), sâu, hàng chờ nông sản, NPC + offer | P0-091..125 |
| `0004_public_chat.sql` | chat công khai theo phòng | P0-126..131 |
| `0005_social.sql` | lời mời/kết bạn (cap 100), chặn, tin nhắn riêng 30 ngày | P0-132..145 |
| `0006_fashion_quests.sql` | thời trang (instance không stack), trang bị, nhiệm vụ ngày | P0-062..070, P0-153..159 |
| `0007_account_security.sql` | email tùy chọn, TOTP người chơi, mã khôi phục, mã email, thử thách đăng nhập | P0-011, P0-146 |
| `0008_admin_operations.sql` | Admin riêng + TOTP bắt buộc, RBAC, điều chỉnh tài sản 2 người, nhật ký append-only, bảo trì | P0-147, P0-160..165 |
| `0009_rename_card.sql` | Thẻ Đổi tên + lịch sử đổi tên | P0-026..029 |

## Seeds

`database/seeds/local/*.sql` là **DEV SAMPLE** (tên NPC "(mẫu)", giá, sản lượng, quà nhiệm vụ…) chỉ được áp dụng khi `THANH_ENV=local`. Đây **không** phải giá trị chốt cho Closed/Open Beta: các giá trị `[CẦN CẤU HÌNH]` phải được Admin nhập và publish qua trang Admin (trong bảo trì). Không có cấu hình thì chức năng trả `FEATURE_NOT_CONFIGURED`, không tự bịa số.

## Quy tắc vận hành

- Không chạy SQL tay trên database thật; mọi thay đổi tài sản đi qua Transaction Service (sổ cái).
- Không `docker compose down -v` (xóa volume database) khi chưa backup và chưa được đồng ý.
- Reset cuối Closed Beta: `deploy/RUNBOOK_CLOSED_BETA_RESET.md`.
