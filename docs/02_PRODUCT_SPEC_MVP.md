# ĐẶC TẢ SẢN PHẨM MVP — Project Thanh

**Phiên bản:** 1.1 — cập nhật quyết định P0 đến 183 (20/10/2026).  
**Tài liệu nền 17 phân hệ:** `archive/GDD_baseline_2026-10-19.md`.  
**Độ chín:** Các nguyên tắc và ràng buộc chính đã chốt; giao diện chi tiết, master data và một số luồng lỗi còn cần kiểm chứng.

## 1. Mục tiêu và thiết bị

[CHỐT] Game Avatar 2D dành cho trình duyệt PC và điện thoại (**mobile ngang**). PC dùng WASD/mũi tên và nhấp điểm đến; mobile dùng joystick ảo và chạm điểm đến. Tác động thủ công hủy auto-path ngay. Game có đời sống cộng đồng, nông trại, NPC, vật phẩm/thời trang, nhiệm vụ và hệ thống kinh tế.

**Vòng trải nghiệm trọng tâm MVP:** đăng ký → tạo nhân vật → vào phòng/map → di chuyển/chat → mua hạt NPC → trồng → xử lý sâu → thu hoạch → túi/chờ nhận → bán nông sản lấy Xu → dùng Xu ở NPC → kết bạn/chat riêng → thay thời trang → nhiệm vụ ngày. Đây là mô tả tích hợp, không xác nhận thứ tự bắt buộc cho mọi người chơi.

## 2. Phạm vi MVP đã chốt

| Phân hệ | Các khả năng bắt buộc trong MVP | Dẫn chứng |
|---|---|---|
| Tài khoản & nhân vật | Đăng ký bằng user/pw; 1 nhân vật/account; lưu trạng thái; phiên đơn; khôi phục mật khẩu theo điều kiện đã chốt; 2FA tùy chọn người chơi | P0-006–029, 146 |
| Bản đồ & di chuyển | Tiled JSON; mobile ngang/PC; pathfinding Client; Server xác thực; tự chia phòng; nút gặp bạn bè; reconnect 30s | P0-001–010, 134–138, 166–169 |
| Chat/bạn bè | Chat công cộng cùng phòng; tìm người/kết bạn/danh sách bạn bè; chat riêng; các ràng buộc riêng tư/giới hạn lịch sử trong GDD cũ khi module bật | P0-155 và TH-05–07 |
| Nông trại | 6 ô ban đầu, 5 cây; mua hạt, trồng, nhổ, thu, thu tất cả, bán NPC; sâu xuất hiện, bắt thủ công, phạt sản lượng | P0-058–102, 156 |
| Túi/chờ nhận | 10 ô ban đầu; +5/upgrade; farm/nâng cấp 999/ô, thời trang 1 món/ô; nhận chờ không mất tài sản | P0-068–093 và TH-02 |
| NPC & kinh tế | NPC đa menu trái/nội dung phải; Xu/KC gốc 5.000/0; mua/bán với NPC; giá/config sau bảo trì; transaction atomic, sổ cái | P0-030–039, 066, 141, 165 |
| Thời trang | NPC bán đồ cơ bản; mặc/tháo; hiển thị trang phục cho người chơi cùng phòng | P0-158 |
| Nhiệm vụ cá nhân ngày | Nhiệm vụ cơ bản theo hoạt động MVP, cấp thưởng chính xác, chống nhận trùng | P0-159 |
| Admin & vận hành | RBAC 4 vai trò, Admin TOTP bắt buộc, điều chỉnh tài sản hai người, nhật ký, sao lưu, bảo trì và test | P0-139–152, 161 |

**Hoãn khỏi MVP theo quyết định**: nạp tiền thật QR/webhook (P0-154), chợ người chơi (P0-157), nuôi chim từ sâu (P0-156). Bang hội, BXH, câu cá/mini-game, nội dung nâng cao trong GDD 17 phân hệ **chưa có quyết định P0 mới buộc đưa vào MVP**; vì vậy không coi là MVP bắt buộc, nhưng vẫn giữ thiết kế cũ cho sau này.

## 3. Định danh và tài khoản

[CHỐT] Một tài khoản một nhân vật, không xóa/tạo lại. Tên duy nhất toàn cục, so sánh không phân biệt hoa/thường nhưng có phân biệt dấu tiếng Việt; 3–16 ký tự hiển thị (Unicode grapheme), tập ký tự cho phép gồm tiếng Việt, số, khoảng trắng và `_ - . ★ ♥ ☆ ♡`. Đổi tên bằng 1 thẻ chỉ khi đổi thành công, không cooldown. Cơ chế khóa/cấm theo bản GDD cũ; cần đối chiếu chi tiết quyết định cũ chưa có transcript.

Đăng nhập phiên mới đá phiên cũ; reconnect tự đồng bộ; nhân vật đứng yên tối đa 30s mất mạng và giữ chỗ phòng, sau đó offline; reconnect muộn lấy vị trí server lưu và có thể vào phòng khác. Tên đăng nhập và email khôi phục phải xử lý an toàn; không coi email là bắt buộc.

## 4. Tiền tệ, túi đồ, tính nhất quán

[CHỐT] Ban đầu 5.000 Xu, 0 Kim Cương. Giới hạn ví Xu ban đầu 999.999.999. Hạn mức KC Admin cấu hình; KC thưởng vượt trần thành các khoản chờ nhận, không hết hạn, nhận theo FIFO, cần có chỗ. Nguồn nạp KC tiền thật **chưa bật MVP** dù mô hình nạp đã thiết kế.

