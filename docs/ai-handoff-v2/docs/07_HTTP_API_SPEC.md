# 07 — Đặc tả HTTP API (bản hợp đồng đề xuất v1)

**Chỉ các route Auth M1 bên dưới đã từng được tạo thành source patch; việc áp dụng patch chưa được xác minh.** Mọi route còn lại là **[ĐỀ XUẤT]** cho AI code, không phải API đã hoạt động. Chi tiết OpenAPI machine-readable tại `contracts/openapi-draft.yaml`.

## 1. Tiêu chuẩn chung

- Prefix `/api/v1`, JSON UTF-8, HTTPS production; idempotency header `Idempotency-Key` cho mọi thay đổi tài sản và tạo message.
- Cookie phiên HttpOnly+Secure+SameSite cấu hình phù hợp, backend auth guard; CSRF protection cho POST cookie-auth; Mobile web same-origin ưu tiên.
- Server không chấp nhận `account_id`, `owner_id`, `price` Client tự khai để ủy quyền. Account/character derive từ auth; giá do bản config Server kiểm soát.
- Có `request_id`, `server_time_utc`, `config_version` trong response nơi cần thiết; dùng mã lỗi ổn định, message tiếng Việt để UI hiển thị.

### Response hình thức gợi ý

```json
{
  "ok": false,
  "error": {
    "code": "INVENTORY_CAPACITY_INSUFFICIENT",
    "message": "Túi không đủ chỗ để nhận toàn bộ lô hàng.",
    "details": {"required": 120, "free_capacity": 50},
    "retryable": false
  },
  "request_id": "req-opaque"
}
```

## 2. Auth và nhân vật

| HTTP | Route | Mục đích | Nghiệp vụ |
|---|---|---|---|
| POST | `/auth/register` | Tạo account (M1 patch) | username/pw; email có thể tùy chọn trong bản cuối |
| POST | `/auth/login` | Đăng nhập (M1 patch) | phiên mới đá phiên cũ |
| GET | `/auth/me` | Lấy account phiên hiện tại (M1 patch) | auth-only |
| POST | `/auth/logout` | Revoke phiên (M1 patch) | không revoke phiên login mới khác |
| GET | `/auth/health` | DB health Auth (M1 patch) | local health route |
| POST | `/auth/password/reset-request` | Bắt đầu reset | mã 10 phút, chống account enumeration |
| POST | `/auth/password/reset-confirm` | Xác nhận reset | revoke tất cả sessions |
| POST | `/auth/totp/enroll` | 2FA player tùy chọn | chỉ tạo secret tạm, chưa bật |
| POST | `/auth/totp/enable` | Kích hoạt TOTP | xác minh OTP & backup codes |
| POST | `/auth/totp/disable` | Tắt TOTP | xác thực sở hữu |
| POST | `/characters` | Tạo character duy nhất | tên Unicode unique, cấp data khởi tạo trong 1 transaction |
| GET | `/characters/me` | Lấy profile bản thân | wallet/inventory/tọa độ ở summaries cần giới hạn |
| POST | `/characters/rename` | Dùng Rename Card | chỉ trừ 1 card nếu rename thành công |

## 3. Inventory, Wallet, Pending

| HTTP | Route | Request chính | Response/ràng buộc |
|---|---|---|---|
| GET | `/inventory` | — | slot cap, stacks, item instances |
| POST | `/inventory/upgrade` | request_id | +5 ô nếu đủ chi phí Admin config |
| GET | `/wallets` | — | Xu, KC, caps; không expose mutable fields |
| GET | `/pending/crops` | — | grouped crop, quantity, earliest timestamp FIFO |
| POST | `/pending/crops/{cropCode}/claim` | request_id | toàn dòng trừ ngoại lệ quá max bag capacity |
| POST | `/pending/crops/claim-all` | request_id | FIFO, dừng khi không đủ |
| GET | `/pending/diamonds` | — | per-source entries FIFO |
| POST | `/pending/diamonds/{awardId}/claim` | request_id | không vượt cap; chính sách partial cần đối chiếu |
| POST | `/pending/diamonds/claim-all` | request_id | FIFO, không mất entry chưa nhận |

## 4. Farm và NPC

| HTTP | Route | Request chính | Ràng buộc |
|---|---|---|---|
| GET | `/farm` | — | slots, plantings, pest events, config version |
| POST | `/farm/plots/{plotNo}/plant` | crop_code, request_id | trừ hạt, snapshot yield/time, 1 cây/plot |
| POST | `/farm/plant-all` | crop_code, request_id, confirmed:true | cần confirm UI; duyệt ID tăng |
| POST | `/farm/plots/{plotNo}/harvest` | request_id | mature, một lần; túi/pending toàn lô |
| POST | `/farm/harvest-all` | request_id | plot tăng, không confirm, tiếp tục sau pending |
| POST | `/farm/plots/{plotNo}/uproot` | request_id, confirmed:true | mất toàn cây; confirm |
| POST | `/farm/uproot-all` | request_id, confirmed:true | non/mature đều nhổ; confirm |
| POST | `/farm/plots/{plotNo}/catch-pest` | request_id | trước deadline; worm item (bag-full unresolved) |
| POST | `/farm/unlock-plot` | request_id | +1 plot, ID tăng, config Admin |
| GET | `/npc/{npcCode}/offers` | — | published catalog version |
| POST | `/npc/{npcCode}/buy` | offer_id, qty, request_id | Client không gửi giá authoritative |
| POST | `/npc/{npcCode}/sell` | offer_id, qty, request_id | chỉ item trong túi được phép bán |

