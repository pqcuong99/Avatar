# PROJECT THANH — KIỂM KÊ TIẾN ĐỘ THIẾT KẾ GAME AVATAR

**Ngày kiểm kê:** 18/10/2026 (Việt Nam)  
**Điểm dừng chuỗi hỏi:** Câu 1198 (đáp án A).  
**Trạng thái:** Ngừng đặt câu hỏi chi tiết, chuyển sang rà soát hệ thống và lập kế hoạch triển khai.  
**Đây là báo cáo kiểm kê thiết kế, KHÔNG phải xác nhận đã lập trình, đã thử nghiệm hoặc sẵn sàng phát hành.**

## 1. Phạm vi và độ tin cậy

Nguồn đối chiếu:
- Tài liệu `thanh_game_avatar_ban_giao_session_C798(1).md`, ghi nhận thiết kế đến câu 797; chứa nhật ký chi tiết câu 661–797 và tóm tắt các quyết định trước câu 661.
- Lịch sử hội thoại đang có trong Project Thanh, đặc biệt quy tắc hệ thống bạn bè, Chat riêng và chặn Chat riêng từ khoảng câu 960 đến câu 1198.
- Các tóm tắt phiên trước và phần quyết định trò chuyện được cung cấp trong dự án.

**Giới hạn nguồn:** Chưa có bản trích xuất nguyên văn, liên tục của toàn bộ câu 1–1198. Một số đoạn, đặc biệt câu 1–660 và khoảng 798–959, cần đối chiếu lại khi lập GDD chính thức. Không coi "chưa xác minh" là "người dùng chưa từng quyết định". Mốc 1198 là số thứ tự, không có nghĩa có 1198 chức năng độc lập hoặc 1198 quyết định còn hiệu lực (một số đã bị ghi đè/hủy).

**Thang trạng thái:**
- **Đã chốt sâu**: nhiều quy tắc hành vi sản phẩm đã được quyết định; vẫn cần đặc tả kỹ thuật và thử nghiệm.
- **Đã chốt nhiều**: có lõi khá rõ; cần đối chiếu, nối các luồng, hoặc kiểm kê thêm.
- **Một phần**: đã có các quyết định, chưa thể xác minh thiết kế xuyên suốt.
- **Chưa xác minh**: không đủ tài liệu để kết luận hệ thống đã được đặc tả.

## 2. Ma trận tình trạng hệ thống

