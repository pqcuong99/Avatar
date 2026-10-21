# 13 — Các hệ thống đã thiết kế nhưng hoãn khỏi MVP

**Không để AI khác tự thêm các module sau vào đợt phát hành MVP.** Đặc tả này giữ nguyên định hướng và quyết định để triển khai theo giai đoạn sau; nội dung không có trong nguồn phải đánh dấu GAP.

## 1. Nuôi chim bắt sâu [CHỐT P0-103–133, SAU MVP]

### Chu kỳ sống

- Mua chim tại NPC nông trại, số lượng chim tối đa do Admin cấu hình, tính cả chim non/chim lớn. Hạ trần phải grandfather số chim cũ, vẫn hoạt động; không mua thêm cho tới khi dưới trần.
- Chim non: thời gian 12 giờ **tính từ lúc mua**; được cho ăn sâu từng phần, lượng sâu cần để lớn do Admin cấu hình. Chỉ thành chim lớn khi **đủ 12 giờ + đủ tổng số sâu đã ăn**; nếu đã 12h nhưng chưa đủ sâu thì chờ cho ăn sau.
- Tuổi chim lớn chỉ bắt đầu khi trưởng thành; chim non không chết vì tuổi; chim lớn sống đúng **720 giờ** tính từ thời điểm trưởng thành, vẫn tính Offline.
- Chim trưởng thành không cần người chơi cho ăn; không chết đói nếu nông trại không có sâu.

### Chim bắt sâu tự động

- Hoạt động cả khi chủ Offline; mỗi chim bắt **tối đa 4 lần trong cửa sổ 60 phút trượt**, không phải 1 lần/15 phút.
- Ưu tiên sâu có deadline gần nhất, hòa thì plot ID nhỏ hơn.
- Sâu chim bắt được chim **ăn ngay**, không vào túi hoặc kho thức ăn của người chơi.
- Hành động bird capture/feeding/sell dùng timestamps Server và transaction/idempotency.

### Bắt sâu, túi, kho thức ăn

- Người chơi bắt sâu thủ công miễn phí; sâu vào túi stack 999, có thể chuyển hai chiều giữa túi và kho thức ăn.
- Kho thức ăn có sức chứa Admin cấu hình. Nếu Admin giảm cap thấp hơn tồn hiện có: giữ tồn cũ, dùng được nhưng không cho gửi thêm cho tới khi giảm dưới cap.
- Giao diện chuyển sâu hỗ trợ nút nhanh **1, 10, 100, Tối đa**, và ô nhập số nguyên dương. Nếu số lượng muốn chuyển vượt khả năng gửi/nhận: **từ chối toàn bộ, không tự giảm**. Nhấn Chuyển hợp lệ thực hiện **ngay, không cần xác nhận lần hai**.
- Cho ăn một chim: lấy sâu từ **túi trước rồi kho**, tự chọn tối đa đủ thức ăn còn thiếu, không confirm; `Cho tất cả` chia cho chim non theo **FIFO thời gian mua**, bắt buộc confirm tổng số chim/sâu.
- Chim non/chim lớn bán lại NPC theo giá riêng do Admin cấu hình, giá không phụ thuộc vào tuổi chim non đã nuôi hoặc thời gian còn lại chim lớn; mọi lệnh bán chim phải confirm rõ tuổi/giá. Bán xong xóa chim, giải phóng slot bird.

### CSDL đề xuất riêng khi triển khai

`pet_birds(id,character_id,bought_at,feed_requirement_snapshot,feed_consumed,adult_at,adult_expires_at,state)`; `worm_feed_stores(character_id,quantity,cap_config_version)`; `bird_pest_catches(bird_id,incident_id,caught_at)` unique per incident; lock bird/window; 60m sliding counter bằng times of catches indexed.

**Chưa có trong MVP:** không thêm item pet bird, feeding UI hoặc kho sâu vào gameplay release v0.x. Item sâu bắt thủ công có thể vẫn có trong túi của MVP để chuẩn bị mở rộng sau.

## 2. Nạp tiền QR ngân hàng [CHỐT P0-040–057, 140, SAU MVP]

- Tỷ giá cơ bản 1.000 VND / 1 Kim Cương; gói cơ bản 10k, 20k, 50k, 100k, 200k, 500k VND, tương ứng 10/20/50/100/200/500 KC **chưa tính thưởng**; Admin cấu hình thưởng gói/khuyến mãi qua bảo trì.
- Ban đầu chỉ mở QR chuyển khoản ngân hàng, sau có thể thêm phương thức Admin bật/tắt. Mỗi đơn unique QR / mã order; chưa thanh toán hết hạn 15 phút.
- Webhook xác thực nguồn/ngân hàng, idempotency; tiền thanh toán trong 15m nhưng webhook muộn vẫn hợp lệ. Thanh toán sau expiry hoặc sau khi hủy chuyển Admin review theo điều kiện P0-050/054.
- Đóng popup/logout không hủy đơn; nhiều đơn chưa trả cùng lúc được phép; hủy đơn chưa thanh toán không cần confirm; sai số tiền/nội dung chuyển Admin đối soát.
- Admin xử lý case-by-case khớp đơn, hoàn/bổ sung, chargeback; bản đối soát sau restore không cấp trùng và kiểm tra asset đã dùng.
- **Không được tự triển khai tích hợp ngân hàng hay thu tiền thật chỉ vì đã có business spec.** Phải có hợp đồng nhà cung cấp, webhook signature, pháp lý, bảo mật tài chính, backup/HA, kiểm thử rollback, production approval.

## 3. Chợ người chơi [CHỐT core, SAU MVP]

- Vật phẩm được đăng bán trên chợ theo thiết kế chỉ áp dụng **item vĩnh viễn** và đủ điều kiện giao dịch; vật phẩm khóa không được bán, vật phẩm không khóa có thể giao dịch nếu loại item và kênh cho phép.
- Nông sản và hạt giống **không thể P2P** và không thuộc hai loại khóa/không khóa.
- Danh sách chợ theo thiết kế cũ ưu tiên **chỉ hiện item, giá, tên người bán**; có đường mở Profile để chat riêng/kết bạn.
- Phí, thời hạn niêm yết, hủy tin, phân trang/escrow nhiều phần trong nguồn cũ chưa đủ dữ liệu; **[CẦN ĐỐI CHIẾU]** trước khi code.
- Kiến trúc đề xuất: listing có state, item held by escrow, trade atomic, logs; không làm trước M3 ledger và inventory.

## 4. Bang hội, BXH, mini-game

- GDD nền có bang, nhiệm vụ bang, phân loại item bang khóa/không khóa, quỹ bang và mở khóa vật phẩm bang; nhiệm vụ bang tự nhận không bắt buộc, tối đa 5/ngày, từ danh sách 10 nhiệm vụ ngẫu nhiên riêng; có nhiệm vụ social như chat/emoji; tích lũy quỹ theo thời gian cho tới khi rời bang.
- **Không áp quy tắc 10 nhiệm vụ random/5 nhận của bang lên nhiệm vụ cá nhân hằng ngày MVP.**
- BXH/danh hiệu, câu cá, hoạt động/mini-game thuộc baseline 17 phân hệ, nhưng chưa có quyết định mới ràng buộc release timeline.
- Trước khi triển khai module hậu MVP, AI phải đọc `reference/GDD_BASELINE_17_MODULES.md` và xác minh các quy tắc gốc chưa rõ.
