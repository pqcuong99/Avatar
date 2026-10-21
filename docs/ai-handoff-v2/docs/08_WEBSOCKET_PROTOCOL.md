# 08 — WebSocket realtime (bản hợp đồng đề xuất)

## 1. Yêu cầu đã chốt

- WSS in production; JSON cho tin/sự kiện tần suất thấp (chat, NPC, notice); binary cho movement/snapshot cập nhật cao (P0-166).
- Client prediction và pathfinding, Go server authoritative (P0-137/138/168).
- Server tự chia phòng, mỗi phòng cap Admin config; reconnect 30s giữ suất/position (P0-134–136).
- Ping/pong, version negotiation, sequence, replay-safe request_id, reconnect snapshot. Chi tiết wire format dưới đây **[ĐỀ XUẤT]**.

## 2. Session handshake

1. Client login HTTP lấy session cookie; WS upgrade gửi cookie trên same-origin. Go Server verify session, `login_generation`, ban state, character exists.
2. Server gửi `hello` với `protocol_version`, `server_time`, `map_id`, `room_id`, `player_entity_id`, `config_version`, grace.
3. Client gửi `client.ready` và version; Server truyền `room.snapshot` các entity có quyền nhìn thấy.
4. Client bắt đầu input sequence; Server ack movement/position correction.
5. Reconnect trong 30s bằng phiên đã được xác thực và resume token không lộ; lấy diff nếu khả dụng, nếu không full snapshot.

## 3. JSON envelopes (ví dụ, CHƯA triển khai)

```json
{
  "v": 1,
  "type": "chat.public.send",
  "request_id": "cd6a3d86-5bb7-4b08-9679-5ac450b82ff9",
  "client_seq": 45,
  "payload": {"body": "Xin chào!"}
}
```

Server thành công sau validate:

```json
{
  "v": 1,
  "type": "chat.public.delivered",
  "server_seq": 12450,
  "request_id": "cd6a3d86-5bb7-4b08-9679-5ac450b82ff9",
  "payload": {"message_id": "server-generated", "room_id": "room-opaque", "sent_at": "2026-10-21T08:00:00Z"}
}
```

## 4. Event catalog đề xuất

| Direction | Event | Payload cốt lõi | QoS |
|---|---|---|---|
| S→C | `hello` | map, room, config_version, server_time | reliable |
| S→C | `room.snapshot` | visible entities, positions, appearances | reliable |
| S→C | `room.player_joined/left` | entity_id, reason | reliable |
| C→S | `room.join_friend` | friend_character_id, request_id | reliable |
| C→S | `chat.public.send` | body, request_id | reliable, idempotent |
| S→C | `chat.public.delivered` | message, sender, sent_at | reliable |
| C→S | `chat.private.send` | recipient_id, body, request_id | reliable, idempotent |
| S→C | `chat.private.delivered` | message_id, sent_at | reliable |
| S→C | `character.appearance.changed` | character_id, equipment refs, version | reliable |
| S→C | `inventory.changed` | slots, version or delta | after commit |
| S→C | `wallet.changed` | currency, balance, ledger version | after commit |
| S→C | `farm.updated` | plots, timestamps, pest state | after commit |
| S→C | `maintenance.notice` | message, announced_at | reliable |
| S→C | `error` | code, request_id, no secret | reliable |

## 5. Binary movement schema (proposal)

Chưa chốt dùng Protocol Buffers hay custom packing. Khuyến nghị **Protocol Buffers với schema versioned**, được generate từ `.proto` sang Go và TS; không tự hardcode magic offsets. Hai message về mặt logic:

```proto
syntax = "proto3";
package thanh.realtime.v1;
message MoveInput {
  uint64 client_sequence = 1;
  int32 horizontal = 2; // validated: -1, 0, +1
  int32 vertical = 3;   // validated: -1, 0, +1
  uint64 client_tick = 4;
}
message PlayerSnapshot {
  string character_id = 1;
  float x = 2;
  float y = 3;
  uint64 server_tick = 4;
  uint64 ack_client_sequence = 5;
}
```

**[ĐỀ XUẤT]** Control acceleration/speed/rate in Server config. Float representation here is illustrative; deterministic/fixed-point may be preferable depending map scale. Do not infer final tick frequency from sample.

## 6. Movement and anti-cheat

- Client gửi input tại tần suất được giới hạn; Go server step deterministic collision, validate map version and current room.
- Nếu Client dự đoán lệch, Server correction gồm server_tick/authoritative position; Client reconcile, không nhận “teleport chính chủ” từ input.
- Lag/noisy packet không tự ban; log/mark Admin review khi lặp hành vi bất thường.
- `manual_move` hủy `autopath` local ngay; Server nhận input mới và xử lý theo thứ tự sequence.

## 7. Reconnect edge cases

- Disconnect 29s, room full do reserved slot: chủ reconnect vào đúng room; không chiếm suất mới.
- Disconnect 31s: room slot giải phóng, reconnect vào room khác nếu cần; lấy vị trí persistent.
- New login same account: old WS rejected/revoked dù chưa quá 30s; không làm hai nhân vật active.
- Game Server crash: session routing tới instance thay thế; snapshot từ durable state, không cam kết state chỉ trong RAM được cứu.

## 8. Reliability, ordering, limits

- `client_seq` tăng đơn điệu per session, server đọc monotonic, bỏ duplicate stale movement; reorder policy đề xuất cần load test.
- Các sự kiện thay tài sản không phát trước commit DB; nếu `outbox` retry phát lại, Client apply `entity_version` tránh nhân đôi update.
- **[CẦN CẤU HÌNH]** max message size, max chat chars, tick/update Hz, queue depth, compression level, per-room cap, disconnect ping timeout. Cần làm load test 300 CCU.