| # | Phân hệ | Trạng thái | Quyết định xác nhận được | Việc cần bổ sung/đối chiếu |
|---|---|---|---|---|
| 1 | Nông trại, cây trồng, Auto Farm | Đã chốt sâu | Trồng, nhổ, thu hoạch; thao tác hàng loạt; cây tăng trưởng qua bảo trì; hạt giống không giao dịch | Bảng dữ liệu hạt/cây, thời gian, sản lượng, giới hạn và cách xử lý đồng thời |
| 2 | Túi đồ, stack, trang phục, chờ nhận | Đã chốt sâu | Túi 10 ô đầu, +5/lần; 999/stack đối với vật phẩm farm/nâng cấp; thời trang từng món; danh sách chờ nhận không làm mất vật phẩm | Định danh item, dung lượng lưu trữ, chính sách an toàn/khả năng phục hồi, kiểm thử túi đầy |
| 3 | NPC, mua bán, hội thoại, Map Editor | Đã chốt sâu | Menu trái/nội dung phải; NPC không đứng chồng; đứng cố định; va chạm, tự tìm đường; NPC thu mua nông sản; template, version | Sơ đồ dữ liệu NPC, kịch bản hội thoại/nhiệm vụ, giao dịch nguyên tử và dữ liệu giá |
| 4 | Bảo trì, cập nhật NPC/giá/thông báo | Đã chốt sâu | Chỉ áp dụng sau bảo trì; thông báo trước; không mất/nhân đôi tài sản; phục hồi nếu thất bại | Versioning, staging, rollback, audit log, cách đối soát giao dịch sát giờ bảo trì |
| 5 | Chat riêng | Đã chốt sâu | Lịch sử từng tin 30 ngày; phân trang 50 đầu/20 thêm; đọc/chưa đọc theo viewport; typing indicator; seen; giữ nháp trong phiên | Đặc tả sự kiện/server, idempotency, kiểm thử mất mạng, đồng bộ nhiều thiết bị |
| 6 | Chặn và bỏ chặn Chat riêng | Đã chốt sâu | Chặn một chiều, không thông báo đối phương; 4 đường bỏ chặn; danh sách không giới hạn, tìm kiếm theo tên/ID; tải 50+20; lỗi/reconnect | Cursor pagination, chỉ mục tìm kiếm, quyền riêng tư, kiểm thử các tình huống đồng thời |
| 7 | Bạn bè, Profile, trạng thái nhân vật | Đã chốt nhiều | Bạn bè tối đa 100; yêu cầu kết bạn và interaction; Profile; Online ở Chat cập nhật trực tiếp | Đối chiếu trọn vẹn luồng gửi/hủy/từ chối/yêu cầu trùng, kiểm thử xã hội |
| 8 | Chợ và giao dịch người chơi | Một phần | Chợ bán thời trang vĩnh viễn không khóa; mỗi tin một món; tiền Xu; không bán hạt/nông sản | Quy trình niêm yết, giữ hàng, mua, hủy, phí, hết hạn, chống trùng và giao dịch đảo ngược (nếu có) |
| 9 | Bang hội, nhiệm vụ bang | Một phần | 10 nhiệm vụ ngẫu nhiên cá nhân/ngày; tối đa 5 nhiệm vụ tự nhận/ngày; đóng góp và quỹ bang, mở khóa item | Đối chiếu mô hình bang, phân quyền, thành viên, bang chiến/sự kiện nếu có; bảng nhiệm vụ và thưởng |
| 10 | Danh hiệu, thành tích, BXH | Đã chốt nhiều | BXH Xu/Kim Cương theo số dư; top 50; chu kỳ làm mới; danh hiệu tháng; thành tích câu cá | Đặc tả job tháng, phần thưởng/event và quản trị thời gian, kiểm thử thưởng trùng |
| 11 | Map, khu vực, điều hướng | Một phần | NPC collision; nhân vật tìm đường; tương tác nhân vật và Profile; một số giới hạn khu vực | Danh mục map, quy mô, chuyển khu, spawn, quy tắc riêng cho PC/mobile, tải tài nguyên |
| 12 | Xu, Kim Cương, kinh tế | Một phần | Ví Xu có giới hạn, số dư; quy tắc tràn Xu; thanh toán NPC và chợ | Toàn bộ nguồn/sink Xu-Kim Cương, giá cây/đồ/nâng cấp, lạm phát, mô phỏng tiến độ |
| 13 | Câu cá, mini-game, sự kiện | Một phần | Thành tích câu cá; mini-game bị hủy do bảo trì thì hoàn 100% phí | Danh mục trò chơi, điều kiện tham gia, tần suất, vòng đời phần thưởng, quy tắc chống lạm dụng |
| 14 | Tài khoản, lệnh cấm, quản trị vi phạm | Cần rà soát | Có lựa chọn về Admin và lý do cấm được nhập; các quy tắc quyền riêng tư của Chat | Đối chiếu đăng ký/đăng nhập, khôi phục, phân quyền Admin, khiếu nại và lịch sử xử lý |
| 15 | Giao diện tổng thể & tài nguyên | Chưa xác minh | Một số hành vi UI PC/mobile chi tiết đã chốt cho chat, NPC, farm | Bộ màn hình hoàn chỉnh, UI kit, asset nhân vật/NPC/map, animation, âm thanh, hỗ trợ thiết bị |
| 16 | Backend, database, bảo mật, chống gian lận | Chưa xác minh | Nhiều quy tắc yêu cầu máy chủ là nguồn dữ liệu chính thức | Kiến trúc, schema, consistency, logs, backup, API, anti-cheat, monitoring |
| 17 | QA, vận hành, phát hành | Chưa xác minh | Có chính sách bảo trì, rollback và một phần audit | Kế hoạch test, tải đồng thời, crash recovery, dữ liệu thử nghiệm, lộ trình phát hành |

## 3. Những quyết định CẤM làm sai khi chuyển sang lập trình

