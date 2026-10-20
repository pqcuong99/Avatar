# PHÁC THẢO MÔ HÌNH DỮ LIỆU (CHƯA PHẢI MIGRATION CHỐT)

**Mục đích:** Chuẩn bị thiết kế PostgreSQL để lập trình Go. Các tên bảng, PK/FK, index, nullable và kiểu dữ liệu sau đây là **[ĐỀ XUẤT KỸ THUẬT]**. Các bất biến nghiệp vụ trích từ P0 đã chốt phải được giữ nguyên.

## 1. Nhóm tài khoản và nhân vật

| Bảng đề xuất | Các trường trọng yếu / invariant |
|---|---|
| `accounts` | `id`, `username_key` UNIQUE, `password_hash`, `email_verified_at NULL`, `status`, `created_at` |
| `account_2fa` | `account_id`, encrypted TOTP secret, status, key rotation; recovery code hash ở bảng khác |
| `auth_sessions` | `id`, `account_id`, session generation, expires/revoked, last seen; chỉ một phiên hợp lệ/account |
| `characters` | `id`, `account_id` UNIQUE, `display_name`, normalized search key với so sánh hoa/thường, `map_id`, `position`, `created_at`; chỉ 1 nhân vật/account |
| `password_reset_requests` | one-time token hash, expiry 10 phút, consume atomically |
| `admin_users`, `admin_roles`, `admin_permissions`, `admin_user_roles` | RBAC, Admin TOTP bắt buộc, audit trail |

**Tên nhân vật**: cần định nghĩa thuật toán Unicode normalization/case folding hỗ trợ *phân biệt dấu*, đếm grapheme (không dùng `len(string)` Go/byte count). Tên `★`/`♥` v.v. cần whitelist chính xác.

## 2. Tiền tệ và sổ cái

| Bảng đề xuất | Nội dung |
|---|---|
| `wallet_balances` | `character_id`, `currency` (XU/DIAMOND), `balance`, `version` / row lock |
| `ledger_transactions` | `tx_id`, `idempotency_key`, `actor_id`, `reason`, `status`, `created_at`, `metadata` |
| `ledger_entries` | `tx_id`, `character_id`, `asset_kind`, `asset_ref`, `delta`, `before`, `after`, các dòng tương quan |
| `asset_adjustment_requests` | đề xuất bởi Admin 1, duyệt bởi Admin 2 khác, loại/số lượng/lý do/trạng thái/audit |
| `pending_diamond_rewards` | nguồn cấp, số lượng chờ, timestamp, claim status; FIFO, không expire |
| `idempotency_records` | scope, account, request id, request hash, result bytes/status, expiration policy cần thống nhất |

**Invariant:** `transaction + entries + wallet/inventory changes + idempotency result` phải commit atomically; không xóa/sửa lịch sử sổ cái; điều chỉnh tạo giao dịch bù.

## 3. Vật phẩm, túi đồ và pending

| Bảng đề xuất | Nội dung |
|---|---|
| `item_definitions` | item type, subtype (fashion/farm/upgrade/seed/worm), bound policy, stack limit, tradability, effective version |
| `inventory_slots` | (`character_id`, `slot_number`) UNIQUE; `item_id`, quantity, instance_id (fashion) |
| `inventory_capacity` | base 10, mỗi upgrade +5, version |
| `pending_items` | loại, lượng, nguồn, earliest timestamp, merge policy, claim state |
| `fashion_equipment` | nhân vật, slot trang bị, item instance; đồng bộ map |

**Invariant quan trọng:** farm/upgrade 999/slot, fashion 1/slot. Túi đầy không làm mất vật phẩm; một lần thu hoạch mà không đủ toàn bộ -> chuyển cả lô vào hàng chờ (P0-076). Pending cùng loại crop gộp, giữ timestamp sớm nhất; ngoại lệ partial chỉ theo P0-083–085. Hạt/nông sản không cho người chơi giao dịch với nhau.

## 4. Nông trại và NPC

| Bảng đề xuất | Nội dung |
|---|---|
| `crop_definitions` | crop ID, seed cost ref, growth minutes, base yield/Admin config, sale price effective version, pest chance |
| `farm_plots` | character, plot ID, unlocked; mặc định 6 plot |
| `farm_plantings` | plot, crop ID, planted_at, matures_at, growth_snapshot, yield_snapshot, harvested_at, state |
| `pest_events` | plant ID UNIQUE, spawn_at, deadline_at, penalty_percent chosen, caught_at, applied; không bốc lại |
| `npc_definitions`, `npc_shop_prices` | type/map position, menu, effective config version; published after maintenance |
| `configuration_releases` | draft/config release version, maintenance publish time, actor approval, checksum |
| `daily_quest_assignments`, `daily_quest_progress`, `daily_quest_claims` | chu kỳ Server, progress, unique claim safeguard |

**Không tạo** bird tables như điều kiện MVP; tài liệu chuẩn bị sau: `birds`, `feed_warehouse`, `bird_feeding_events`, bird catch ledger. Để tránh vỡ schema, có thể thiết kế item sâu ngay MVP, chưa bật chuyển kho/nuôi chim.

## 5. Bản đồ và tương tác xã hội

| Bảng đề xuất | Nội dung |
|---|---|
| `map_definitions`, `map_releases` | Tiled JSON immutable versions, hash và collision geometry |
| `room_sessions` / registry Redis | map ID, room ID, server instance, current population, reconnect reservations TTL 30s |
| `friend_requests`, `friendships` | trạng thái yêu cầu, quan hệ, unique sorted pair; theo GDD cũ |
| `public_chat_messages` | map/room, sender, payload, timestamp, moderation data |
| `private_threads`, `private_messages` | 30 ngày retention/expiry từ gửi thành công, cursor pagination, message IDs |
| `private_chat_blocks` | one-way privacy, block/unblock audit |

**Cần đối chiếu:** các chính sách seen/unread theo viewport, typing, tìm kiếm, block khi thiết kế schema chat; tài liệu `archive/GDD_baseline_2026-10-19.md` có ràng buộc sâu hơn bản P0.

## 6. Audit & vận hành

`admin_audit_log`, `security_events`, `maintenance_jobs`, `backup_restore_drills`, `outbox_events` (nếu dùng outbox). `outbox_events` là đề xuất giúp giao tiếp liên dịch vụ nhất quán, không phải quy tắc gameplay.

## 7. Quy tắc migration và khóa dữ liệu

1. Mọi migration đi kèm review, rollback/forward-fix plan và test trên dữ liệu mô phỏng.
2. Mã idempotency được scope theo account + action; tránh reuse khác payload.
3. Transaction lock ví, inventory và farm theo thứ tự nhất quán để tránh deadlock.
4. Thay đổi config/game giá chỉ công bố sau bảo trì; cây đã gieo snapshot đúng lúc trồng.
5. Cần index theo `account_id`, `character_id`, `request_id`, `matures_at`, `room_id`, cursor chat và `expires_at` tùy truy vấn thật.
6. Thiết kế đầy đủ các policy không cho số dư âm; quy tắc cap Xu vượt trần còn cần đối chiếu trước khi cài đặt.

**Không chạy SQL từ bản mô tả này như một migration hoàn chỉnh.** Cần chuyển thành DDL, invariants, tests và review concurrency trước khi áp dụng vào dữ liệu thật.
