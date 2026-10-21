# Avatar — Web Game (Monorepo)

Game xã hội 2D trên trình duyệt: Quảng trường nhiều người chơi, chat, bạn bè, nông trại 5 cây, NPC, thời trang, nhiệm vụ ngày và trang Admin.

**Trạng thái:** MVP lõi đã chạy end-to-end **trên máy local**; chưa có hạ tầng Beta. Bảng nghiệm thu PASS/PARTIAL/BLOCKED và các điểm chờ quyết định: **[docs/IMPLEMENTATION_STATUS.md](docs/IMPLEMENTATION_STATUS.md)**.

## Tài liệu

- **Đặc tả gốc cho lần hiện thực này:** [docs/ai-handoff-v2/MASTER_SPEC_ALL_IN_ONE.md](docs/ai-handoff-v2/MASTER_SPEC_ALL_IN_ONE.md).
- Hồ sơ thiết kế: [docs/00_INDEX.md](docs/00_INDEX.md), [sổ quyết định P0](docs/01_DECISION_REGISTER.md), [kiến trúc](docs/03_ARCHITECTURE.md), [QA](docs/06_ACCEPTANCE_QA.md), [Git/deploy](docs/08_GIT_DEPLOYMENT.md).
- Database: [database/migrations/README.md](database/migrations/README.md) · Giao thức/bản đồ dùng chung: [shared/README.md](shared/README.md).
- Vận hành: [reset cuối Closed Beta](deploy/RUNBOOK_CLOSED_BETA_RESET.md), [cảnh báo VPS 2012 R2](deploy/README_VPS_WIN2012.md), biến môi trường: [.env.example](.env.example).

## Kiến trúc

```text
Trình duyệt (React + Phaser 3 + TypeScript, Vite)
   │  HTTPS JSON (/api, /admin/api)   WSS (/ws: JSON + khung nhị phân di chuyển)
   ▼
game-server (Go)  — xác thực, phòng/di chuyển server-authoritative, chat, bạn bè, Admin
   │  HTTP nội bộ + INTERNAL_API_TOKEN
   ▼
transaction-service (Go) — nơi DUY NHẤT ghi tài sản: ví, túi, nông trại, NPC, thời trang,
   │                        nhiệm vụ, đổi tên; idempotency + sổ cái append-only
   ▼
PostgreSQL 16 (Redis có trong compose nhưng chưa dùng)
```

| Thư mục | Nội dung |
|---|---|
| `client/` | Game (`/`) và trang Admin (`/admin`, tải riêng) |
| `game-server/` | `cmd/game-server`, `cmd/admin` (tạo Admin), `internal/{api,authn,world,realtime,sim,proto,social,admin,…}` |
| `transaction-service/` | `cmd/transaction-service`, `cmd/migrate`, `cmd/beta-reset`, `internal/{econ,farm,inventory,names,httpapi,…}` |
| `database/` | `migrations/` (0001–0009), `seeds/local/` (**DEV SAMPLE**, chỉ khi `THANH_ENV=local`) |
| `shared/` | `proto/realtime.proto`, `maps/plaza.json`, golden fixtures dùng chung Go/TS |
| `tools/` | `mapgen/` (sinh bản đồ), `dev/` (admin-e2e, tua nhanh cây trồng — chỉ local) |
| `deploy/` | `compose.local.yml`, `windows/*.bat`, runbook |

## Chạy trên máy local (Docker Desktop)

```powershell
.\deploy\windows\start-local.bat
```

- Game: http://localhost:5173 — đăng ký tài khoản, tạo nhân vật, vào Quảng trường.
- Admin: http://localhost:5173/admin — tạo tài khoản Admin bằng CLI (xem [IMPLEMENTATION_STATUS §5](docs/IMPLEMENTATION_STATUS.md#5-tài-khoản-thử-nghiệm-local)).
- Health: http://localhost:8081/healthz, http://localhost:8082/healthz.

`db-migrate` tự áp dụng migration và seed mẫu khi khởi động. Dừng bằng `.\deploy\windows\stop-local.bat`. **Không dùng `docker compose down -v`** (xóa database).

Giá, sản lượng, tỷ lệ sâu, quà nhiệm vụ… trong seed local là **giá trị mẫu** để chơi thử, không phải số đã chốt. Ngoài local, chức năng chưa được Admin cấu hình sẽ trả `FEATURE_NOT_CONFIGURED`.

## Kiểm thử

```powershell
.\deploy\windows\run-tests.bat
```

Chạy `gofmt`/`go vet`/`go test -race` cho hai dịch vụ Go (test tích hợp tự tạo database riêng `thanh_test_*`, không đụng dữ liệu dev), rồi test/typecheck/build Client. Kiểm tra Admin end-to-end: `node tools/dev/admin-e2e.mjs`. CI GitHub Actions (`.github/workflows/ci.yml`) chạy cùng các bước trên mỗi push/PR.

Toolchain: Go 1.27, Node.js 24, PostgreSQL 16, image Alpine 3.24.

## Cảnh báo triển khai

- VPS hiện có là **Windows Server 2012 R2** — không hỗ trợ Docker Linux/WSL2 và runtime Go mới. **Không mở Beta công khai trên máy này** (GAP-02).
- Open Beta (không reset) cần HA/PITR, nhiều instance và load test 300 người (O-01..O-03, O-07 đang BLOCKED).
- Đặt `REQUIRE_SYNC_STANDBY=true` khi có standby đồng bộ; `COOKIE_SECURE=true` và `TOTP_ENCRYPTION_KEY` là bắt buộc ngoài local.
- **Không commit `.env`, mật khẩu, token, khóa TOTP, `deploy/local-secrets/` hay dữ liệu người chơi.**
