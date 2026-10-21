# PROMPT TASK — QA, security, deploy

Sử dụng `docs/09_SECURITY_OPERATIONS.md`, `docs/10_ACCEPTANCE_TESTS.md`, `docs/11_ROADMAP_AND_BACKLOG.md`, `docs/12_GAPS_BLOCKERS.md`.

1. Lập traceability matrix P0 -> API/DB/test và report tất cả case PASS/FAIL/BLOCKED bằng chứng chạy thật.
2. Kiểm thử security login/session/2FA/CSRF/role audit, brute-force and websocket payload validation.
3. Kiểm thử tích hợp item/wallet ledger under concurrent retries, farm offline restart, backup+PITR restore, room failover và DB standby fencing.
4. Thiết lập CI không lộ secret; backup offsite; runbook deploy maintenance/rollback, reset Closed Beta giữ Admin.
5. Kiểm thử tải 300 CCU active có move/chat/farm/NPC; đo latency, CPU, RAM, DB locks, packet rate.
6. Không claim Open Beta-ready trên 1 VPS Windows Server 2012 R2; viết báo cáo chưa đạt HA/OS support và phương án nâng cấp.

Đầu ra: test plan, scripts, evidence/log redacted, report gate go/no-go, rollback checklist; không tự triển khai internet/public nếu chủ dự án chưa phê duyệt.