1. **Thay đổi NPC, giá cả và công bố thông báo cập nhật đều chỉ có hiệu lực SAU bảo trì.** Mọi quy tắc từng nói đổi trực tiếp phải nhường quyết định mới.
2. **NPC tuyệt đối không đứng chồng lên nhau**; người chơi không thể đi xuyên NPC.
3. **NPC đa chức năng: menu bên trái, nội dung bên phải**, cả trên điện thoại; chuyển menu trực tiếp, không có bước quay về menu cũ.
4. **Hạt giống và nông sản không giao dịch người-chơi với người-chơi, không phân loại khóa/không khóa**; nông sản được bán cho NPC.
5. **Túi đầy không làm mất các vật phẩm/phần thưởng chưa nhận**; giữ chờ cho đến khi người chơi đủ chỗ.
6. **Chat riêng: tin nhắn hết hạn sau 30 ngày kể từ thời điểm gửi thành công.** Hành vi đọc phụ thuộc viewport và các quy tắc cuộn chi tiết.
7. **Chặn Chat riêng là một chiều và kín đáo**; bị chặn gửi tin sẽ thấy thông báo chung, không tiết lộ danh tính người chặn.
8. **Bỏ chặn không tự gửi tin, không tự kết bạn và không khôi phục tin đã quá 30 ngày.**
9. **Danh sách chặn không giới hạn số lượng**, sắp mới chặn trước, có tìm kiếm theo tên không phân biệt hoa/thường/dấu hoặc ID đầy đủ; tải 50 đầu, 20 mỗi lần thêm.
10. **Các yêu cầu làm thay đổi trạng thái/tài sản phải được máy chủ xác nhận**, không có giả thành công khi mạng lỗi; không phát sinh giao dịch trùng.
11. **Giới hạn ví Xu có quy tắc riêng về Xu vượt trần:** hoạt động thông thường có thể làm phần vượt trần bị tiêu hủy vĩnh viễn, không cảnh báo; phải triển khai theo quyết định gốc và xác minh phạm vi ngoại lệ hoàn trả lỗi.
12. **Không đánh đồng quy tắc hiển thị dữ liệu với hành động thay đổi dữ liệu:** đóng danh sách chặn có thể hủy fetch, nhưng không hủy lệnh bỏ chặn đã được gửi lên máy chủ.

## 4. Khoảng trống quan trọng trước khi giao cho lập trình viên

### P0 — cần trước khi khóa phạm vi phiên bản đầu

- **Phạm vi MVP/V1:** màn hình, vòng gameplay và các chức năng phải có ngay khi ra bản thử đầu. Tránh đưa toàn bộ 1.198 mốc hành vi vào một đợt xây dựng.
- **Bảng cấu hình gameplay/kinh tế:** dữ liệu cây, thời gian lớn, năng suất, giá hạt, giá NPC, giá nâng cấp túi, nguồn và nơi tiêu Xu/Kim Cương, luật cấp bậc và thưởng. Tài liệu hiện có chưa đủ để xác nhận các con số.
- **Danh mục nội dung game:** map, NPC cụ thể, nhiệm vụ, cây, bộ đồ, giải thưởng, cấp bậc... dưới dạng master data dễ cập nhật có version.
- **Mô hình dữ liệu và giao dịch:** tài khoản/nhân vật; inventory; tiền tệ; hộp thư & chờ nhận; giao dịch chợ/NPC; farm; bạn bè/bang; chat; block; lịch sử/audit. Thiết kế idempotency và chống nhân đôi vật phẩm.
- **Đặc tả client/server:** trò chơi chạy trên nền tảng nào, điều khiển PC/mobile, mạng thời gian thực, sync khi reconnect, định nghĩa trạng thái Online.
- **Công cụ Admin và bảo trì:** vai trò/quyền, bản nháp, phê duyệt/công bố, kiểm tra, chạy migration, rollback, lịch sử; bảo đảm giá không đổi khi người chơi đang giao dịch.

### P1 — cần trước beta công khai

- Hoàn chỉnh các luồng giao dịch trên chợ, vòng đời bang, xử lý bạn bè/yêu cầu trùng và quản trị vi phạm.
- Hoàn chỉnh UX chính (menu, bản đồ, hiệu ứng, animation, giao diện điện thoại), test thử người chơi mới.
- Cơ chế kiểm toán và khôi phục khi phát thưởng/bán hàng/trừ tiền gặp lỗi; rà soát UX Xu vượt trần.
- Hiệu năng: stress test map, chat, tìm kiếm, farm, shop NPC, bảng xếp hạng, fan-out sự kiện và dữ liệu hàng nghìn người.

