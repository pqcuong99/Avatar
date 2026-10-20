# LỘ TRÌNH PHÁT TRIỂN PROJECT THANH

**Nguyên tắc:** P0-163 chốt xây dựng **từng giai đoạn, tích hợp và kiểm thử liên tục**. Không hứa lịch ngày/tuần khi chưa biết số người code, giờ làm và cấu hình máy. Những “M0–M8” dưới đây là **[ĐỀ XUẤT PHÂN RÃ]** để triển khai đúng phạm vi MVP đã chốt.

## Tóm tắt đường găng

`M0 repo + toolchain` → `M1 account + DB` → `M2 map/WSS + rooms` → `M3 inventory/wallet/ledger` → `M4 farm/NPC` → `M5 chat/friends` → `M6 fashion/quests/admin` → `M7 Closed Beta` → `M8 Open Beta gate`.

Một số đầu việc backend, security, QA thực hiện **song song** với milestone chứ không chờ cuối.

## Chi tiết milestones

| Mốc | Hạng mục | Sản phẩm có thể chạy/kiểm thử | Điều kiện xong (DoD) |
|---|---|---|---|
| M0 | Monorepo, Git, script local, lint/test, compose | Client demo và Go health services chạy local | Git history, CI cơ bản, no-secrets, hướng dẫn tái lập |
| M1 | PostgreSQL schema/migrations, đăng ký, đăng nhập, 1 nhân vật, session/recovery | Đăng nhập và tạo nhân vật, có DB persistent | Unique name Unicode, password hash, auth tests, phiên đơn |
| M2 | Tiled map thật, WSS JSON/binary, phòng, movement server authority, reconnect | 2+ người trong phòng nhìn thấy nhau; demo chuyển phòng | Pathfinding Client, collision Server, reconnect 30s, movement cheat rejects |
| M3 | Inventory, currency, transaction service Go, ledger/idempotency | NPC buy simulation, wallet/inventory consistency | Concurrent retry không nhân đôi, full bag/pending, rollback test |
| M4 | Farm 5 cây, crop snapshot, pest, NPC mua/bán, config maintenance | Full loop gieo→bắt sâu→thu→bán→mua | Offline timing, FIFO pending, Thu hoạch tất cả test |
| M5 | Chat public/private, friends/profile, privacy/block | Người chơi trò chuyện/kết bạn/PM | Retention 30d, pagination 50+20, block privacy/reconnect |
| M6 | Fashion base, quest day, RBAC/TOTP Admin, 2-admin asset adjust | MVP đầy đủ tính năng được chọn | Dress sync; rewards idempotent; 2-person flow; admin audit |
| M7 | Closed Beta giới hạn thực tế, reset workflow, bug fixing | Người chơi thử đăng ký tự do và trải nghiệm | Runtime an toàn đã xác minh, QA baseline, backup rồi reset thử |
| M8 | Hạ tầng HA/PITR/failover, bảo vệ DDoS, perf 300, Open Beta | Go-live candidate trên hệ thống phù hợp | **P0-161** gate, No P0 bugs, 300-online proof, no-reset ready |

### Mốc đang ở đâu

- **Có:** M0 *bộ khung repo* (starter), gồm client demo, Go health endpoints, compose local. Chưa xác minh chạy end-to-end trên máy cá nhân chủ dự án.
- **Chưa:** M0 CI đầy đủ, M1–M8, Game Server online thực, transaction thật, schema migration thật, deploy VPS cũ.
- **Không được đánh đồng:** ‘thiết kế đã chốt’ ≠ ‘đã lập trình’ ≠ ‘đã QA’ ≠ ‘có thể Beta’.

## Backlog đầu tiên có thể đưa lên GitHub Issues

Ưu tiên thực hiện: `TH-DEV-001` → `...002` → `...003` rồi hoàn thiện M1. Danh sách đầy đủ xuất tại `master-data/dev_backlog.csv`.

- TH-DEV-001: Clone/copy starter vào Git repo trống, thiết lập `.gitignore`, `README`, pipeline build/test.
- TH-DEV-002: Quyết định phiên bản toolchain **còn được bảo trì** cho Go, Node.js và image Docker ở môi trường local.
- TH-DEV-003: Chuẩn hóa migration framework + schema account/character/session.
- TH-DEV-004: API account registration/login/session single-device + password hashing.
- TH-DEV-005: Client màn hình đăng ký/đăng nhập/tạo nhân vật.
- TH-DEV-006: Tiled map sample + collision export version.
- TH-DEV-007: WebSocket handshake auth, room registry, movement authoritative.
- TH-DEV-008: Inventory/wallet/ledger/idempotency contract và tests.
- TH-DEV-009: Implement farm MVP với pest event deterministic persisted.
- TH-DEV-010: Chat/friends, UI fashion, daily quests và Admin theo bảng phụ thuộc.
- TH-DEV-011: Đánh giá ràng buộc VPS cũ; **không tuyên bố Closed Beta deploy-ready** khi chưa có runtime hợp lệ.

## Phần sau MVP theo quyết định

- **V1 / bản cập nhật sau:** nuôi chim và kho sâu, chợ người chơi, nạp tiền thật (chỉ khi đủ điều kiện bảo vệ tài chính).
- **Cần xác định thời điểm:** Bang hội, câu cá/mini-game, BXH, danh hiệu và các nội dung 17 hệ thống cũ — không tự chuyển chúng vào MVP.
- Bản roadmap cũ 19/10 là đề xuất **đã bị một phần P0-153–162 thay thế**; lưu tham khảo trong `docs/archive/ROADMAP_DRAFT_superseded_2026-10-19.md`.
