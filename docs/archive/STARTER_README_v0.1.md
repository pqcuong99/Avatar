# Project Thanh — Monorepo Starter v0.1

**Tình trạng: bộ khung lập trình, chưa phải game MVP hoàn chỉnh và chưa an toàn để phát hành công khai.**

Đã chốt: Client = Phaser 3 + React + TypeScript; Game Server = Go; Transaction Service = Go; PostgreSQL + Redis; mono-repository Git (P0-164, P0-165, P0-180, P0-181).

## Có gì trong bản này?

- Web client React + Phaser: bản đồ và một nhân vật demo có thể điều khiển bằng WASD/phím mũi tên/nhấp chuột. **Chỉ mô phỏng ở Client, chưa đồng bộ Server**.
- `game-server/` Go: HTTP `/healthz`, `/readyz`. Chưa triển khai WSS, chia phòng, xác thực di chuyển.
- `transaction-service/` Go: HTTP `/healthz`, `/readyz`. Các API giao dịch trả `501 Not Implemented`, **không thay đổi tài sản**.
- `database/` chứa vị trí đặt migrations; PostgreSQL, Redis chạy local bằng Docker Compose.
- `deploy/compose.local.yml` chạy toàn bộ stack phục vụ **phát triển trên máy cá nhân**, không phải production.
- `deploy/windows/start-local.bat`: một lần chạy để khởi động môi trường local bằng Docker Compose.

## Chạy thử nhanh trên Windows (máy cá nhân)

Điều kiện: Git và Docker Desktop hỗ trợ Linux containers/Compose, ảo hóa tương thích. **Không áp dụng cho VPS Windows Server 2012 R2**.

1. Giải nén ZIP / clone repository về máy Windows của bạn.
2. Chạy `deploy\\windows\\start-local.bat` (hoặc từ thư mục gốc `docker compose -f deploy/compose.local.yml up --build -d`).
3. Mở http://localhost:5173 để xem bản đồ demo.
4. Game Server: http://localhost:8081/healthz.
5. Transaction Service: http://localhost:8082/healthz.
6. Dừng dịch vụ bằng `deploy\\windows\\stop-local.bat`.

Các image Docker và dependency Node.js được tải lần đầu từ Internet. API `/readyz` cố ý trả HTTP 503 vì dịch vụ chưa tích hợp Database, giao dịch hoặc multiplayer; không dùng như dấu hiệu sẵn sàng production. PostgreSQL và Redis không công khai cổng ra ngoài máy localhost. **Mật khẩu trong `compose.local.yml` chỉ dùng cho phát triển local**; không được sử dụng trên VPS công khai.

## Chạy Go không cần Docker (kiểm thử khởi động từng dịch vụ)

```powershell
cd game-server
go run .
```

Mở PowerShell khác:

```powershell
cd transaction-service
go run .
```

Có thể chạy bằng Go 1.23+ trên **hệ điều hành được phiên bản Go đó hỗ trợ**. Dockerfile Go 1.23 chỉ nhằm đồng bộ môi trường kiểm thử starter; **phải nâng lên một phiên bản Go còn được vá bảo mật trước khi chạy công khai**. Trên máy Windows, bản demo Client độc lập có thể chạy bằng `cd client; npm install; npm run dev` (yêu cầu Node.js tương thích).

## Giới hạn triển khai Windows Server 2012 R2

VPS hiện tại là **Windows Server 2012 R2, build 9600**. Không có WSL2; không chạy Docker Engine Linux qua WSL2; các bản Go hiện đại không hỗ trợ Windows Server 2012 R2 như môi trường chạy được bảo đảm. **Không chạy các script local Docker của dự án này trên VPS đó.**

Không có cách trung thực nào để cam kết "clone Git + một BAT chạy được toàn bộ Go + Linux Docker containers" trên VPS cũ mà chưa có phương án môi trường chạy tương thích. Xem `deploy/README_VPS_WIN2012.md` để biết cách tiếp tục kiểm tra và lộ trình hạ tầng. Quy trình Git, code và thư mục đã chuẩn bị để chuyển sang Linux/Docker khi có máy chủ phù hợp.

## Bước lập trình kế tiếp (chưa triển khai)

1. Schema PostgreSQL có migrations, tài khoản/người chơi, khóa mật khẩu và quản lý phiên an toàn.
2. WSS + phòng chơi, giao thức JSON/nhị phân, server authoritative, chống gian lận di chuyển.
3. Dịch vụ giao dịch Go thật: PostgreSQL transaction, idempotency key, ledger bất biến, khóa khi mất đồng bộ an toàn.
4. Nông trại và NPC theo quyết định P0; chat/bạn bè; Admin RBAC/2FA; thử nghiệm tải.

Không dùng starter làm hệ thống thực nhận tiền hoặc quản lý tài sản người chơi trước khi hoàn thành và kiểm thử đầy đủ.

## Git

```bash
git init
git add .
git commit -m "bootstrap Project Thanh monorepo"
# Tạo repository riêng tư trên Git provider, sau đó:
git remote add origin <URL_REPOSITORY_CUA_BAN>
git push -u origin main
```

Nếu branch mặc định chưa là `main`, dùng `git branch -M main` trước khi push. Không commit mật khẩu, file `.env`, cơ sở dữ liệu hoặc dữ liệu người chơi.