### P2 — có thể chuẩn bị sau khi MVP ổn định

- Nội dung mở rộng, mini-game/sự kiện bổ sung, các lựa chọn mỹ thuật thứ cấp, các hiệu ứng trang trí và tinh chỉnh quản trị không thiết yếu.

## 5. Rủi ro đáng kiểm tra kỹ (không tự ý sửa quyết định)

| Rủi ro | Tác động | Hướng xử lý tiếp |
|---|---|---|
| Xu vượt trần bị tiêu hủy không cảnh báo; bán nông sản vẫn mất nông sản | Có thể gây khiếu nại hoặc mất tài sản do người chơi không biết | Đưa thành test case bắt buộc; nếu muốn đổi quy tắc phải xin quyết định riêng |
| Danh sách chờ nhận không hạn thời gian | Dữ liệu/phần thưởng tồn tại lâu, cần tránh nhân đôi hoặc lạm dụng | Schema, job xử lý, audit và kiểm thử khôi phục |
| Bảo trì là cổng duy nhất để áp dụng các thay đổi kinh tế/NPC/bảng tin | Rủi ro công bố sai hoặc rollback không nhất quán | Staging/versioning, checklist, transaction boundary, rollback |
| Đồng thời thao tác farm, giao dịch NPC, chợ, nhận thưởng | Dễ race condition và cộng/trừ tài sản sai | Giao dịch nguyên tử, transaction IDs, kiểm thử concurrent |
| Chat, block, online, unread, expired messages, kết nối lại | Có thể lộ thông tin, trùng tin hoặc hiển thị sai | Mô hình server-authoritative, privacy cases, pagination và reconnect tests |
| Nhiều quyết định E Admin tùy chỉnh | Phình phạm vi Admin và độ phức tạp bản đầu | Phân loại tùy chỉnh bắt buộc lúc ra mắt và các công cụ nâng cao để sau |

## 6. Tiêu chuẩn để chuyển một phân hệ sang "sẵn sàng lập trình"

Một phân hệ chỉ gắn trạng thái **Ready for Dev** khi có: (1) hành vi người chơi đã chốt; (2) luồng màn hình/sự kiện; (3) input/output và lỗi; (4) dữ liệu, giới hạn, quyền kiểm tra phía máy chủ; (5) tiêu chí nghiệm thu và test cases; (6) sự phụ thuộc với phân hệ khác; (7) mọi mâu thuẫn quy tắc đã đối chiếu và giải quyết. Không tự động gắn Ready for Dev chỉ vì đã hỏi rất nhiều câu.

## 7. Lộ trình thực hiện đề xuất — KHÔNG tiếp tục hỏi từng nút bấm

**Giai đoạn 1 — Chuẩn hóa bản GDD:** Chuyển các câu trả lời thành tài liệu theo hệ thống, có điều khoản, ID quy tắc và ghi đè. Đối chiếu đoạn lịch sử còn thiếu nếu cần.  
**Giai đoạn 2 — Chốt phạm vi bản thử đầu:** Định nghĩa một vòng trải nghiệm từ tạo nhân vật → vào map/farm → thu hoạch → bán NPC → dùng Xu → gặp người chơi khác. Những phần chưa cần có trong vòng này để triển khai sau. Đây là **đề xuất phạm vi**, chưa phải quyết định đã chốt của người dùng.  
**Giai đoạn 3 — Đặc tả kỹ thuật:** Cấu trúc dữ liệu, API/sự kiện, giao dịch an toàn, map, UI, admin deployment.  
**Giai đoạn 4 — Prototype và thử nghiệm:** Có phiên bản chạy được, test kinh tế/tài sản/chat/mất mạng/khả năng mở rộng trước khi đưa thêm nội dung.

**Kết luận:** Thiết kế hành vi đã khá dày, nhưng chưa có cơ sở trung thực để kết luận game hoàn thành bao nhiêu phần trăm hoặc còn chính xác bao nhiêu câu hỏi. Thay vì hỏi thêm hàng trăm câu, nên giải quyết khoảng trống theo **nhóm quyết định cấp hệ thống** và ưu tiên dựng một phiên bản chơi được. Bản kiểm kê này là ảnh chụp hiện trạng, không tự sửa bất kỳ lựa chọn nào đã chốt.
