# KHOẢNG TRỐNG & RỦI RO — CẦN ĐỐI CHIẾU TRƯỚC KHI CODE/OPEN BETA

Những mục ở đây **không phải lựa chọn đã chốt**. Chúng xác định những câu hỏi triển khai có ảnh hưởng lớn, ưu tiên theo rủi ro; không khởi động lại chuỗi hỏi chi tiết không cần thiết.

## Rủi ro Critical

1. **VPS Windows Server 2012 R2** chưa có runtime phù hợp cho Go hiện đại + Docker Linux; hệ điều hành hết hỗ trợ. Kế hoạch dùng tạm Closed Beta không tự động giải quyết khả năng chạy/khả năng bảo mật. **Cần test máy thật hoặc đổi môi trường thử nghiệm.**
2. **Một VPS ≠ HA**: không thỏa tự phục hồi khi VPS chết và PostgreSQL dự phòng đồng bộ theo P0-148–150; **không tuyên bố đủ điều kiện Open Beta** cho đến khi có hạ tầng độc lập.
3. **Sổ cái/asset** mới ở mức quyết định kiến trúc; chưa có migration, transaction thật, công cụ restore/đối soát. Ưu tiên M3 trước mở Beta.
4. **Bản ghi reset Closed Beta**: muốn xóa toàn bộ account/player, giữ admin/config; cần sơ đồ FK, test reset trên staging, backup, xác nhận rõ chính sách retention; **không cho phép reset trên Open Beta**.
5. **Bảng giá bán nông sản, yield mỗi cây** chưa có số chính thức. P0-062 để Admin cấu hình; yêu cầu master data trước test kinh tế.

## Cần quyết định khi module vào code

| ID | Chủ đề | Vấn đề còn thiếu / lưu ý |
|---|---|---|
| GAP-01 | Xu vượt trần | GDD 19/10 có quy tắc Xu vượt trần bị tiêu hủy không cảnh báo; thiếu nguyên văn các ngoại lệ. Không tự code khi chưa đối chiếu |
| GAP-02 | API/transport | JSON/binary đã chốt loại, nhưng protobuf/custom schema, tick rate, message size, retry policy chưa chốt |
| GAP-03 | Nhiệm vụ ngày | Số nhiệm vụ/ngày, bộ nhiệm vụ, cách reset timezone, lượng thưởng/cap chưa chốt |
| GAP-04 | Nông trại | Yield/sell prices, từng crop pest probability, giá/mức tăng ô đất và túi chưa cấu hình |
| GAP-05 | NPC/shop | Danh sách NPC, tọa độ Tiled, toàn bộ menu/shop catalog, giá thời trang MVP còn thiếu |
| GAP-06 | Chat | GDD cũ có seen/read dựa viewport, typing, block, retention 30d; cần đọc chi tiết trước triển khai |
| GAP-07 | Thời trang | Bộ asset/item ban đầu, cơ chế slot mặc và đồng bộ sprite chưa thiết kế kỹ thuật |
| GAP-08 | Chống tấn công | Nhà cung cấp WAF/DDoS, chiến lược auth session, Admin key recovery, rate-limit threshold chưa chốt |
| GAP-09 | Nguồn thiết kế cũ | Thiếu transcript nguyên văn 1–1198, nhất là các đoạn 1–660/798–959; GDD cũ có đánh dấu xác minh |
| GAP-10 | Beta operations | Ngày giờ sao lưu, thời gian lưu, người vận hành, quy trình sự cố, RPO/RTO, giám sát chưa có mức định lượng |
| GAP-11 | VPS | IP/SSH access, port policy, CPU/RAM/storage, nested virtualization và nhà cung cấp chưa được xác nhận; không cần chia sẻ mật khẩu cho ChatGPT |
| GAP-12 | Xu vs KC | Các quy tắc chờ nhận khác nhau giữa KC và nông sản; cần cấm trộn hai ledger/pending flows |

## Những điều KHÔNG ĐƯỢC nhầm

- `P0-174 A` chọn WSL2 đã **bị thực tế VPS 2012 R2 phủ định**; quy tắc hiện hành là P0-176 giữ VPS để thử tạm, với cảnh báo kỹ thuật.
- `P0-177` từng mô tả chỉ người được mời, nhưng **P0-178 được người dùng sửa thành đăng ký tự do**. Chính sách mời không áp dụng.
- GDD 19/10 và Roadmap 19/10 từng ghi **MVP chưa phê duyệt**; nay P0-153–162 đã phê duyệt nội dung MVP và Open Beta. Không dùng lại bảng phân kỳ cũ như hiện hành.
- Không cho rằng đã được 300 online chỉ vì có nhiều phòng hoặc viết Go.
- Không cho rằng git repo đã có commit chỉ vì user cung cấp URL; người dùng xác nhận repo mới rỗng.
- Không biến việc đã viết tài liệu thành đã lập trình Game Server, database transaction hoặc chức năng Admin.

## Gợi ý đánh dấu issue

`priority:P0`, `priority:P1`, `type:implementation`, `type:spec-gap`, `type:security`, `milestone:M0–M8` là nhãn **đề xuất** cho GitHub Issues, không tự áp dụng vào repo nếu chưa kết nối.
