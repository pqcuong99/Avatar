# M1-01 — Đăng ký, đăng nhập tài khoản (bản thử nghiệm v0.2)

**Trạng thái**: Mã nguồn mẫu để chạy trên **máy cá nhân bằng Docker**, chưa phải hệ thống xác thực hoàn chỉnh và **không được mở Internet/Closed Beta**.

## Những gì bản này thực sự triển khai

- Migration `database/migrations/0001_auth.sql`: `accounts` và `auth_sessions`, vẫn giữ volume PostgreSQL hiện tại.
- Go Game Server (8081): `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me`, `GET /api/v1/auth/health`.
- Mật khẩu băm bcrypt (không lưu mật khẩu thuần); token phiên ngẫu nhiên 256-bit, chỉ hash lưu DB, cookie HttpOnly + SameSite=Lax, cookie `Secure` bật ở HTTPS triển khai sau.
- Đăng nhập mới tăng `login_generation`, khiến phiên cũ hết hiệu lực khi được kiểm tra lại ở backend.
- UI React đăng ký/đăng nhập/đăng xuất, hiển thị bản đồ demo sau khi đăng nhập.
- PostgreSQL có dữ liệu tài khoản bền vững qua restart container **nếu không xóa volume**.

## Chạy sau khi chép patch vào thư mục F:\Avatar

1. Mở Docker Desktop trên máy Windows cá nhân, đợi Docker Engine hoạt động.
2. Mở CMD:

   ```bat
   cd /d F:\Avatar
   .\deploy\windows\start-local.bat
   ```

3. Mở `http://localhost:5173`; đăng ký bằng một tài khoản thử nghiệm, sau đó đăng nhập.
4. Kiểm tra DB qua `http://localhost:8081/api/v1/auth/health` (phải trả về `{"service":"auth","status":"ok"}`).
5. Kiểm tra tài khoản đã lưu:

   ```bat
   docker compose -f deploy\compose.local.yml exec postgres psql -U thanh_dev -d thanh_dev -c "SELECT username, created_at FROM accounts;"
   ```

6. Refresh trang: phiên đăng nhập phải còn hiệu lực. Logout rồi refresh: phải quay về form đăng nhập.

Nếu Docker build lỗi (cần tải các module Go mới qua Internet):

```bat
docker compose -f deploy\compose.local.yml logs --tail 100 game-server db-migrate
```

**Không bao giờ dùng `docker compose down -v` để dừng game**: `-v` xóa toàn bộ dữ liệu PostgreSQL và Redis local.

## Hạn chế chưa hoàn thiện (bắt buộc trước Beta)

- `username` đăng nhập tạm dùng ASCII `[A-Za-z0-9_]{3,32}`, **chưa phải luật đặt tên nhân vật**. Tên nhân vật theo P0-020–025 hỗ trợ Unicode/dấu tiếng Việt và sẽ được triển khai trong M1-02.
- Mật khẩu tạm 12–72 byte; yêu cầu mật khẩu cuối cùng, email xác minh, đặt lại mật khẩu 10 phút chưa hoàn tất.
- Một tài khoản chỉ được có 1 nhân vật: bảng `characters` và màn tạo nhân vật sẽ làm **M1-02**, chưa tồn tại trong patch.
- Chưa cấp 5.000 Xu/0 Kim Cương khi tạo nhân vật; việc này phải đi qua Transaction Service cùng ledger (M3), **không thực hiện ở đăng ký tài khoản**.
- 2FA tùy chọn người chơi, bảo vệ đa tầng chống bot theo P0-145, captcha, rate-limit phân tán và audit sự kiện vẫn chưa đạt yêu cầu Beta. Bộ đếm thử theo IP trong bộ nhớ Go chỉ để demo, không thay thế rate limiter ở Gateway/Redis.
- `GET /readyz` vẫn trả 503 cho toàn bộ game vì phòng chơi, WebSocket chưa được triển khai; **đúng với trạng thái starter**. `GET /healthz` và `/api/v1/auth/health` mới là healthcheck của giai đoạn này.
- Chính sách cookie và CORS ở bản demo **chỉ dành cho origin `http://localhost:5173` và giao thức HTTP localhost**. Khi đưa lên môi trường khác, bắt buộc HTTPS/WSS, cookie Secure, reverse proxy đáng tin cậy, CSRF/security headers và kiểm thử bảo mật.
- `go.sum` sẽ được tạo trong Docker lúc build. Trước khi đóng băng phiên bản production phải chạy `go mod tidy` trên máy phát triển và commit `go.sum` để đảm bảo kiểm soát phụ thuộc.
- Phần lớn giao dịch quan trọng vẫn chưa cài đặt; không xem bản này là hoàn thiện MVP.

## Các bất biến nghiệp vụ đã được chuẩn bị

- `accounts.username_key` UNIQUE, không phân biệt hoa thường cho tài khoản đăng nhập trong phạm vi ASCII tạm thời.
- `auth_sessions.login_generation` đối chiếu `accounts.login_generation` để chỉ phiên đăng nhập mới nhất có hiệu lực.
- Không dùng session cookie từ Client để quyết định tài sản/di chuyển. Khi nối WebSocket phải xác thực phiên trên server.
- Các thay đổi về nhân vật sẽ được thiết kế đúng quy tắc `1 account = 1 character`; không thêm API xóa nhân vật.

**Thứ tự sau patch:** (1) Kiểm thử luồng Auth trên máy cá nhân → (2) PostgreSQL schema nhân vật, tên Unicode và kiểm tra trùng → (3) Form tạo nhân vật → (4) Bản đồ nhiều người/WS.
