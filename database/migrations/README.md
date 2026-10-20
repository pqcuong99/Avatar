# Database migrations

Nơi đặt các SQL migration PostgreSQL theo thứ tự phiên bản.

**Chưa sinh schema tài khoản/sổ cái giả định:** cần xây dựng migration chính thức cùng nghiệp vụ kiểm tra, transaction và idempotency của dịch vụ Go. Không tùy ý tạo tài sản hoặc bảng kinh tế trước khi review các ràng buộc đã chốt.