Hạt giống/nông sản **không giao dịch giữa người chơi và không thuộc loại khóa/không khóa**. Nông sản bán NPC, không bán thẳng từ hàng chờ. Túi đầy không xóa lô sản phẩm. Với thu hoạch một cây, nếu cả lô không thể cho vào túi thì **toàn bộ lô chuyển chờ**. Chờ nông sản cùng loại gộp một dòng (giữ timestamp sớm nhất); nhận nguyên dòng, ngoại lệ cho phép nhận một phần nếu dòng vượt dung lượng túi tối đa có thể có. Các thao tác phải nguyên tử/chống lặp. **Đặc tả chi tiết phải áp dụng đầy đủ P0-075–085**, không tự đơn giản hóa.

**CẦN ĐỐI CHIẾU**: Tài liệu GDD 19/10 ghi quy tắc phần Xu vượt trần có thể bị tiêu hủy không cảnh báo. Chưa có nguyên văn tất cả tình huống ngoại lệ; không tự code chính sách này trước khi xác minh.

## 5. Nông trại và sâu bệnh

[CHỐT] Cây đầu tiên: cà rốt, khoai tây, cà chua, ngô, bắp cải; thời gian lần lượt **30, 60, 120, 180, 240 phút**. Giá hạt *tham chiếu* lần lượt **100, 250, 500, 750, 1.000 Xu**, chốt giá trước mở Server; giá bán NPC và sản lượng do Admin điền trước phát hành. Cây lớn bằng mốc thời gian Server kể cả Offline/bảo trì; thời gian lớn và sản lượng cơ bản snapshot lúc gieo. Cây chín không héo. Mỗi cây một lượt thu hoạch rồi biến mất.

Sâu xuất hiện tối đa 1 lần trong giai đoạn lớn của một cây theo xác suất từng loại do Admin cấu hình; chỉ phát sinh trong **70% đầu** thời gian lớn. Thời hạn xử lý tính từ lúc sâu xuất hiện bằng **30% tổng thời gian lớn của cây**. Bắt sâu thủ công miễn phí, sâu vào túi stack 999. Nếu bỏ quá hạn: trừ sản lượng bằng `floor(yield_snapshot × random_percent / 100)` với tỷ lệ nguyên 5–10%; làm tròn thành 0 thì trừ 0. Sự kiện sâu và tỷ lệ phạt đã chọn phải được lưu bền vững/chống random lại khi reconnect.

Hệ thống nuôi chim 12h/720h, kho sâu, bắt sâu tự động là **thiết kế sau MVP** (P0-103–133). Không xóa thiết kế.

## 6. Xã hội, NPC, thời trang, nhiệm vụ

[CHỐT] Chat công cộng trong phòng; kết bạn chủ động theo người chơi tìm được; chat riêng có cơ chế lịch sử 30 ngày, lazy-load và quy tắc chặn đã kiểm kê trong GDD, khi bật cần giữ toàn bộ tính đúng và quyền riêng tư. NPC không đứng chồng nhau, người chơi không đi xuyên NPC. NPC đa chức năng: **menu bên trái, nội dung bên phải**, đổi menu đổi nội dung mà không mở lại cửa sổ. Giá/NPC/bảng tin được cập nhật theo **đợt bảo trì**, không cập nhật mid-transaction.

Thời trang cơ bản: NPC bán trang phục, mua/mặc/tháo, đồng bộ trạng thái khi cùng phòng. Nhiệm vụ hằng ngày: ít nhất loại trồng, thu, bán, chat và hoạt động tương thích MVP; phần thưởng, hạn mức, cách làm mới cần cấu hình/kiểm thử trước khi phát hành, không bịa giá trị.

## 7. Beta & phát hành

[CHỐT] Đợt đầu Closed Beta thử nghiệm trên VPS đang có **nếu tìm được cách chạy tương thích và an toàn**; người chơi **tự đăng ký**, không whitelist/mã mời. Kết thúc Closed Beta **xóa toàn bộ dữ liệu người chơi**; vẫn giữ Admin/cấu hình và lưu bản sao lưu có kiểm soát. Người chơi phải được thông báo trước về reset. Open Beta sau này là **không reset** và đòi hỏi vượt kiểm thử an toàn tài sản, bảo mật, backup/PITR, failover và **tối thiểu 300 online đồng thời**. 300 là mục tiêu thử tải, không phải giới hạn account.

## 8. Kết quả đầu ra mong đợi

- Người chơi mới có thể tự đăng ký, tạo nhân vật, vào phòng, di chuyển mượt và chat.
- Người chơi có thể trồng/thu/bán và thấy thay đổi Xu + túi chính xác, kể cả khi túi đầy/mạng chập chờn.
- Người chơi có thể kết bạn/chat riêng, thay thời trang và hoàn thành nhiệm vụ ngày.
- Admin được xác thực bằng 2FA, quyền hạn đúng vai trò, thay đổi tài sản thủ công cần hai người độc lập.
- Giao dịch, sao lưu và khôi phục đáp ứng test; nếu không bảo đảm đồng bộ an toàn, giao dịch tài sản được tạm dừng.

**Các màn hình chi tiết, animation, hình ảnh, âm thanh, master data giá bán/năng suất và thử nghiệm UX chưa hoàn thiện**, không được coi là đã Ready for Dev đầy đủ chỉ vì có luật kinh doanh.
