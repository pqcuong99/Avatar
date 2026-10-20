# Project Thanh — Avatar Web Game (Monorepo)

**Bản bàn giao:** 20/10/2026. **Repository:** `https://github.com/pqcuong99/Avatar.git` (người dùng xác nhận repo rỗng tại thời điểm bàn giao).  
**Trạng thái:** 🛠️ Starter v0.1 + hồ sơ thiết kế; **không phải game MVP đã hoàn tất**.

## Đọc tài liệu trước khi code

- **[Hướng dẫn bộ hồ sơ](docs/00_INDEX.md)** — bắt đầu tại đây.
- **[Đặc tả MVP](docs/02_PRODUCT_SPEC_MVP.md)** — chức năng & giới hạn.
- **[Sổ quyết định P0-001–P0-183](docs/01_DECISION_REGISTER.md)** — luật đã chốt và quyết định thay thế.
- **[Kiến trúc](docs/03_ARCHITECTURE.md)** — Phaser/React + Go + PostgreSQL/Redis.
- **[Mô hình dữ liệu đề xuất](docs/04_DATA_MODEL_DRAFT.md)**, **[API đề xuất](docs/05_API_EVENTS_DRAFT.md)**.
- **[Kiểm thử và điều kiện Beta](docs/06_ACCEPTANCE_QA.md)**.
- **[Roadmap lập trình](docs/07_ROADMAP.md)**, **[Git/deploy](docs/08_GIT_DEPLOYMENT.md)**, **[Các điểm còn thiếu](docs/09_OPEN_ISSUES.md)**.
- **[Nguồn gốc dữ liệu](docs/10_SOURCE_MAP.md)** và GDD 17 phân hệ trong `docs/archive/`.

## Công nghệ đã chốt

Client `Phaser + React + TypeScript`, Game Server `Go`, Transaction Service `Go`, PostgreSQL, Redis, WSS JSON+binary, bản đồ Tiled. Monorepo 1 Repository, code trên máy cá nhân → Git → deploy qua script đến môi trường **tương thích**.

## Thư mục

```text
client/                # Phaser + React demo
 game-server/           # Go health endpoints; multiplayer chưa làm
 transaction-service/   # Go health endpoints; transactions chưa làm
 database/migrations/   # migration placeholder
 deploy/                # compose local + Windows local .bat + lưu ý VPS
 shared/                # giao thức/dữ liệu chung (placeholder)
 docs/                  # GDD mới, quyết định, lộ trình, QA, archive
 master-data/           # CSV / JSON export cấu hình & backlog
```

## Chạy demo trên máy hỗ trợ Docker Linux containers

```powershell
.\deploy\windows\start-local.bat
# http://localhost:5173 (demo client)
# http://localhost:8081/healthz
# http://localhost:8082/healthz
```

Dừng bằng `.\deploy\windows\stop-local.bat`. **Go readiness endpoint cố ý chưa OK**: chưa có WSS, DB auth, ledger hoặc gameplay nhiều người chơi. Xem chi tiết trong `docs/archive/STARTER_README_v0.1.md`.

## Cảnh báo triển khai quan trọng

VPS hiện có là **Windows Server 2012 R2 Datacenter**. Không hỗ trợ WSL2 / Docker Linux containers như kế hoạch ban đầu. **Không có BAT tự deploy toàn bộ stack an toàn đã kiểm chứng cho VPS này**. Dùng máy này làm Closed Beta chỉ sau khi lựa chọn runtime/hạ tầng hợp lệ, test và giới hạn rủi ro. **Không mở Open Beta không reset trên hạ tầng một VPS cũ hiện tại** khi chưa đạt tiêu chuẩn P0-161.

## Bước tiếp theo

**M0** push starter và docs lên repo rỗng, chạy local + CI. **M1** làm migrations PostgreSQL + đăng nhập và nhân vật. Sau đó M2 map multiplayer, M3 ledger/inventory, M4 farm/NPC, M5 chat, M6 Admin/fashion/quests, M7 Closed Beta, M8 kiểm thử/chuẩn bị Open Beta. Không có mốc thời gian ảo khi chưa biết tài nguyên triển khai.

**Không commit `.env`, mật khẩu, SSH keys, token hay dữ liệu tài khoản người chơi.**
