# VPS Windows Server 2012 R2 — trạng thái và lộ trình

**Đã xác minh qua ảnh PowerShell người dùng:** Windows Server 2012 R2 Datacenter, Version 6.3.9600, Build 9600.

- Windows Server 2012 R2 không có WSL2; **không thể dùng phương án Ubuntu + Docker Engine trong WSL2** đã từng cân nhắc.
- Nhiều phiên bản Go hiện đại không hỗ trợ hệ điều hành này; không cam kết chạy bản Go hiện đại trực tiếp ở đây.
- Docker Desktop/Linux containers **không phải phương án triển khai được hỗ trợ** trên Windows Server 2012 R2.
- Trạng thái: VPS này chỉ được dự kiến làm Closed Beta tạm thời; chưa có một runtime tương thích đã được xác nhận. **Không có script auto-run production cho VPS cũ trong starter này.**
- Đừng mở truy cập công khai trên bản Windows Server cũ khi chưa có biện pháp bảo vệ phù hợp và xác minh an toàn. Không đưa tiền thật vào Closed Beta.

## Quy trình Git mong muốn của chủ dự án

1. Viết code ở máy cá nhân.
2. Commit/push lên một Git repo.
3. Khi có môi trường chạy tương thích, file `.bat` trên máy cá nhân có thể SSH tới máy chủ và gọi script `git clone`/`git pull`, build và restart dịch vụ.
4. Mọi cập nhật khi có người chơi phải qua bảo trì an toàn (P0-152).

## Điều kiện cần xác nhận trước khi làm BAT deploy VPS

- Quyền của VPS, khả năng cài lại OS hoặc chạy VM Linux phù hợp, tài nguyên RAM/CPU/SSD.
- Máy chạy Go tương thích và kế hoạch bảo mật có thể thực hiện; nếu không, chạy Beta trên một máy/hạ tầng khác.
- Git repository URL, cơ chế SSH, nhánh phát hành, đường dẫn deploy, cách giữ bí mật và sao lưu.

**Dù đang phát triển trên một máy, yêu cầu Open Beta không reset vẫn đòi hỏi kiến trúc dự phòng/sao lưu và tiêu chuẩn kiểm thử đã chốt (P0-139, P0-148–150, P0-161).**
