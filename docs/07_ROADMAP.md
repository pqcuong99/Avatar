# LỘ TRÌNH PHÁT TRIỂN AVATAR

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

- **Có:** M0 *bộ khung repo* (starter), gồm client demo, Go health endpoints, compose local; toolchain còn hỗ trợ (Go 1.27, Node.js 24 LTS, Alpine 3.24), lockfile Client và CI GitHub Actions cơ bản (Go vet/test, build Client, build Docker image). Chưa xác minh chạy end-to-end trên máy cá nhân chủ dự án.
- **Chưa:** lint riêng trong CI (ESLint, `gofmt`, golangci-lint), M1–M8, Game Server online thực, transaction thật, schema migration thật, deploy VPS cũ.
- **Không được đánh đồng:** ‘thiết kế đã chốt’ ≠ ‘đã lập trình’ ≠ ‘đã QA’ ≠ ‘có thể Beta’.

## Backlog đầu tiên có thể đưa lên GitHub Issues

**Nguồn chuẩn duy nhất:** `master-data/dev_backlog.csv` (cột `issue_id, milestone, priority, title, definition_of_done, rule_refs`). Bảng dưới chỉ phản chiếu ID + tiêu đề để dễ đọc; nếu lệch thì **CSV thắng**. Không đặt số `TH-DEV-xxx` ở nơi khác — ticket mới thêm vào CSV với ID kế tiếp. Điều kiện xong (DoD) và `rule_refs` của từng ticket cũng chỉ đọc trong CSV.

Ưu tiên thực hiện: `TH-DEV-001` → `TH-DEV-002` → `TH-DEV-003` (M0), rồi hoàn thiện M1 (`TH-DEV-004`, `005`, `006`, `023`).

| ID | Mốc | Ưu tiên | Tiêu đề (theo CSV) |
|---|---|---|---|
| TH-DEV-001 | M0 | P0 | Tạo Git repo và commit starter + tài liệu |
| TH-DEV-002 | M0 | P0 | Chạy local Go và Client; xác thực toolchain còn hỗ trợ |
| TH-DEV-003 | M0 | P1 | Thiết lập GitHub Actions CI |
| TH-DEV-004 | M1 | P0 | Database migrations cho account/characters/session |
| TH-DEV-005 | M1 | P0 | API đăng ký, login, phiên đơn, password hash |
| TH-DEV-006 | M1 | P0 | Tạo nhân vật/đổi tên Unicode |
| TH-DEV-007 | M2 | P0 | Map Tiled JSON và collision dữ liệu chung |
| TH-DEV-008 | M2 | P0 | WSS, phòng tự phân tải, reconnect 30s |
| TH-DEV-009 | M2 | P0 | Server authoritative movement và pathfinding |
| TH-DEV-010 | M3 | P0 | Ledger, idempotency, transaction service Go |
| TH-DEV-011 | M3 | P0 | Inventory/stack/pending claims |
| TH-DEV-012 | M4 | P0 | Farm 5 cây, snapshot growth & yield |
| TH-DEV-013 | M4 | P0 | Sâu bệnh có sự kiện bền vững |
| TH-DEV-014 | M4 | P0 | NPC shop và giá version theo bảo trì |
| TH-DEV-015 | M5 | P0 | Public chat, friends, private messages |
| TH-DEV-016 | M6 | P1 | Fashion NPC + equipment sync |
| TH-DEV-017 | M6 | P1 | Daily quests cơ bản |
| TH-DEV-018 | M6 | P0 | Admin RBAC/2FA và hai admin approve |
| TH-DEV-019 | M7 | P0 | Đánh giá runtime VPS 2012 R2 hoặc chuyển môi trường thử nghiệm |
| TH-DEV-020 | M7 | P0 | Closed Beta reset, giữ admin/config |
| TH-DEV-021 | M8 | P0 | Offsite PITR + PostgreSQL HA / failover |
| TH-DEV-022 | M8 | P0 | Game Server failover và test 300 CCU |
| TH-DEV-023 | M1 | P0 | Client màn hình đăng ký, đăng nhập, tạo nhân vật |
| TH-DEV-024 | M2 | P0 | Map Tiled mẫu và export collision có version (đầu vào cho TH-DEV-007) |

Lưu ý: `TH-DEV-019` giữ nguyên ràng buộc — **không tuyên bố Closed Beta deploy-ready** khi chưa có runtime hợp lệ.

## Phần sau MVP theo quyết định

- **V1 / bản cập nhật sau:** nuôi chim và kho sâu, chợ người chơi, nạp tiền thật (chỉ khi đủ điều kiện bảo vệ tài chính).
- **Cần xác định thời điểm:** Bang hội, câu cá/mini-game, BXH, danh hiệu và các nội dung 17 hệ thống cũ — không tự chuyển chúng vào MVP.
- Bản roadmap cũ 19/10 là đề xuất **đã bị một phần P0-153–162 thay thế**; lưu tham khảo trong `docs/archive/ROADMAP_DRAFT_superseded_2026-10-19.md`.