**[ĐỀ XUẤT]** Các lệnh có thể được gửi qua WS reliable commands thay vì HTTP nếu thuận tiện; contract và idempotency phải giống. Không cho cả HTTP/WS cùng commit nếu thiếu chung idempotency namespace.

## 5. Social, fashion, quest

| HTTP | Route | Mục đích |
|---|---|---|
| GET | `/players/search?q=` | Tìm character, user chủ động kết bạn |
| GET | `/players/{characterId}/profile` | Profile, context actions |
| POST | `/friends/requests` | Gửi lời mời |
| POST | `/friends/requests/{id}/accept` | Đồng ý |
| POST | `/friends/requests/{id}/reject` | Từ chối |
| GET | `/friends` | Danh sách ≤100 bạn |
| POST | `/friends/{characterId}/join-room` | Đến cùng phòng nếu còn chỗ |
| GET | `/private-chat/{characterId}/messages?before=&limit=` | 50 đầu/20 sau, cursor, không trả tin >30d |
| POST | `/private-chat/{characterId}/messages` | Gửi tin riêng, idempotent, check one-way block |
| POST | `/private-chat/messages/{id}/seen` | Chỉ recipient, sau khi viewport visible |
| POST | `/private-chat/blocks/{characterId}` | Chặn kín đáo |
| DELETE | `/private-chat/blocks/{characterId}` | Bỏ chặn |
| GET | `/private-chat/blocks` | 50 đầu, 20 thêm, tìm có/không dấu/ID |
| POST | `/fashion/equip` | Chọn item owned instance |
| POST | `/fashion/unequip` | Tháo món |
| GET | `/quests/daily` | Nhiệm vụ ngày |
| POST | `/quests/daily/{questId}/claim` | Cấp thưởng once, check ledger |

## 6. Admin

| HTTP | Route | Mục đích |
|---|---|---|
| POST | `/admin/auth/login` | mật khẩu và bước TOTP bắt buộc |
| GET | `/admin/permissions` | quyền hiện tại |
| POST | `/admin/asset-adjustments` | tạo yêu cầu thay đổi tài sản thủ công |
| POST | `/admin/asset-adjustments/{id}/approve` | duyệt bởi Admin khác, role/permission check |
| POST | `/admin/asset-adjustments/{id}/reject` | từ chối, ghi lý do |
| GET | `/admin/audit` | tra cứu audit theo quyền |
| POST | `/admin/config/drafts` | chuẩn bị version config mới |
| POST | `/admin/maintenance/publish` | kiểm soát triển khai qua đợt bảo trì đã báo |
| POST | `/admin/closed-beta/reset` | **TỐI NGUY HIỂM**: không dùng như endpoint production; nên thay bằng công cụ offline vận hành kèm safeguards |

## 7. Mã lỗi đề xuất

| Error code | HTTP | Ý nghĩa |
|---|---:|---|
| `AUTH_REQUIRED` | 401 | phiên hết hạn/chưa login |
| `AUTH_2FA_REQUIRED` | 401 | cần TOTP |
| `FORBIDDEN` | 403 | thiếu quyền, không lộ block đối phương |
| `CHARACTER_NAME_INVALID` | 422 | sai grapheme/char set |
| `CHARACTER_NAME_TAKEN` | 409 | tên unique conflict |
| `CHARACTER_ALREADY_EXISTS` | 409 | 1 account/character |
| `ROOM_FULL` | 409 | không đủ suất room |
| `PLOT_NOT_READY` | 409 | cây chưa lớn |
| `PLOT_OCCUPIED` | 409 | plot đã trồng |
| `INVENTORY_CAPACITY_INSUFFICIENT` | 409 | túi không đủ chỗ |
| `INSUFFICIENT_FUNDS` | 409 | thiếu Xu/KC |
| `PENDING_WHOLE_BATCH_REQUIRED` | 409 | pending phải nhận nguyên dòng |
| `CONFIG_VERSION_OUTDATED` | 409 | giá Client stale |
| `IDEMPOTENCY_CONFLICT` | 409 | request_id trùng nhưng payload khác |
| `ASSET_WRITES_DISABLED` | 503 | không đủ độ bền DB standby |
| `RATE_LIMITED` | 429 | hạn tốc độ |
| `MAINTENANCE` | 503 | đang bảo trì |

Không dùng lỗi `BLOCKED_BY_TARGET` vì sẽ lộ người chặn. Message lỗi cho user cần được kiểm duyệt, không show SQL trace/secret.
