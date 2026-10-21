# 09 — Bảo mật, Admin, DevOps, Backup, Closed/Open Beta

## 1. Mức bảo đảm và giới hạn môi trường

**[CHỐT]** Tiền/vật phẩm phải giữ tính toàn vẹn; Server là nguồn sự thật; DB failover an toàn; daily backup + PITR; Admin RBAC và TOTP, dual approval.  
**[THỰC TẾ]** Một VPS Windows Server 2012 R2 không thể đạt HA nhiều host; không hỗ trợ WSL2 và runtime Go mới. **Không coi hệ thống hiện tại là đủ an toàn để mở Open Beta hoặc nhận thanh toán.**

## 2. Authentication/Authorization

- Player username/password, verified recovery email optional; không tiết lộ tài khoản có tồn tại trong reset flow.
- Password hash bằng thuật toán được bảo trì (patch M1 dùng bcrypt; production cần review cost), không log password; session token ngẫu nhiên, lưu hash trên DB, HttpOnly+Secure+SameSite, CSRF defense, session expiration/revoke.
- Một account một phiên: `login_generation` + kiểm tra server; đặt lại password vô hiệu mọi session.
- TOTP Admin bắt buộc; player tùy chọn; recovery codes chỉ hiển thị một lần và lưu hash; secret TOTP encrypted at rest và giới hạn quyền truy cập.
- Gateway throttling + account/IP/device signal + CAPTCHA khi nghi ngờ; không khóa vĩnh viễn do brute force. Cần Redis/distributed limiter khi multi replica; local memory limiter M1 không đủ.
- RBAC từng permission, Default Deny; kiểm tra ở API và transaction service; tránh IDOR bằng owner checks; CSRF/cookie origin controls.

## 3. Data consistency và chống gian lận

- Tiền/vật phẩm được cấp/trừ trong PostgreSQL transaction; ledger append-only, idempotency unique, outbox after commit.
- Farm timestamps server and persisted pests; no tampering qua sửa giờ máy, request forged yield/price/speed.
- Movement input validated speed, collision, room; nghi vấn lặp lại được log/Admin review, **không auto ban chỉ do input sai** (P0-138).
- Chat rate-limit, input length bounds, escape HTML/XSS, generic reply when blocked, TTL từng PM = 30 ngày.
- Admin asset adjustment cần 2 nhân viên khác nhau, RBAC/TOTP, reason, audit, no self-approve; không làm trong console update SQL ad-hoc.

## 4. Lớp chống DDoS/Bot

1. DDoS/CDN/WAF hỗ trợ cả HTTP và WebSocket, bảo vệ origin IP.
2. Gateway limit connection/requests cho IP/account/token, origin restriction, TLS.
3. Game Server action-rate limit (move/chat/NPC/farm), payload schema verification.
4. Metrics/alerts khi tăng lỗi, connection churn, latency, failed login, DB failover, transaction rejection.

Các ngưỡng cụ thể vẫn `[CẦN CẤU HÌNH]`, không tự hardcode theo suy đoán.

## 5. Bảo trì và publish cấu hình

- Soạn config/map/NPC/news theo version dự thảo; kiểm thử staging; thông báo lịch; ngừng accept giao dịch mới; drain/reconcile inflight; chụp backup/restore point; deploy; smoke test; mở lại khi an toàn.
- Không thay giá NPC và bảng tin đang mở trực tiếp lúc server hoạt động.
- Khi rollback, không được phục hồi Snapshot cũ làm mất giao dịch đã commit; dùng migrate-backwards/forward hotfix hoặc PITR cùng reconciliation theo runbook.
- Audit ai duyệt bản publish, config hash/version, Git commit và thời gian hiệu lực.

## 6. Backup/PITR và failover

