# HỢP ĐỒNG API VÀ THÔNG ĐIỆP — BẢN NHÁP

**Trạng thái:** [ĐỀ XUẤT KỸ THUẬT], chưa phải public API chốt. Dùng để chia công việc triển khai, chưa có implementation tương ứng. P0-166 đã chốt **WSS: JSON cho sự kiện chậm, binary cho chuyển động**.

## 1. Nguyên tắc chung

- `request_id` (UUID/ULID) từ một hành động người chơi, duy nhất trong scope đã định; lệnh retry giữ `request_id`.
- `protocol_version`, `server_time`, `type`, `payload`, `error.code` chuẩn hóa. Server không tin bất kỳ client-calculated balance, price, movement collision, yield hay timer.
- Sự kiện thành công chỉ phát sau transaction commit và đạt chính sách độ bền P0-150.
- Đề xuất `error.code`: `UNAUTHENTICATED`, `FORBIDDEN`, `INVALID_INPUT`, `NOT_FOUND`, `INSUFFICIENT_BALANCE`, `INVENTORY_FULL`, `CONFLICT`, `RATE_LIMITED`, `READ_ONLY`, `VERSION_MISMATCH`, `RETRY_LATER`. Mã lỗi vẫn chưa chốt.

## 2. HTTP endpoint đề xuất

| Endpoint | Mục đích / trạng thái |
|---|---|
| `GET /healthz`, `GET /readyz` | **Đã có trong starter Go**; readyz cố ý chưa pass |
| `POST /auth/register` | Đăng ký tự do, tạo account; chưa code |
| `POST /auth/login`, `POST /auth/logout` | Single active session, revoke/reconnect |
| `POST /auth/password-reset/request`, `/confirm` | One-time expiring token 10 phút |
| `GET /characters/me`, `POST /characters` | Nhân vật 1/account, tên hợp lệ |
| `GET /inventory`, `POST /inventory/pending/{id}/claim` | Túi và pending, quy tắc nhận toàn lô/ngoại lệ |
| `GET /farm`, `POST /farm/plots/{id}/plant` | Farm theo Server time + data snapshot |
| `POST /farm/plots/{id}/harvest`, `/catch-pest` | Thu hoạch/bắt sâu thủ công, idempotent |
| `POST /npc/{id}/buy`, `/npc/{id}/sell` | Giao dịch NPC chịu trách nhiệm bởi Transaction Service |
| `GET /wallet`, `GET /ledger` | Thống kê và truy vết tài sản (quyền/trả dữ liệu giới hạn) |
| `GET /friends`, `POST /friends/requests` | Tìm/gửi lời mời/quan hệ bạn bè |
| `POST /admin/asset-adjustments`, `/approve` | Hai người khác nhau, phân quyền RBAC |

**Chưa quyết định** dùng REST endpoint public ở client hay nội bộ Go RPC cho transaction service. Không để transaction service mở trực tiếp vào Internet mà không có auth/service identity.

## 3. WebSocket JSON đề xuất

Client → Server (ngoại trừ move binary):

```json
{"v":1,"type":"chat.public.send","request_id":"<uuid>","payload":{"text":"Xin chào"}}
```

Server → Client:

```json
{"v":1,"type":"chat.public.ack","request_id":"<uuid>","payload":{"message_id":"<id>","sent_at":"<server-utc-time>"}}
```

Tên type demo: `room.join`, `room.leave`, `room.resync`, `friend.join_room`, `chat.public.send`, `chat.private.send`, `npc.interact`, `farm.changed`, `inventory.changed`, `server.maintenance`. Danh sách này **chưa chốt**, chỉ là scaffold đề xuất.

## 4. WebSocket binary: nguyên tắc, chưa chọn schema cụ thể

- Thông điệp movement có message type + schema/protocol version, tick hoặc sequence, action input, server state snapshot.
- [CHỐT] Client tính đường đi; server kiểm tra từng bước, không chấp nhận client set tọa độ tự do.
- [ĐỀ XUẤT] Bộ mã hóa có thể là protobuf hoặc gói custom có phiên bản; cần thử bandwidth/CPU trước khi chốt.
- Chỉ broadcast người cùng phòng, không rò rỉ tin chat riêng sang phòng khác.

## 5. Giao dịch minh họa

`npc.buy` nhận item ID, quantity, request_id, NPC interaction token/version. Transaction service lấy giá có hiệu lực từ server, kiểm tra đủ Xu/kho, lock state, ghi inventory+wallet+ledger+idempotency atomically. Nếu không còn dung lượng: reject hoặc pending theo **đúng luật nghiệp vụ cho loại thao tác đó**; không tự ý cho mọi NPC buy vào pending.

`farm.harvest` trả `harvest_batch_id`, quantity, source/claim location; nếu toàn bộ lô không đủ chỗ túi thì chuyển toàn bộ vào pending, không nửa vào túi nửa pending.

`admin.adjust` ghi proposal trước, execute chỉ sau khi Admin thứ hai phù hợp approve. Người đề xuất không được duyệt chính mình.

## 6. Cần review trước khi code production

Auth scheme/cookie/session CSRF; nguyên tắc message ordering, replay, idempotency expiration; schema binary; ngưỡng rate limiting; cấu trúc lỗi Việt hóa; ledger reconciliation; chat retention, privacy và moderation; version negotiation khi cập nhật bảo trì.
