# 11 — Lộ trình triển khai bằng AI, phụ thuộc module và Definition of Done

**Nguyên tắc P0-163:** Từng giai đoạn phải chạy/kiểm thử được trước khi chuyển mốc. Mốc dưới là đề xuất chia việc để AI khác thực hiện, không phải cam kết ngày hoàn thành.

## A. Trạng thái đã biết (21/10/2026)

- [ĐÃ XÁC NHẬN] Máy local hiện chạy được Phaser/React demo, đã trả `/healthz` đúng từ Go Game Server/Transaction Service.
- [CHƯA XÁC NHẬN] Patch Auth v0.2 được tạo nhưng chưa thấy người dùng báo áp dụng thành công.
- [CHƯA XÁC NHẬN] git push nội dung mới lên `pqcuong99/Avatar`.
- [CHƯA LÀM] Character, gameplay network thực, inventory/transaction schema, farm, NPC, xã hội thực tế, admin.

## B. Mốc triển khai

| Milestone | Phạm vi | Bằng chứng DoD bắt buộc | Phụ thuộc |
|---|---|---|---|
| M0 — Setup | Monorepo, Docker local, Go/TS test/CI, docs | chạy local + test + commit clean + no secret | none |
| M1a — Auth | PostgreSQL migrations, registration/login/logout, session generation, 2FA sau auth core | tests, cookie security, migration non-destructive | M0 |
| M1b — Character | 1 char/account, Unicode/grapheme/name unique, initial player data, màn tạo | concurrent create 1 row, seed data đúng, test Vietnamese | M1a, base asset transaction |
| M2a — World | Tiled map, collision shared, WSS auth, rooms + snapshot | 2 browser sync realtime, valid room/map version | M1b |
| M2b — Movement | joystick/WASD/A*, reconcile/reconnect 30s/join friend room | test W-01..08 | M2a |
| M3a — Economy | inventory, wallet, item catalog, transaction ledger, idempotency, pending | T-01..05 and F-15/16 pass | M1b |
| M3b — NPC | UI menu trái/phải, catalog version/publish, buy/sell | T-06..08 pass | M3a,M2a |
| M4 — Farm | plots, plant-all, pests, harvest/pending, NPC loop | F-01..F-16 pass | M3a,M3b |
| M5 — Social | friends, public/PM, blocking/privacy, pagination | S-01..S-10 pass | M2b,M1 |
| M6 — MVP Complete | fashion, daily quests, Admin RBAC/TOTP/dual approval, maintenance | security, reward once, outfit sync, admin audit | M3–M5 |
| M7 — Closed Beta | trial data lifecycle, restrictions, logs, bug fixes | tested reset retains Admin/config; legal runtime host | M6 |
| M8 — Open Beta | HA DB/world, backup/PITR, DDoS, load 300 CCU, no reset | P0-161 gate PASS with evidence | M7 + infra |

**Không đưa** chim, chợ người chơi, nạp tiền thật vào M0–M8 MVP. Chúng có đặc tả sản phẩm để triển khai sau.

## C. Nguyên tắc giao việc cho AI

Mỗi lần giao 1 module (hoặc PR nhỏ), yêu cầu AI:

1. Đọc README + source rules + module spec + GAPs; trước khi thay đổi, tóm tắt yêu cầu đúng và những mục chưa chốt.
2. Kiểm tra tree hiện tại và schema thực, không giả định bản Auth patch đã merge.
3. Đề xuất thiết kế phạm vi ngắn: files changed, migration, HTTP/WS contract, test.
4. Viết code + migration tiến lên không phá dữ liệu; bảo vệ secrets.
5. Viết unit/integration/e2e tests tương ứng trong `docs/10_ACCEPTANCE_TESTS.md`.
6. Chạy build/test/lint, ghi lệnh và output thật. Nếu không chạy được, nói rõ blocker.
7. Xuất patch/commit cùng `DECISIONS P0/TH traceability` và danh sách thay đổi.
8. Không tự thay đổi business rules; đưa câu hỏi lớn vào `docs/12_GAPS_BLOCKERS.md`.

## D. Mốc làm ngay sau handoff

**Task 1** — Merge/tái tạo Auth M1 hiện có trong repo, verify schema migration và local UI login; không xóa Docker volume.

**Task 2** — Create Character với `characters` + Unicode grapheme + 1/account và tạo ví/6 plots/10 slots theo transaction; tránh tự cộng Xu trong Client.

**Task 3** — Tiled 2D real map + Go WS room skeleton, di chuyển authoritative + 30s reconnect.

**Task 4** — Ledger idempotency và inventory trước farm; viết concurrent race tests bắt buộc.

**Task 5** — Farm 5 cây + admin master-data + full purchase→grow→harvest→NPC sale, có pending.

**Task 6** — Social, fashion, quest, Admin; security/ops tests; Closed Beta.

## E. Cách đánh giá tiến độ

`SPEC_DONE` (luật rõ) → `IMPLEMENTED` (code merged) → `INTEGRATED` (chạy với DB/client) → `QA_PASSED` (test report) → `RELEASED` (đã phát hành); không suy từ SPEC_DONE sang IMPLEMENTED.

Dùng CSV `master-data/dev_backlog.csv` làm điểm khởi đầu; milestone chi tiết ở trên là phiên bản mới, luôn xem nguồn quyết định P0 khi có thay đổi.
