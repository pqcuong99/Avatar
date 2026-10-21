# 01 — Đặc tả sản phẩm MVP (Product Requirements + GDD)

## 1. Định vị và nền tảng [CHỐT]

- Game 2D xã hội/phong cách Avatar: giao tiếp và nông trại là trọng tâm. Chạy trên trình duyệt PC và mobile (ngang).
- PC: WASD/mũi tên, click để tự tìm đường. Mobile: joystick ảo, chạm điểm đến. Điều khiển tay hủy auto-path.
- Chủ dự án triển khai theo từng module, build tích hợp/test liên tục (P0-163); mục tiêu test 300 CCU cho Open Beta (P0-162).

## 2. Chức năng MVP bắt buộc [CHỐT]

| Mã | Phạm vi chính thức | Ngoài MVP |
|---|---|---|
| MVP-01 | Đăng ký/đăng nhập, 1 nhân vật/account, phiên duy nhất, tùy chọn 2FA người chơi, khôi phục tài khoản | Không bỏ các yêu cầu bảo mật chỉ vì bản test |
| MVP-02 | Bản đồ Tiled, phòng tự phân, nhiều người cùng phòng, đi lại và reconnect | Không cần UI chọn kênh |
| MVP-03 | Chat công cộng, tìm người, kết bạn, danh sách bạn, chat riêng | Bang hội chưa bắt buộc |
| MVP-04 | Túi đồ, chờ nhận, Xu, Kim Cương, NPC mua/bán | Chợ P2P sau MVP |
| MVP-05 | Nông trại 6 ô, 5 cây, sâu bệnh và bắt sâu thủ công | Nuôi chim, kho thức ăn chim sau MVP |
| MVP-06 | Thời trang cơ bản, NPC bán đồ, mặc/tháo, đồng bộ ngoại hình | Thời trang nâng cao sau MVP |
| MVP-07 | Nhiệm vụ cá nhân hằng ngày, có thưởng | Nhiệm vụ bang/hoạt động nâng cao chưa bắt buộc |
| MVP-08 | Admin RBAC/TOTP/audit, bảo trì, backup và kiểm thử | Không được bỏ an toàn tiền tệ |
| MVP-09 | Closed Beta tự đăng ký, có reset | Nạp tiền thật không hoạt động ở MVP |

## 3. Luồng người chơi end-to-end [CHỐT yêu cầu, ĐỀ XUẤT thứ tự UI]

**Luồng A — Người chơi mới**

1. Mở game từ desktop/mobile ngang; đăng ký username/password; email khôi phục là tùy chọn.
2. Đăng nhập; phiên mới làm phiên cũ mất quyền.
3. Nếu account chưa có character, tạo 1 nhân vật tên hợp lệ; không hỗ trợ xóa để tạo lại.
4. Server lưu nhân vật, khởi tạo 5.000 Xu, 0 KC, 10 ô túi, 6 ô đất.
5. Game Server phân vào phòng còn chỗ trên bản đồ công cộng; hiển thị nhân vật của những người cùng phòng.
6. Người chơi di chuyển/nhấp điểm, tương tác NPC/nông trại/chat/bạn bè.

**Luồng B — Nông trại → Xu**

1. Mua hạt giống ở NPC (giá hiệu lực theo bản bảo trì gần nhất).
2. Chọn ô đất trống, trồng hạt; ghi mốc thời gian và snapshot yield/duration.
3. Trong giai đoạn tăng trưởng có thể phát sinh sâu theo xác suất riêng cây; người chơi bắt thủ công miễn phí nếu còn hạn.
4. Cây đạt thời gian trưởng thành thì chờ mãi, không héo.
5. Thu hoạch: nếu không đủ chỗ cho **toàn lô của một cây**, toàn lô sang pending; nếu đủ, vào túi.
6. Nhận pending vào túi theo luật; bán nông sản có trong túi cho NPC để tăng Xu.
7. Mọi bước thay đổi tài sản phải nguyên tử, có audit ledger và chống replay.

**Luồng C — Xã hội**

1. Chat công cộng trong phòng, xem/nghiêng theo nhân vật cùng phòng.
2. Tìm người dùng, mở Profile, bấm Kết bạn; sau khi được chấp nhận quản lý danh sách bạn.
3. Bấm Chat riêng từ Profile; lưu lịch sử, seen/typing/privacy theo đặc tả TH-05/06.
4. Nếu hai bạn ở phòng khác nhau, Profile có `Đến cùng phòng` nếu phòng đích còn chỗ (P0-135).

**Luồng D — Vận hành Beta**

1. Admin cấu hình master-data, test staging và thông báo bảo trì.
2. Closed Beta cho tự đăng ký, không mã mời/whitelist; không nạp tiền thật; không cam kết giữ dữ liệu.
3. Trước kết thúc phải sao lưu, thông báo reset; xóa account và mọi dữ liệu người chơi, giữ admin/master-data.
4. Chỉ sau khi kiểm thử đạt mới chuyển môi trường an toàn hơn và mở Open Beta không reset.

## 4. Tính năng, quyền, bất biến

- Một account một character; đăng ký không cấp nhân vật tự động nếu người chơi chưa tạo.
- Không giao dịch giữa người chơi đối với hạt giống và nông sản; chúng **không thuộc phân loại khóa/không khóa**.
- NPC không đứng trùng nhau, có collider; nhân vật không đi xuyên NPC.
- Không thay đổi giá/NPC/bản tin đang hoạt động ngoài đợt bảo trì.
- Các thay đổi tài sản được thực hiện qua Transaction Service Go, không bởi Client.
- Hiển thị kết quả giao dịch chỉ sau khi backend cam kết và ghi đủ điều kiện bền vững.

## 5. Trạng thái phát hành

### Closed Beta [CHỐT]

Đăng ký tự do; VPS Windows Server 2012 R2 chỉ dùng nếu có runtime đủ tương thích và đã quản trị rủi ro. Chưa phải nền tảng đủ chuẩn Open Beta. Kết thúc xóa toàn bộ player-data, giữ Admin/config. Báo trước việc xóa.

### Open Beta [CHỐT]

Không reset dữ liệu hợp lệ; 300 CCU là điều kiện test tối thiểu, không phải room cap; cần dự phòng DB/Game Server, offsite backup/PITR, chống DDoS/rate-limit, kiểm thử an toàn tài sản, bảo mật và giao dịch. Không có ngày release cố định.

## 6. Không được tự suy diễn

- Chưa được chốt: tọa độ NPC, asset/sprites, quy tắc animation, FPS/tick chuẩn, danh sách map MVP cụ thể, giá bán nông sản, yield, ngưỡng/bảng thưởng nhiệm vụ, giá đồ thời trang.
- Hệ thống phong cách Avatar không có nghĩa tự sử dụng hình ảnh/nhân vật có bản quyền của trò chơi khác.
- Thiết kế TH-01–TH-17 cũ có nhiều nội dung sau MVP hoặc chưa kiểm chứng, giữ nguyên dưới `reference/`.