- Daily DB backup tự động + WAL archiving; offsite storage độc lập, mã hóa, access least privilege.
- Kiểm tra restore định kỳ; test backup không được coi thành công chỉ vì file tồn tại.
- HA PostgreSQL primary/eligible standby, tự failover chỉ khi không split-brain; standby mất khả năng đồng bộ an toàn => freeze asset-writes (P0-150).
- World Server có room allocator/circuit breaker; lỗi một instance không làm chết toàn hệ thống, reconnect đến replacement dựa trên durable state.
- **[CẦN CẤU HÌNH]** RPO/RTO, retention offsite backup, alert SLA, release windows chưa chốt.

## 7. Reset dữ liệu Closed Beta

**Luồng an toàn [ĐỀ XUẤT]**:

1. Xác nhận rõ môi trường `CLOSED_BETA`; tuyệt đối reject trên `OPEN_BETA`/`PRODUCTION`.
2. Hai bước xác thực người vận hành + xác nhận nhập lại tên môi trường/commit; phương thức approver riêng nếu có quyền phải theo RBAC.
3. Công bố thông báo reset; đóng đăng nhập và giao dịch, revoke player sessions.
4. Snapshot toàn DB và lưu offsite; ghi checksum, thời điểm, operator, job ID.
5. Chạy dry-run đếm rows theo danh sách bảng sở hữu người chơi, xác thực sample FK.
6. Xóa toàn bộ player-owned (`accounts`/`characters`/social/ledger/inventory/farm/quest), giữ **admin_users, roles, permissions, game_config_versions, NPC/catalog**.
7. Verify 0 player accounts, 0 dangling rows; Admin login/TOTP còn hoạt động, catalog giữ nguyên.
8. Ghi audit, mở đăng ký mới sau reset.

Không cung cấp một nút reset có thể click nhầm trên hệ thống Open Beta; đừng chạy `docker compose down -v` hoặc drop toàn DB.

## 8. Closed Beta ở VPS cũ

- Hệ điều hành Windows Server 2012 R2 build 9600 hết hỗ trợ thông thường, không có WSL2. Không thể triển khai Linux Compose trực tiếp như kế hoạch ban đầu.
- Nếu buộc dùng VPS này cho Closed Beta giới hạn: đánh giá thời gian đầu Go runtime/build binary compatible và dependency; cân nhắc build tại môi trường khác/host trung gian; không downgrade phụ thuộc bảo mật về bản không bảo trì; không phơi SSH/RDP/DB trực tiếp ra Internet.
- Khuyến nghị [ĐỀ XUẤT]: dùng môi trường Linux mới cho Server thật, giữ Windows Server 2012 R2 chỉ như mục tiêu thử nghiệm được hạn chế quyền, không tài sản có giá trị.
- Docker Desktop chỉ cài trên máy Windows cá nhân để phát triển; Docker Engine + Linux Compose hợp lý khi có Ubuntu VPS.

## 9. CI/CD và Git

- Repo duy nhất `pqcuong99/Avatar`. Branch protection khi có workflow; PR có test reports. Không commit `.env`, DB dump, secrets, `node_modules`, private keys.
- CI: Go fmt/vet/test/race, TypeScript typecheck/lint/build, OpenAPI validate, SQL lint/schema migration integration, unit/gameplay fixtures, dependency scanning.
- Triển khai bảo trì: pinned version/tag, config/checksum, backup, migrate, deploy, smoke, rollback/reconcile.
- File `.bat` tự động cho developer là tiện ích; **không đồng nghĩa VPS cũ có thể chạy Docker Linux**. Kịch bản deploy production phải khác local.

## 10. Bảo vệ dữ liệu cá nhân và retention

- Thông tin người dùng và chat chỉ lưu các trường cần thiết; không log token/mật khẩu/mã khôi phục; audit nghiêm.
- Chat riêng TTL 30 ngày tính trên mỗi message và lọc hết hạn ngay lúc đọc; logs/security retention cần chính sách độc lập.
- Các yêu cầu hỗ trợ khi không có email phải lưu bằng chứng với quyền giới hạn, purge theo chính sách chưa chốt.
