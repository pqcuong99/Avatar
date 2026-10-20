# HƯỚNG DẪN GIT, CHẠY LOCAL, TRIỂN KHAI VÀ GIỚI HẠN VPS

## 1. Repository được xác nhận

`https://github.com/pqcuong99/Avatar.git` — người dùng xác nhận mới tạo và đang trống. **Chưa có bằng chứng đã push**. Bộ bàn giao này **chưa được tự động đẩy** lên GitHub.

## 2. Đưa gói `Repo_Ready` lên GitHub lần đầu (Windows PowerShell)

Cài Git, chuẩn bị quyền push Repo. Giải nén gói và làm theo thứ tự:

```powershell
# Clone repository mới tạo. Nếu repo rỗng, lệnh vẫn tạo thư mục Avatar.
git clone https://github.com/pqcuong99/Avatar.git

# Copy TẤT CẢ nội dung bên trong Project_Thanh_Repo_Ready_2026-10-20/
# vào thư mục Avatar/ vừa clone (không copy cả folder bao ngoài).
cd Avatar

git status
git add .
git status
# Kiểm tra KHÔNG có file .env, khóa riêng, mật khẩu hoặc data nhạy cảm.
git commit -m "chore: bootstrap Project Thanh monorepo and specifications"
git push -u origin HEAD
```

**Lưu ý:** `git push` cần đăng nhập GitHub bằng credential manager hoặc SSH. Nếu repo GitHub đã tự có README/LICENSE, clone sẽ chứa những tệp đó; **so sánh rồi merge, không xóa ẩu**. Nếu GitHub cấu hình bảo vệ nhánh, tạo nhánh và Pull Request.

## 3. Chạy dự án ở máy phát triển hỗ trợ Docker Linux containers

Điều kiện: Git, Docker Engine/Compose hoặc Docker Desktop trên Windows **được hỗ trợ**, dịch vụ ảo hóa bật.

```powershell
cd Avatar
.\deploy\windows\start-local.bat
# Mở: http://localhost:5173
# Go server health: http://localhost:8081/healthz
# Transaction health: http://localhost:8082/healthz
# Dừng:
.\deploy\windows\stop-local.bat
```

Các endpoint `/readyz` trả chưa sẵn sàng đúng ý vì chưa có DB integration/transaction thật. Không triển khai starter như môi trường công khai. Mật khẩu Docker local chỉ dùng local, không được dùng trên máy chủ Internet.

## 4. Chu trình làm việc hằng ngày

1. `git pull` trước khi code; mở branch `feature/...` cho một module.
2. Code + test tại máy cá nhân; không commit secrets hoặc dump DB.
3. `go test ./...` cho mỗi Go module (từ thư mục module), `npm run build`/type-check cho Client; commit có ý nghĩa.
4. `git push`, review và merge. Đánh dấu version/release tag trước bảo trì.
5. Trên môi trường triển khai **đã xác minh tương thích**, script tự checkout release → lưu/kiểm tra backups → build/restart có kiểm soát → health check → đối soát.

## 5. Quan trọng — VPS Windows Server 2012 R2 Datacenter

Nguồn kiểm tra bằng ảnh: Windows Server 2012 R2 Datacenter, version **6.3.9600**, build **9600**. Người dùng muốn dùng **một VPS cũ tạm cho Closed Beta**, chuyển hạ tầng khi hoàn thiện.

**Không thể mặc định dùng** Windows Server 2012 R2 + WSL2 + Ubuntu + Docker Engine. Windows Server 2012 R2 **không có WSL2** và không chạy Docker Desktop/Linux containers được hỗ trợ. Phiên bản Go hiện đại không được bảo đảm hỗ trợ Windows Server 2012 R2. VPS này còn là hệ điều hành ngoài thời hạn hỗ trợ thường lệ.

### Điều gì CHƯA có

- Chưa có cách chạy toàn bộ stack mới một cách được xác minh trên Windows 2012 R2.
- Chưa có `deploy-to-vps.bat` chạy được trên cấu hình thực tế; không trình bày file giả khả năng.
- Chưa có PostgreSQL HA/failover độc lập, auto-scale hoặc chống DDoS đầu cuối trên một VPS.

### Cách tháo gỡ kỹ thuật trước Closed Beta

- Kiểm tra CPU/RAM/ổ SSD, quyền quản trị, internet/firewall, dịch vụ đang chạy, điều khoản VPS, hỗ trợ VM/upgrade OS.
- Quyết định môi trường chạy tương thích: ưu tiên máy Linux phù hợp (VPS khác hoặc nâng cấp OS) cho Go + Docker; hoặc môi trường test nội bộ được bảo vệ, **không** dùng bản Go ngừng hỗ trợ để chạy Internet.
- Sau khi có runtime tương thích, mới triển khai `.bat` Windows → SSH và script máy chủ `git fetch` + checkout tag + `docker compose up -d --build` cùng preflight/backups/health checks.

### Với một VPS không có dự phòng

Phù hợp prototype/dev; **không** đạt mục tiêu HA PostgreSQL P0-149 và Game Server P0-148. Cơ chế P0-150 yêu cầu dừng giao dịch tài sản nếu không thể bảo đảm dữ liệu ghi an toàn, vì thế chưa thể tuyên bố Open Beta không reset sẽ hoạt động trên đúng VPS hiện tại.

## 6. Mẫu checklist triển khai khi đã có môi trường tương thích

- [ ] Có tag Git đã kiểm thử, môi trường env vars được bảo mật.
- [ ] Thông báo bảo trì; dừng thao tác mới; đối soát giao dịch đang xử lý.
- [ ] Kiểm tra backup ngoài máy chủ và PITR, snapshot mốc khôi phục.
- [ ] Pull image/code đã chọn, chạy DB migrations tương thích.
- [ ] Health/ready check, kiểm thử đăng nhập, farm, NPC, chat, ledger.
- [ ] Nếu lỗi: tiếp tục bảo trì; xử lý rollback/forward-fix không làm mất giao dịch sau mốc backup.
- [ ] Mở lại và theo dõi lỗi/latency/tải/log.

Không lưu private SSH key trong repo; không hardcode mật khẩu trong `.bat` hay Docker Compose.
