# Shared contracts

Dữ liệu dùng chung giữa Client (TypeScript) và Game Server (Go). Cả hai phía có test đọc cùng các file này, nên lệch giao thức hay lệch luật di chuyển sẽ làm CI đỏ.

| Đường dẫn | Nội dung |
|---|---|
| `proto/realtime.proto` | Khung nhị phân WSS cho di chuyển (P0-166): `InputFrame`, `Snapshot`, `EntityState`. Mã hóa tay theo wire format protobuf ở cả hai phía (`game-server/internal/proto`, `client/src/game/net/proto.ts`). Chat/NPC/thông báo dùng phong bì JSON `{v, type, request_id, payload}`. |
| `maps/plaza.json`, `maps/maps.json` | Bản đồ Quảng trường (`map_version` `plaza@1`): ô, vật cản, spawn, NPC, bán kính tương tác. Sinh lại bằng `node tools/mapgen/generate-plaza.mjs` (CI kiểm tra sinh lại ra đúng file). Client gửi `map_version` khi vào; lệch phiên bản → `MAP_VERSION_MISMATCH` (W-08). |
| `maps/tiles_placeholder.png` | Tileset tạm (GAP-13: chưa chốt bộ hình). |
| `fixtures/movement_cases.json` | Golden case di chuyển nguyên số (1/100 px, bước con 1 px, va chạm AABB). Go (`internal/sim`) và TS (`game/sim/movement.test.ts`) phải ra kết quả giống hệt. Cập nhật bằng `UPDATE_GOLDEN=1 go test ./internal/sim/`. |
| `fixtures/proto_frames.json` | Byte mẫu của các khung nhị phân để hai codec kiểm tra chéo. Chỉ cập nhật khi đổi giao thức có chủ đích (và tăng `protocol_version`): `UPDATE_GOLDEN=1 go test ./internal/proto/`. |
