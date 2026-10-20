# Các quyết định kiến trúc dùng để tạo starter

- P0-163 A: triển khai từng giai đoạn, tích hợp và kiểm thử liên tục.
- P0-164 B: Phaser + React + TypeScript, Go Game Server, PostgreSQL và Redis.
- P0-165 A: transaction-service chuyên trách quản lý tài sản, phục vụ nhiều Game Server.
- P0-166 C: WSS JSON + binary (chưa triển khai trong starter).
- P0-167 A: Tiled Map Editor (chưa có map asset).
- P0-168 A: client pathfinding, server validate (starter có click-to-move demo, **chưa có pathfinding/collision thực tế**).
- P0-169 A: thời gian chuẩn từ Server và timestamp trong Database (chưa triển khai).
- P0-170 B: VPS tự quản lý; hiện chỉ một VPS.
- P0-172 A: Docker Compose (đối với môi trường Linux tương thích).
- P0-173 D: Windows Server, từ ảnh kiểm tra là Windows Server 2012 R2.
- P0-174 A: dự kiến WSL2 + Ubuntu + Docker, **bất khả thi trên Windows Server 2012 R2**, cần thay đổi môi trường chạy.
- P0-176: người dùng muốn dùng VPS hiện tại tạm cho Closed Beta, chuyển hạ tầng khi hoàn thiện.
- P0-177 A: Closed Beta có reset.
- P0-178: đăng ký tự do trong Closed Beta, không dùng mã mời.
- P0-179 A: reset hết người chơi, giữ Admin và cấu hình game.
- P0-180 A: monorepo Git.
- **P0-181 A: Go cho cả Game Server và transaction-service.**

**Tài liệu này ghi quyết định, không ngụ ý tất cả chức năng đã được code.**
