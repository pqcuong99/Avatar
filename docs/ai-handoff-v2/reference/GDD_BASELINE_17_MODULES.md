# PROJECT THANH

## GAME DESIGN DOCUMENT (GDD) — BẢN CHUẨN HÓA v1.0

**Tên dự án:** Project Thanh — Game Avatar  
**Loại tài liệu:** Tài liệu thiết kế trò chơi (Game Design Document), bản gốc chuẩn hóa từ các quyết định hiện có  
**Ngày phát hành bản tài liệu:** 19/10/2026 (Việt Nam)  
**Điểm dừng khảo sát yêu cầu:** Câu 1198 (đáp án A)  
**Trạng thái tài liệu:** Baseline để quản lý thiết kế — chưa phê duyệt phạm vi MVP, chưa Ready for Dev, chưa có bằng chứng lập trình hoặc QA  
**Đơn vị quy tắc:** Mỗi mã `TH-XX-NNN` là một quy tắc hoặc một nhóm hành vi liên quan, **không** tương ứng 1:1 với câu hỏi trong lịch sử.

> **Nguyên tắc bảo toàn:** Tài liệu không tự sửa lựa chọn đã chốt; khi nguồn thiếu, ghi rõ **CẦN ĐỐI CHIẾU**. Các nội dung **ĐỀ XUẤT KỸ THUẬT** hoặc **ĐỀ XUẤT MVP** chỉ là phương án triển khai, không được hiểu là quyết định sản phẩm của chủ dự án.

---

# PHẦN A — QUẢN TRỊ TÀI LIỆU VÀ THIẾT KẾ TỔNG THỂ

## A1. Mục tiêu sản phẩm và định hướng

Project Thanh là dự án game Avatar có yếu tố nhân vật, bản đồ nhiều người chơi, nông trại, NPC, kinh tế Xu/Kim Cương, trang phục, tương tác xã hội, chat, bang hội, bảng xếp hạng và hoạt động/mini-game. Đây là **phạm vi các hệ thống đã được nhắc đến**, không khẳng định đầy đủ thể loại, thị trường, phong cách mỹ thuật, nền tảng hoặc định vị sản phẩm.

**Vòng trải nghiệm minh họa để đánh giá bản thử đầu (CHƯA PHÊ DUYỆT):** vào game → nhân vật xuất hiện trên map → đến khu nông trại → trồng/thu hoạch → bán nông sản cho NPC → nhận/chi tiêu Xu → gặp và tương tác với người chơi khác. Vòng này là đề xuất từ báo cáo kiểm kê, không phải lựa chọn cuối cùng.

**Ngoài phạm vi cam kết của GDD v1.0:** mã nguồn, database schema hoàn chỉnh, bộ thông số kinh tế, asset, UI kit, sản phẩm đã chạy, server đã triển khai, tỷ lệ hoàn thành lập trình.

## A2. Danh mục và xếp hạng nguồn

| Mã nguồn | Nguồn hiện truy cập | Mức sử dụng |
|---|---|---|
| `SRC-01` | `Project_Thanh_Kiem_ke_thiet_ke_game_1198(1).md`, ngày kiểm kê 18/10/2026 | Có nội dung tệp, nguồn chính của baseline |
| `SRC-02` | Trích đoạn lịch sử hội thoại Project Thanh được cung cấp trong ngữ cảnh làm việc, gồm các phiên 07–13/10/2026 | Chỉ dùng các phát biểu tự do có nghĩa rõ ràng; các chữ A/B/C/D/E đơn lẻ không đủ ngữ cảnh để suy ra tính năng |
| `SRC-03` | `thanh_game_avatar_ban_giao_session_C798(1).md`, được `SRC-01` dẫn chiếu | **Chưa có bản gốc để đọc trực tiếp**, không chứng nhận nội dung chi tiết |
| `SRC-04` | Bộ câu hỏi/đáp án nguyên văn liên tục 1–1198 | **Chưa có trong nguồn truy cập**, đặc biệt cần đối chiếu câu 1–660 và 798–959 |

**Quy tắc ưu tiên nguồn:** Khi quyết định mới hơn có nội dung rõ và xung đột quyết định cũ, lấy quyết định mới; ghi lại quyết định cũ là “đã bị thay thế”. Chỉ dùng nguyên tắc này khi có bằng chứng phiên bản/thời điểm; nếu không xác định được, đánh dấu xung đột chứ không tự chọn.

## A3. Quy ước trạng thái

- **CHỐT (`C`)**: quy tắc có xác nhận trong `SRC-01` hoặc nội dung tự do rõ ràng từ `SRC-02`; vẫn có thể thiếu cách triển khai.
- **CHỐT MỘT PHẦN (`P`)**: có lõi hành vi đã xác nhận nhưng luồng/giá trị/chức năng mở rộng còn thiếu.
- **CẦN ĐỐI CHIẾU (`V`)**: có dấu vết yêu cầu nhưng nguồn chưa đủ để kết luận hoặc thiếu bản gốc.
- **ĐỀ XUẤT (`D`)**: ý kiến kiến trúc, kiểm thử hoặc phạm vi mới của bản GDD; chưa được phê duyệt.
- **BỊ THAY THẾ (`S`)**: quy tắc cũ đã bị quyết định mới phủ nhận (chỉ ghi nhận khi có bằng chứng rõ).

**Mức độ hoàn chỉnh phân hệ** (khác với trạng thái từng quy tắc): `Đã chốt sâu`, `Đã chốt nhiều`, `Một phần`, `Cần rà soát`, `Chưa xác minh`. **Ready for Dev** là tiêu chuẩn riêng và chưa được gắn cho phân hệ nào ở bản này.

## A4. Ma trận 17 phân hệ

| ID | Phân hệ | Kiểm kê hiện có | Sẵn sàng lập trình? |
|---|---|---|---|
| TH-01 | Nông trại, cây trồng, Auto Farm | Đã chốt sâu | Chưa |
| TH-02 | Túi đồ, stack, trang phục, chờ nhận | Đã chốt sâu | Chưa |
| TH-03 | NPC, mua bán, hội thoại, Map Editor | Đã chốt sâu | Chưa |
| TH-04 | Bảo trì, cập nhật NPC/giá/thông báo | Đã chốt sâu | Chưa |
| TH-05 | Chat riêng | Đã chốt sâu | Chưa |
| TH-06 | Chặn và bỏ chặn Chat riêng | Đã chốt sâu | Chưa |
| TH-07 | Bạn bè, Profile, trạng thái nhân vật | Đã chốt nhiều | Chưa |
| TH-08 | Chợ và giao dịch người chơi | Một phần | Chưa |
| TH-09 | Bang hội, nhiệm vụ bang | Một phần | Chưa |
| TH-10 | Danh hiệu, thành tích, bảng xếp hạng | Đã chốt nhiều | Chưa |
| TH-11 | Map, khu vực, điều hướng | Một phần | Chưa |
| TH-12 | Xu, Kim Cương, kinh tế | Một phần | Chưa |
| TH-13 | Câu cá, mini-game, sự kiện | Một phần | Chưa |
| TH-14 | Tài khoản, lệnh cấm, quản trị vi phạm | Cần rà soát | Chưa |
| TH-15 | Giao diện tổng thể và tài nguyên | Chưa xác minh | Chưa |
| TH-16 | Backend, database, bảo mật, chống gian lận | Chưa xác minh | Chưa |
| TH-17 | QA, vận hành, phát hành | Chưa xác minh | Chưa |

## A5. Quy tắc xuyên hệ thống bắt buộc giữ nguyên

1. `INV-01` [C, SRC-01] Thay đổi NPC, giá và công bố cập nhật thông báo chỉ có hiệu lực **sau bảo trì**; không triển khai kiểu thay đổi trực tiếp khi game đang vận hành.
2. `INV-02` [C, SRC-01] NPC không đứng chồng nhau; nhân vật người chơi không đi xuyên NPC.
3. `INV-03` [C, SRC-01 + SRC-02] NPC nhiều chức năng có **menu bên trái, cửa sổ nội dung bên phải**, kể cả mobile; chọn mục khác thay nội dung ngay.
4. `INV-04` [C, SRC-01 + SRC-02] Hạt giống và nông sản không thể giao dịch người chơi–người chơi và **không thuộc phân loại khóa/không khóa**; nông sản bán cho NPC.
5. `INV-05` [C, SRC-01 + SRC-02] Túi đầy không được khiến item/phần thưởng chưa nhận bị mất; tiếp tục giữ ở hàng chờ nhận.
6. `INV-06` [C, SRC-01] Chat riêng: thời hạn tin nhắn **30 ngày kể từ thời điểm gửi thành công**; quy tắc đọc/chưa đọc gắn với viewport.
7. `INV-07` [C, SRC-01] Chặn Chat riêng một chiều và kín đáo; lỗi chung khi người bị chặn cố gửi, không tiết lộ ai chặn.
8. `INV-08` [C, SRC-01] Bỏ chặn không tự gửi tin, không tự kết bạn, không phục hồi tin hết hạn.
9. `INV-09` [C, SRC-01] Danh sách chặn không giới hạn; tìm theo tên không phân biệt hoa/thường/dấu hoặc ID đầy đủ; tải trang 50 đầu + 20 thêm; mới chặn trước.
10. `INV-10` [C, SRC-01] Thao tác làm thay đổi tài sản/trạng thái cần máy chủ xác nhận; không báo thành công giả khi lỗi mạng; chống phát sinh trùng.
11. `INV-11` [P, SRC-01] Ví Xu có trần; phần Xu vượt trần từ hoạt động thường **có thể bị tiêu hủy vĩnh viễn, không cảnh báo**. Phạm vi ngoại lệ hoàn trả lỗi **cần đối chiếu trước khi code**.
12. `INV-12` [C, SRC-01] Đóng giao diện danh sách chặn có thể hủy lệnh **tải dữ liệu** nhưng không được tự hủy lệnh **bỏ chặn** đã gửi đến server.

---

# PHẦN B — ĐẶC TẢ THEO 17 PHÂN HỆ

## TH-01. Nông trại, cây trồng và Auto Farm

**Mức thiết kế:** Đã chốt sâu. **Mục đích:** cung cấp vòng nông trại, thu nông sản, tương tác với túi đồ và NPC thu mua.

### Quy tắc được bảo lưu

- `TH-01-001` [C, SRC-01] Game có hành vi trồng cây, nhổ cây và thu hoạch.
- `TH-01-002` [C, SRC-01] Có thao tác hàng loạt và phạm vi Auto Farm đã được bàn tới; **chưa xác minh** chi tiết loại thao tác, điều kiện kích hoạt, giới hạn và ưu tiên xử lý.
- `TH-01-003` [C, SRC-01] Cây có quy tắc tiếp tục tăng trưởng qua đợt bảo trì; chưa xác nhận số liệu thời gian và công thức bù.
- `TH-01-004` [C, SRC-01 + SRC-02] Hạt giống không thể giao dịch giữa người chơi, không có trạng thái khóa/không khóa.
- `TH-01-005` [C, SRC-01] Nông sản không thể giao dịch giữa người chơi, không có trạng thái khóa/không khóa; có thể bán cho NPC.
- `TH-01-006` [P, SRC-01] Kết quả thu hoạch cần đi vào túi/hàng chờ theo quy tắc bảo toàn tài sản TH-02; logic chi tiết khi nhiều lệnh đồng thời chưa đặc tả.

### Dữ liệu và luồng còn thiếu

`seed_id`, `crop_id`, ô đất, trạng thái ô, thời điểm trồng, thời điểm thu hoạch, điều kiện nhổ, số lượng mỗi lần thu, bảng giá hạt, năng suất, cách xử lý bảo trì và thao tác hàng loạt. **Đây là nhóm dữ liệu cần thiết để đặc tả, không phải schema đã chốt.**

### Ca nghiệm thu tối thiểu (đề xuất kiểm thử)

- Lệnh thu hoạch lặp/đến cùng lúc không tạo gấp đôi sản lượng.
- Bảo trì không làm mất cây hoặc tăng trưởng sai theo quy tắc gốc.
- Không có chức năng chào bán/gửi hạt hoặc nông sản sang người chơi khác.
- Túi đầy phải tuân thủ cơ chế giữ chờ TH-02, không xóa tài sản.

**Phụ thuộc:** TH-02, TH-03, TH-04, TH-12, TH-16.

## TH-02. Túi đồ, stack, thời trang và chờ nhận

**Mức thiết kế:** Đã chốt sâu. **Mục đích:** quản lý vật phẩm và phòng mất/nhân đôi tài sản.

### Quy tắc được bảo lưu

- `TH-02-001` [C, SRC-01 + SRC-02] Người chơi bắt đầu với **10 ô túi**.
- `TH-02-002` [C, SRC-01 + SRC-02] Mỗi lần nâng cấp túi **tăng thêm 5 ô**. Chi phí, trần số ô và điều kiện nâng cấp chưa được xác minh.
- `TH-02-003` [C, SRC-01 + SRC-02] Item farm và item nâng cấp cùng loại: **tối đa 999 vật phẩm trên một ô**.
- `TH-02-004` [C, SRC-01 + SRC-02] Mỗi item thời trang là **một sản phẩm/một đơn vị riêng**, không gom theo stack 999.
- `TH-02-005` [C, SRC-01 + SRC-02] Túi đầy không làm mất item/phần thưởng chưa nhận; vật phẩm ở trạng thái chờ và nhận sau khi đủ chỗ.
- `TH-02-006` [P, SRC-02] Có khái niệm vật phẩm **khóa/không khóa**, item không khóa có thể giao dịch theo quy tắc của kênh giao dịch tương ứng; hạt giống và nông sản là ngoại lệ không tham gia phân loại này.
- `TH-02-007` [V, SRC-01] Thời hạn, danh sách, thứ tự và cơ chế lưu/nhận phần thưởng chờ cần đối chiếu; báo cáo kiểm kê đánh dấu rủi ro khi giữ chờ lâu, **không đủ cơ sở để suy ra mọi hàng chờ đều vô thời hạn**.

### Chưa xác minh

Định danh item, biến thể, item tồn tại vĩnh viễn/hạn sử dụng, quy tắc stack khi vượt 999, phân tách/gộp stack, sức chứa tối đa, giá nâng cấp, trang bị và phục hồi lỗi. Không mặc định item farm/nâng cấp đều được giao dịch.

### Ca nghiệm thu tối thiểu

- Với túi 10 ô: kiểm đúng số ô ban đầu; sau một lần nâng cấp là 15 ô.
- Item stack 999 không tự thành stack 1000; item thời trang không gộp hai món thành một.
- Khi túi đầy, giao thưởng/thu hoạch không mất và không cấp hai lần.
- Sau mất mạng hoặc đăng nhập lại, item chờ vẫn có thể tiếp nhận nếu đủ điều kiện.

**Phụ thuộc:** TH-01, TH-08, TH-09, TH-12, TH-16.

## TH-03. NPC, cửa hàng, hội thoại và Map Editor

**Mức thiết kế:** Đã chốt sâu. **Mục đích:** NPC tương tác nhiều chức năng, NPC thu mua và các cấu hình trên bản đồ.

### Quy tắc được bảo lưu

- `TH-03-001` [C, SRC-01 + SRC-02] NPC đa chức năng hiển thị **menu bên trái, nội dung bán/hội thoại tương ứng bên phải**.
- `TH-03-002` [C, SRC-01 + SRC-02] Nhấp mục ở menu trái sẽ đổi nội dung bên phải ngay, **không quay lại màn hình menu trước**, kể cả mobile.
- `TH-03-003` [C, SRC-01 + SRC-02] NPC **không đứng chồng lên nhau**.
- `TH-03-004` [C, SRC-01] NPC có vị trí đứng cố định; người chơi không đi xuyên NPC và cần điều hướng tìm đường phù hợp khi gặp vật cản NPC.
- `TH-03-005` [C, SRC-01] Có NPC thu mua nông sản; giao dịch giá phải tuân thủ lịch bảo trì TH-04.
- `TH-03-006` [P, SRC-01] Có Map Editor, template và version cho NPC; quyền sửa, trường dữ liệu và cách xuất bản chưa xác minh.
- `TH-03-007` [C, SRC-01 + SRC-02] Mọi điều chỉnh NPC hoặc giá thuộc cơ chế cập nhật thông qua bảo trì, không sửa trực tiếp giữa phiên giao dịch của người chơi.

### Chưa xác minh

Danh sách NPC cụ thể, tọa độ, hướng quay, kích thước collider, các nhánh hội thoại, menu dịch vụ, hàng mua/bán, công thức giá, tương tác với nhiều người cùng lúc, lỗi giao dịch, quy tắc NPC/map từng phiên bản.

### Ca nghiệm thu tối thiểu

- Hai NPC không thể xếp trùng vị trí hợp lệ trên bản đồ vận hành.
- Người chơi không thể đi xuyên collider NPC; tìm đường không làm kẹt nhân vật vô hạn.
- Đổi tab menu NPC giữ đúng phiên giao diện; giá đang giao dịch không đổi giữa chừng do cập nhật Admin.
- Giao dịch mua/bán cần xác nhận phía server, không trừ hoặc nhân đôi item/Xu khi retry.

**Phụ thuộc:** TH-01, TH-02, TH-04, TH-11, TH-12, TH-16.

## TH-04. Bảo trì, cấu hình NPC/giá và thông báo

**Mức thiết kế:** Đã chốt sâu. **Mục đích:** đảm bảo cập nhật nhất quán và tránh tranh chấp tài sản khi phiên bản thay đổi.

### Quy tắc được bảo lưu

- `TH-04-001` [C, SRC-01 + SRC-02] **Mọi thay đổi NPC và giá cả chỉ áp dụng sau bảo trì**.
- `TH-04-002` [C, SRC-01 + SRC-02] **Thông báo cập nhật** cũng công bố sau bảo trì; không có hành vi đẩy nội dung mới vào bản tin khi người chơi đang mở.
- `TH-04-003` [C, SRC-01 + SRC-02] Trước khi bảo trì phải thông báo; nội dung, kênh và khoảng báo trước chưa được xác minh.
- `TH-04-004` [C, SRC-01] Cập nhật hoặc thất bại không được làm mất/nhân đôi tài sản; cần cơ chế phục hồi.
- `TH-04-005` [P, SRC-02] Theo quyết định trong lịch sử, giao dịch người chơi vẫn cố thực hiện bất chấp thông báo bảo trì **không mặc nhiên được hoàn**. Ranh giới giao dịch nào thành công/thất bại vẫn phải đối chiếu quyết định gốc và bảo đảm không vi phạm bảo toàn tài sản.
- `TH-04-006` [S, SRC-01] Mọi yêu cầu cũ cho phép chỉnh giá/NPC/thông báo **trực tiếp trong lúc đang vận hành** bị thay bằng TH-04-001/002.

### Phải đặc tả trước triển khai

Kịch bản thông báo và đóng dịch vụ, trạng thái đang giao dịch, snapshot bảng giá, version nội dung, quy trình staging/duyệt/publish, rollback, audit, thời gian hiệu lực và đối soát giao dịch giáp giờ bảo trì.

### Ca nghiệm thu tối thiểu

- Bảng giá đang dùng giữ nguyên cho đến lần triển khai bảo trì thích hợp.
- Khi người chơi đang đọc bản tin, nội dung không bị thay giữa phiên vì Admin soạn bản mới.
- Bảo trì bị lỗi rollback không gây mất/đúp Xu hay item.

**Phụ thuộc:** TH-03, TH-12, TH-14, TH-16, TH-17.

## TH-05. Chat riêng

**Mức thiết kế:** Đã chốt sâu. **Mục đích:** nhắn tin riêng, đồng bộ trạng thái đọc và dữ liệu lịch sử có thời hạn.

### Quy tắc được bảo lưu

- `TH-05-001` [C, SRC-01] Tin nhắn riêng hết hạn **sau 30 ngày tính từ lúc gửi thành công**, theo từng tin riêng biệt.
- `TH-05-002` [C, SRC-01] Tải phần lịch sử đầu tiên **50 tin**, cuộn gần đầu phần đã tải sẽ lấy thêm **20 tin cũ**; giữ nguyên vị trí cuộn.
- `TH-05-003` [C, SRC-01] Trạng thái đọc/chưa đọc gắn với phần hội thoại thực sự hiển thị trong **viewport**; không suy đoán là đã đọc toàn bộ chỉ vì mở hội thoại.
- `TH-05-004` [C, SRC-01] Có tín hiệu đang nhập (`typing indicator`) và đã xem (`seen`).
- `TH-05-005` [C, SRC-01] Nội dung nháp Chat riêng được giữ **trong phiên**; chưa xác minh tính bền vững qua đăng xuất hoặc đổi thiết bị.
- `TH-05-006` [P, SRC-02] Người chơi có thể chọn người chơi khác và chọn **Chat riêng** để bắt đầu tương tác. Hành vi mở đúng cửa sổ/đổi người nhận cần đối chiếu thêm bối cảnh, không tự dựng nhiều cửa sổ.
- `TH-05-007` [P, SRC-01] Trạng thái Online liên quan Chat cập nhật trực tiếp; định nghĩa Online và xử lý mất kết nối chưa xác minh.
- `TH-05-008` [C, SRC-01] Mọi kết quả gửi liên quan trạng thái server; không được giả thành công khi mạng lỗi.

### Chưa xác minh

Định dạng tin nhắn, giới hạn độ dài, file/emoji, chính sách thu hồi/xóa, đọc đồng bộ nhiều thiết bị, thời gian typing, xử lý trùng gửi, cursor phân trang, thông báo khi offline và logic cuộn đến cuối khi tin mới đến. Không khẳng định đặc tả chi tiết viewport chỉ từ tóm tắt.

### Ca nghiệm thu tối thiểu

- Tin A và B gửi khác thời điểm hết hạn độc lập sau 30 ngày.
- Cuộn gần đầu nạp thêm 20 tin mà không nhảy vị trí xem.
- Tin chưa xuất hiện trên vùng nhìn thấy không tự nhận đã đọc.
- Mạng ngắt rồi gửi lại không sinh hai tin do cùng một yêu cầu.
- Tương tác với người bị chặn tuân thủ TH-06 mà không lộ người chặn.

**Phụ thuộc:** TH-06, TH-07, TH-14, TH-16.

## TH-06. Chặn và bỏ chặn Chat riêng

**Mức thiết kế:** Đã chốt sâu. **Mục đích:** kiểm soát Chat riêng và quyền riêng tư của người chơi.

### Quy tắc được bảo lưu

- `TH-06-001` [C, SRC-01] Chặn Chat riêng là **một chiều**: chặn người khác không mặc định tạo chặn đối xứng.
- `TH-06-002` [C, SRC-01] Không gửi thông báo trực tiếp cho người bị chặn; khi họ gửi tin bị từ chối, chỉ hiển thị **thông báo chung**, không tiết lộ danh tính/nguyên nhân chặn cụ thể.
- `TH-06-003` [C, SRC-01] Có **4 đường bỏ chặn** đã được thống nhất ở thiết kế trước; **chưa đủ nguồn để liệt kê chính xác 4 đường**. Không tự dựng tên các nút/menu.
- `TH-06-004` [C, SRC-01] Bỏ chặn không tự gửi tin, không tự kết bạn, không phục hồi tin Chat riêng đã hết 30 ngày.
- `TH-06-005` [C, SRC-01] Danh sách chặn **không giới hạn số lượng**; sắp người bị chặn gần nhất lên trước.
- `TH-06-006` [C, SRC-01] Có tìm kiếm người trong danh sách chặn theo **tên không phân biệt hoa/thường và dấu** hoặc theo **ID đầy đủ**.
- `TH-06-007` [C, SRC-01] Danh sách tải **50 người đầu**, thêm **20 mỗi lần** khi phân trang.
- `TH-06-008` [C, SRC-01] Đóng danh sách có thể ngừng yêu cầu tải thêm dữ liệu, nhưng không được hiểu là hủy thao tác bỏ chặn đã gửi lên server.
- `TH-06-009` [P, SRC-01] Có xử lý lỗi và reconnect cho thao tác chặn/bỏ chặn; thông báo lỗi, retry và xung đột đồng thời chưa có đặc tả nguyên văn.

### Chưa xác minh

Vị trí chính xác của 4 đường bỏ chặn, API block/unblock, trạng thái đối phương thấy trong profile, tác động lên yêu cầu kết bạn và tin nhắn đang chờ, cách tìm tên trùng, tốc độ làm mới danh sách, đồng bộ nhiều thiết bị.

### Ca nghiệm thu tối thiểu

- Chặn A không tự chặn ngược B.
- Danh sách chặn trên 50 đối tượng tải trang đầu 50, mỗi trang thêm 20.
- Tìm tên có/không dấu trả kết quả theo cùng quy tắc tìm kiếm; ID chỉ chấp nhận ID đầy đủ.
- Bỏ chặn thành công không tự tạo tin, quan hệ bạn bè hay lịch sử hết hạn.
- Đóng cửa sổ giữa khi yêu cầu bỏ chặn đã gửi không tự vô hiệu hóa lệnh.

**Phụ thuộc:** TH-05, TH-07, TH-14, TH-16.

## TH-07. Bạn bè, Profile và trạng thái nhân vật

**Mức thiết kế:** Đã chốt nhiều. **Mục đích:** quan hệ xã hội, thao tác trên người chơi khác và hiển thị hồ sơ.

### Quy tắc được bảo lưu

- `TH-07-001` [C, SRC-01] Mỗi người chơi có giới hạn **tối đa 100 bạn bè**.
- `TH-07-002` [P, SRC-01 + SRC-02] Kết bạn thực hiện **thủ công** bằng tìm người muốn kết bạn và chọn nút kết bạn; có luồng lời mời/interaction.
- `TH-07-003` [P, SRC-01 + SRC-02] Tương tác với người chơi từ map hoặc danh sách có thể mở **Profile** và có hành động Chat riêng / Kết bạn.
- `TH-07-004` [C, SRC-01] Trạng thái Online trong Chat cập nhật trực tiếp; chưa xác minh mức độ phơi bày tại Profile/map và quy tắc ẩn trạng thái.

### Chưa xác minh

Trạng thái lời mời (đã gửi/đã nhận/hủy/từ chối/hết hạn), xung đột khi hai bên gửi đồng thời, danh sách bạn, giới hạn số lời mời, điều kiện tìm theo ID/tên, chặn với bạn bè, trường Profile và quyền riêng tư.

### Ca nghiệm thu tối thiểu

- Không thêm bạn thứ 101 khi đang có đủ 100.
- Hai lời mời cùng cặp người chơi không tạo trùng quan hệ.
- Thao tác kết bạn khi offline không hiển thị thành công giả.

**Phụ thuộc:** TH-05, TH-06, TH-11, TH-14, TH-16.

## TH-08. Chợ và giao dịch người chơi

**Mức thiết kế:** Một phần. **Mục đích:** nơi niêm yết và mua bán item giữa người chơi với Xu.

### Quy tắc được bảo lưu

- `TH-08-001` [C, SRC-01 + SRC-02] Các item được phép rao trên chợ theo quy tắc hiện ghi nhận là **thời trang vĩnh viễn, không khóa**.
- `TH-08-002` [C, SRC-01] **Mỗi tin bán tương ứng một món thời trang**; không niêm yết stack sản phẩm thời trang.
- `TH-08-003` [C, SRC-01] Giao dịch chợ sử dụng **Xu**.
- `TH-08-004` [C, SRC-01] Không cho bán hạt giống hoặc nông sản tại chợ người chơi.
- `TH-08-005` [P, SRC-02] Thẻ hiển thị tin chợ gồm **item, giá, tên người bán**; chọn người bán cho phép mở Profile và từ đó Chat riêng/Kết bạn. Các thành phần UI khác không được coi là đã chốt.
- `TH-08-006` [C, SRC-01] Mua/bán phải không mất, không nhân đôi item/Xu và không thành công giả khi lỗi mạng.

### Chưa xác minh

Quy trình niêm yết, khóa/giữ hàng khi đăng bán, giới hạn tin, giá tối thiểu/tối đa, phí, thời hạn, hủy, xử lý người bán offline, mua đồng thời một món, hoàn tiền, lịch sử mua bán, item giao dịch qua kênh khác (nếu có). **Không tự mở chợ cho item không phải thời trang** chỉ vì chúng có trạng thái không khóa.

### Ca nghiệm thu tối thiểu

- Chợ từ chối item có khóa, thời trang không vĩnh viễn, hạt và nông sản.
- Hai người cùng mua một món: tối đa một giao dịch thành công.
- Giao dịch thất bại không vừa trừ Xu vừa giữ item.

**Phụ thuộc:** TH-02, TH-07, TH-12, TH-16.

## TH-09. Bang hội và nhiệm vụ bang

**Mức thiết kế:** Một phần. **Mục đích:** hoạt động cộng đồng, nhiệm vụ tự nguyện và quỹ/tiến độ bang.

### Quy tắc được bảo lưu

- `TH-09-001` [C, SRC-01 + SRC-02] Mỗi người chơi được **một danh sách riêng gồm 10 nhiệm vụ bang ngẫu nhiên mỗi ngày**.
- `TH-09-002` [C, SRC-01 + SRC-02] Người chơi **tự nhận nhiệm vụ**, không bắt buộc; mỗi người **tối đa 5 nhiệm vụ bang/ngày**, trừ khi có thay đổi sau này.
- `TH-09-003` [P, SRC-02] Danh mục nhiệm vụ có thể đa dạng, bao gồm hành vi xã hội như chat/thả emoji; đây là định hướng được nêu trong đối thoại, **chưa có bảng nhiệm vụ cụ thể**.
- `TH-09-004` [P, SRC-02] Quy tắc “**phần thưởng của 5 nhiệm vụ bang mỗi ngày giống nhau theo từng nhiệm vụ**” phải được giữ nguyên câu chữ để đối chiếu; không tự hiểu là cả 5 nhiệm vụ luôn có giá trị thưởng bằng nhau.
- `TH-09-005` [C, SRC-01 + SRC-02] Có quỹ bang/đóng góp; khi quỹ đạt mốc có thể **mở khóa item riêng cho bang**.
- `TH-09-006` [P, SRC-02] Có chỉ số/tích lũy được giữ cho đến khi người chơi **rời bang**; cần xác minh chính xác chỉ số nào và xử lý ra/vào lại.
- `TH-09-007` [P, SRC-01] Mô hình bang, phân quyền, chức danh, sự kiện/bang chiến và chu kỳ reset chưa đủ nguồn xác minh.

### Chưa xác minh

Điều kiện lập/gia nhập/rời bang, số thành viên, quyền của bang chủ/phó, cách nhận thưởng, reset nhiệm vụ theo múi giờ, ngẫu nhiên và chống trùng, đóng góp quỹ, giá/mốc mở item, item khóa/không khóa, khôi phục khi rời bang.

### Ca nghiệm thu tối thiểu

- Mỗi thành viên nhận danh sách 10 nhiệm vụ riêng; không tự động nhận tất cả.
- Không thể nhận quá 5 nhiệm vụ trong ngày hợp lệ.
- Chuyển ngày/đổi thiết bị không làm nhận thưởng lặp.
- Góp quỹ và mở item phải xác nhận từ server và ghi lịch sử giao dịch.

**Phụ thuộc:** TH-02, TH-07, TH-12, TH-16.

## TH-10. Danh hiệu, thành tích và bảng xếp hạng

**Mức thiết kế:** Đã chốt nhiều. **Mục đích:** ghi nhận thành tích và cạnh tranh xã hội.

### Quy tắc được bảo lưu

- `TH-10-001` [C, SRC-01] Có bảng xếp hạng **Xu** và **Kim Cương** dựa trên **số dư** tương ứng.
- `TH-10-002` [C, SRC-01] Bảng xếp hạng hiển thị **Top 50**.
- `TH-10-003` [P, SRC-01] Có chu kỳ làm mới BXH và danh hiệu theo tháng; tần suất cập nhật chính xác, mốc chốt kỳ, timezone chưa được xác minh.
- `TH-10-004` [P, SRC-01] Có hệ thống danh hiệu và thành tích câu cá; tiêu chuẩn, số lượng và thưởng chưa được liệt kê từ nguồn đầy đủ.

### Chưa xác minh

Cách xử lý đồng hạng, cache/thời gian làm mới, lịch sử top, thời gian nhận danh hiệu tháng, phần thưởng, chống cấp trùng, ranking khi số dư biến động giữa lúc chốt.

### Ca nghiệm thu tối thiểu

- Đối tượng xếp hạng Xu/Kim Cương lấy số dư chứ không nhầm tổng thu nhập.
- Danh sách hiển thị tối đa 50 mục hợp lệ.
- Công việc cấp danh hiệu/chuyển tháng chạy lại không thưởng trùng.

**Phụ thuộc:** TH-12, TH-13, TH-16.

## TH-11. Map, khu vực và điều hướng

**Mức thiết kế:** Một phần. **Mục đích:** người chơi di chuyển và tương tác trong thế giới chung.

### Quy tắc được bảo lưu

- `TH-11-001` [C, SRC-01] Có cơ chế va chạm NPC và điều hướng/tự tìm đường khi nhân vật gặp NPC.
- `TH-11-002` [C, SRC-01] NPC đứng cố định và không chồng nhau; người chơi không đi xuyên NPC.
- `TH-11-003` [P, SRC-01] Người chơi có thể tương tác với nhân vật khác để mở Profile/Chat riêng/Kết bạn theo các quy tắc xã hội.
- `TH-11-004` [V, SRC-01] Có đề cập giới hạn khu vực nhưng chưa có danh mục/kịch bản đầy đủ.

### Chưa xác minh

Danh sách map, tọa độ spawn, vùng chuyển map, kích cỡ phòng/khu, số người đồng thời, chuyển kênh, chặn đường, mobile joystick/tap-to-move, PC keyboard/mouse, hiệu năng render và tải asset.

### Ca nghiệm thu tối thiểu

- Người chơi không sinh ra đè vị trí NPC hoặc vào ô/điểm không hợp lệ.
- Di chuyển tránh vật cản NPC theo thuật toán được đặc tả sau.
- Mở Profile đúng người chơi được chọn khi nhiều avatar đứng gần nhau.

**Phụ thuộc:** TH-03, TH-07, TH-15, TH-16.

## TH-12. Xu, Kim Cương và kinh tế

**Mức thiết kế:** Một phần. **Mục đích:** điều phối tài sản tiền tệ và các giao dịch.

### Quy tắc được bảo lưu

- `TH-12-001` [C, SRC-01] Có các đơn vị **Xu** và **Kim Cương**.
- `TH-12-002` [P, SRC-01] Ví Xu có giới hạn số dư; **giá trị trần cụ thể chưa được xác minh**.
- `TH-12-003` [P, SRC-01] Hoạt động thường khi vượt trần Xu có thể làm **phần vượt bị tiêu hủy vĩnh viễn và không cảnh báo**; cần phân loại chính xác từng nguồn nhận và ngoại lệ hoàn trả lỗi từ quyết định gốc trước khi lập trình.
- `TH-12-004` [C, SRC-01] Xu liên quan giao dịch NPC và chợ người chơi.
- `TH-12-005` [C, SRC-01] BXH Xu/Kim Cương xếp theo **số dư**, không theo tổng tích lũy lịch sử.
- `TH-12-006` [V, SRC-01] Nguồn cấp/tiêu Xu và Kim Cương, tỷ giá (nếu có), giá nâng cấp/tương tác và cân bằng kinh tế chưa đủ dữ liệu số.

### Rủi ro đặc biệt cần giữ khi test

Nếu nhận Xu từ bán nông sản trong tình huống tràn trần, nông sản vẫn có thể bị tiêu thụ trong khi phần Xu vượt bị hủy theo nguồn kiểm kê. Đây là nguy cơ UX, **không được tự đổi thành “từ chối bán” hoặc “trả vật phẩm”** khi chưa có quyết định mới.

### Ca nghiệm thu tối thiểu

- Tràn trần Xu được kiểm thử cho từng loại nguồn, có cả ca bán nông sản khi ví gần đầy.
- Hoàn tiền/rollback khi lỗi không bị dùng nhầm quy tắc mất phần Xu vượt trần.
- Retry không trừ/cộng Xu hai lần.

**Phụ thuộc:** TH-01, TH-02, TH-03, TH-04, TH-08, TH-09, TH-10, TH-16.

## TH-13. Câu cá, mini-game và sự kiện

**Mức thiết kế:** Một phần. **Mục đích:** bổ sung hoạt động và tiến trình thành tích.

### Quy tắc được bảo lưu

- `TH-13-001` [P, SRC-01] Có nội dung câu cá gắn với thành tích; chưa có đặc tả trọn vòng chơi.
- `TH-13-002` [C, SRC-01] **Mini-game bị hủy do bảo trì thì hoàn 100% phí**.
- `TH-13-003` [P, SRC-02] Đã đề cập chức năng “Sẽ tham gia” cho một hoạt động và **xem danh sách cụ thể ai đã chọn “Sẽ tham gia”**; loại sự kiện, quyền xem và điều kiện thời gian chưa đủ rõ.
- `TH-13-004` [V, SRC-01] Điều kiện tham gia, nhịp mở, thời hạn sự kiện, giới hạn lượt và thưởng chưa có danh mục đáng tin cậy.

### Chưa xác minh

Luồng chơi câu cá, danh sách mini-game, phí/giải thưởng, lịch sự kiện, cơ chế hủy ngoài bảo trì, chống lạm dụng, kết quả offline, thời hạn nhận thưởng, tương tác với BXH/bang.

### Ca nghiệm thu tối thiểu

- Hủy mini-game do bảo trì hoàn đúng 100% phí (không hoàn thiếu/đúp).
- Sự kiện ghi danh không hiện người không chọn “Sẽ tham gia” khi danh sách cần hiển thị (sau khi xác nhận đúng hệ thống).

**Phụ thuộc:** TH-04, TH-09, TH-10, TH-12, TH-16.

## TH-14. Tài khoản, lệnh cấm và quản trị vi phạm

**Mức thiết kế:** Cần rà soát. **Mục đích:** định danh người chơi, xử lý vi phạm và điều hành an toàn.

### Quy tắc được bảo lưu

- `TH-14-001` [P, SRC-01 + SRC-02] Có xử lý cấm tài khoản bởi Admin và **hiển thị lý do bị ban do Admin nhập**.
- `TH-14-002` [P, SRC-01] Quy tắc riêng tư Chat/bị chặn phải không làm lộ danh tính người chặn thông qua hệ thống quản trị hoặc thông báo đến đối phương.
- `TH-14-003` [V, SRC-01] Đăng ký/đăng nhập, bảo mật tài khoản, khôi phục, chính sách khiếu nại, thời hạn ban và nhật ký thao tác Admin chưa đủ nguồn.

### Chưa xác minh

Loại ban (tạm thời/vĩnh viễn), ID tài khoản/nhân vật, thiết bị đăng nhập đồng thời, xác minh, reset mật khẩu, phân quyền support/moderator/admin, đánh giá kháng nghị và lịch sử quyết định.

### Ca nghiệm thu tối thiểu

- Người bị cấm xem đúng lý do mà Admin đã nhập theo luồng đã chốt.
- Admin cấp thấp không sửa được trạng thái cấp cao khi chưa được phân quyền.
- Một thao tác ban/unban phải có dấu vết cho việc điều tra khiếu nại (đề xuất kỹ thuật).

**Phụ thuộc:** TH-05, TH-06, TH-16, TH-17.

## TH-15. Giao diện tổng thể và tài nguyên

**Mức thiết kế:** Chưa xác minh ở cấp hệ thống. **Mục đích:** xác lập trải nghiệm nhất quán PC/mobile và quy chuẩn asset.

### Quy tắc đã có dấu vết

- `TH-15-001` [C, SRC-01 + SRC-02] NPC đa chức năng dùng **menu trái, nội dung phải**, kể cả mobile.
- `TH-15-002` [C, SRC-01] Chat riêng có quy tắc viewport/cuộn, trạng thái typing/seen, nháp trong phiên.
- `TH-15-003` [P, SRC-01] Farm có hành vi tương tác hàng loạt; chưa có bộ màn hình và kiểm thử thao tác.
- `TH-15-004` [V, SRC-01] Chưa có căn cứ xác nhận bộ màn hình hoàn chỉnh, UI kit, asset nhân vật/NPC/map, animation, âm thanh, hỗ trợ thiết bị.

### Tài liệu cần hình thành (đề xuất)

Sitemap màn hình, wireframe PC/mobile, hành vi responsive, biểu đồ trạng thái cửa sổ, quy chuẩn component, danh mục asset, ma trận tương thích thiết bị, accessibility tối thiểu, fallback khi tải asset thất bại.

### Ca nghiệm thu tối thiểu

- Mobile vẫn thể hiện rõ hai vùng menu trái/nội dung phải của NPC theo thiết kế đã chốt.
- Cuộn Chat và danh sách chặn không nhảy vị trí khi nạp thêm trang.
- Không thiết kế UX khiến người chơi tưởng thao tác tài sản đã thành công trước phản hồi server.

**Phụ thuộc:** Tất cả các phân hệ có UI, đặc biệt TH-03, TH-05, TH-06, TH-11.

## TH-16. Backend, database, bảo mật và chống gian lận

**Mức thiết kế:** Chưa xác minh về giải pháp kỹ thuật. **Mục đích:** nền dữ liệu nhất quán cho tài sản, đồng bộ xã hội và vận hành.

### Ràng buộc sản phẩm bắt buộc đưa vào kỹ thuật

- `TH-16-001` [C, SRC-01] Các thao tác thay đổi tài sản/trạng thái do **máy chủ xác nhận**; không báo giả thành công khi lỗi mạng.
- `TH-16-002` [C, SRC-01] Giao dịch không mất/nhân đôi vật phẩm, Xu; chống retry trùng khi farm, giao dịch NPC, chợ, thưởng và hoàn phí.
- `TH-16-003` [C, SRC-01] Bảo trì áp bản NPC/giá/bản tin nhất quán, có khả năng phục hồi nếu đợt cập nhật thất bại.
- `TH-16-004` [C, SRC-01] Chat/Block phải giữ đúng quyền riêng tư, phân trang 50/20, tin 30 ngày và trạng thái khi reconnect.
- `TH-16-005` [V, SRC-01] Kiến trúc, ngôn ngữ, máy chủ, database, giao thức, API, backup, monitoring và anti-cheat chưa được chốt từ nguồn hiện có.

### Đề xuất kỹ thuật — chưa phê duyệt

- Gắn ID yêu cầu (idempotency key) cho nghiệp vụ cộng/trừ tiền, item và phần thưởng.
- Chụp snapshot phiên bản cấu hình phục vụ giao dịch NPC/chợ sát giờ bảo trì.
- Lưu ledger hoặc audit bất biến cho biến động Xu/item quan trọng và phục hồi có kiểm soát.
- Dùng cursor phân trang cho chat và block thay vì offset dễ xô lệch; tách thao tác đọc danh sách khỏi thao tác cập nhật block.
- Bảo vệ dữ liệu và giới hạn truy cập ở server, không tin số dư hoặc hàng trong túi do client gửi.

### Dữ liệu thực thể cần định nghĩa (chưa phải schema chính thức)

`Account`, `Character`, `MapInstance`, `NPC`, `NPCServiceVersion`, `ItemDefinition`, `InventorySlot`, `PendingReward`, `Wallet`, `WalletTransaction`, `FarmPlot`, `CropInstance`, `MarketListing`, `Guild`, `GuildMembership`, `GuildQuest`, `FriendRequest`, `FriendEdge`, `DirectMessage`, `ChatBlock`, `LeaderboardSnapshot`, `MaintenanceRelease`, `AdminAuditLog`.

### Ca nghiệm thu tối thiểu

- Một transaction tài sản khi retry/race chỉ có một kết quả cuối cùng hợp lệ.
- Server phục hồi sau gián đoạn không phát lại phát thưởng/hoàn tiền hai lần.
- Bộ dữ liệu nhạy cảm chat/block không rò khi thao tác chặn hoặc tìm người chơi.

**Phụ thuộc:** Tất cả hệ thống.

## TH-17. QA, vận hành và phát hành

**Mức thiết kế:** Chưa xác minh ở cấp quy trình. **Mục đích:** chứng minh game vận hành đúng bản GDD trước khi phát hành.

### Ràng buộc sản phẩm phải được kiểm thử

- `TH-17-001` [C, SRC-01] Bảo trì phải thông báo trước, áp cập nhật đúng lúc, không mất/đúp tài sản và có hướng phục hồi.
- `TH-17-002` [C, SRC-01] Giao dịch kinh tế, farm, chat, block, danh sách nhận thưởng phải chịu được lỗi mạng và thao tác đồng thời theo giới hạn của từng phân hệ.
- `TH-17-003` [P, SRC-01] Cần kiểm thử hiệu năng map, chat, tìm kiếm, farm, NPC shop, bảng xếp hạng và fan-out sự kiện; **chưa có target tải cụ thể**.
- `TH-17-004` [V, SRC-01] Chưa chốt kế hoạch alpha/beta, môi trường test, crash recovery, điều kiện phát hành và trách nhiệm vận hành.

### Đề xuất chuẩn chất lượng

- Mỗi quy tắc `C/P` có ít nhất một test case và dữ liệu kiểm thử.
- Kiểm thử tải dữ liệu thật và mô phỏng mạng chập chờn, gửi lại lệnh, đăng nhập nhiều thiết bị, bảo trì thất bại.
- Trước mỗi đợt cập nhật phải có bộ migration/rollback có thử nghiệm và đối soát tài sản trước-sau.
- Ghi lại lỗi trái với `INV-01` đến `INV-12` là lỗi ưu tiên cao, không được bỏ qua bằng thay đổi thiết kế ngầm.

**Phụ thuộc:** Tất cả hệ thống.

---

# PHẦN C — CÁC LUỒNG NGHIỆP VỤ LIÊN PHÂN HỆ

## C1. Thu hoạch → túi → bán NPC → nhận Xu

1. Người chơi thực hiện thao tác hợp lệ trên ô cây (TH-01).
2. Server xác nhận cây có thể thu; ghi biến động cây và kết quả nhận item đúng một lần (TH-16).
3. Nếu túi không đủ chỗ, không làm mất item: áp cơ chế chờ nhận đã chốt (TH-02).
4. Người chơi mở NPC, chọn chức năng ở menu trái; nội dung mua/bán tương ứng hiện bên phải (TH-03).
5. Server kiểm tra số lượng hàng và giá **theo phiên bản có hiệu lực sau bảo trì** (TH-04).
6. Server trừ item và xử lý Xu theo giới hạn ví; trường hợp Xu vượt trần phải theo nguyên tắc TH-12, không tự sửa để làm giao dịch có vẻ “công bằng” hơn.

**Thiếu để lập trình:** bảng cây/nông sản, giá, ID giao dịch, ngoại lệ tràn Xu, chính sách chờ nhận và nghiệp vụ trả lại khi lỗi.

## C2. Niêm yết thời trang → người mua đặt mua

1. Chỉ item thời trang **vĩnh viễn, không khóa** theo TH-08 mới có thể niêm yết.
2. Một tin tương ứng một món; màn hình có item/giá/tên người bán.
3. Khi có yêu cầu mua, server phải giải quyết đồng thời an toàn: không một món được bán thành công cho hai người.
4. Người mua có thể chọn người bán để xem Profile/Chat riêng/Kết bạn theo giao diện đã quyết định.

**Thiếu để lập trình:** giữ hàng/tiền lúc niêm yết và mua, phí, thời hạn, hủy, trả lại item, lịch sử giao dịch.

## C3. Chặn → từ chối tin → bỏ chặn

1. A chặn B: đây là quan hệ một chiều, không thông báo cho B.
2. Nếu B cố gửi tin riêng bị ngăn, chỉ hiện thông báo chung; không nói “A đã chặn bạn”.
3. A có thể quản lý danh sách không giới hạn theo phân trang 50/20 và tìm theo tên/ID.
4. Bỏ chặn là thay đổi trạng thái server; đóng danh sách chỉ có thể hủy tải dữ liệu, không hủy lệnh đã phát.
5. Sau bỏ chặn không có tin nhắn mới được tự gửi, không tự kết bạn và không phục hồi tin đã quá 30 ngày.

**Thiếu để lập trình:** chính xác bốn vị trí bỏ chặn, thời điểm từ chối tin trong các trường hợp race, trạng thái tin đang gửi.

## C4. Soạn thay đổi → bảo trì → công bố cấu hình

1. Admin chuẩn bị thay đổi NPC/giá/bản tin trong vùng cấu hình chưa có hiệu lực (phương án staging là **đề xuất kỹ thuật**).
2. Có thông báo trước bảo trì theo quyết định đã chốt.
3. Sau đợt bảo trì, phiên bản mới có hiệu lực cùng lúc và nhất quán; nội dung bản tin người chơi đang đọc không được tự thay giữa phiên.
4. Có phương án phục hồi nếu bản cập nhật lỗi mà không mất/nhân đôi tài sản.

**Thiếu để lập trình:** điểm chốt giao dịch, quy trình version/rollback, kịch bản đăng nhập lại và điều kiện không hoàn giao dịch cố chấp.

---

# PHẦN D — DANH SÁCH CẦN ĐỐI CHIẾU VÀ QUYẾT ĐỊNH TẬP TRUNG

## D1. P0 — bắt buộc trước khi khóa MVP và giao phát triển

| Mã | Vấn đề | Trạng thái | Đầu ra cần có |
|---|---|---|---|
| `GAP-P0-01` | Bản gốc các câu 1–1198, trọng tâm 1–660 và 798–959 | Chưa đủ nguồn | Soát xung đột và nguồn của mọi quy tắc |
| `GAP-P0-02` | MVP/V1 gồm những chức năng, màn hình và nền tảng nào | Đề xuất chưa phê duyệt | Danh sách in/out/sau MVP |
| `GAP-P0-03` | Master data cây, thời gian, sản lượng, giá hạt, giá bán, nâng cấp túi | Chưa xác minh | Bảng thông số có version |
| `GAP-P0-04` | Hạn mức Xu, quy tắc nhận Xu vượt trần và ngoại lệ rollback/hoàn | Một phần/rủi ro cao | Ma trận từng nguồn nhận/hoàn |
| `GAP-P0-05` | Giao dịch nguyên tử, idempotency, hàng chờ tài sản | Chưa xác minh kỹ thuật | Contract dữ liệu + test race |
| `GAP-P0-06` | Danh sách map, NPC, bộ đồ, quest, item, phần thưởng | Chưa xác minh | Master catalog có ID và version |
| `GAP-P0-07` | Client PC/mobile và server realtime, reconnect, định nghĩa Online | Chưa xác minh | Kiến trúc/transport và UX phục hồi |
| `GAP-P0-08` | Quy trình Admin staging/duyệt/publish/rollback và thời điểm giao dịch | Chưa xác minh | Checklist bảo trì + audit |

## D2. P1 — trước beta công khai

| Mã | Vấn đề | Đầu ra cần có |
|---|---|---|
| `GAP-P1-01` | Trọn quy trình chợ (niêm yết/mua/hủy/phí/hết hạn/hoàn) | Sơ đồ trạng thái và ca xung đột |
| `GAP-P1-02` | Vòng đời bang, quyền, reset nhiệm vụ, đóng góp và unlock item | Sơ đồ bang + bảng nhiệm vụ |
| `GAP-P1-03` | Luồng bạn bè, yêu cầu trùng, block và hiển thị Profile | Đặc tả event/state |
| `GAP-P1-04` | Bốn đường bỏ chặn; giới hạn/race đồng bộ DM | Wireframe + event contract |
| `GAP-P1-05` | UI kit, map, asset, hiệu năng PC/mobile, trải nghiệm người mới | Bộ prototype + usability test |
| `GAP-P1-06` | Quy trình ban/unban, lý do ban, khiếu nại và lịch sử quản trị | Chính sách vận hành + Admin spec |
| `GAP-P1-07` | Load/stress test map, chat, farm, chợ, leaderboard | Bộ chỉ số mục tiêu + kết quả thử |

## D3. P2 — sau khi MVP ổn định (chỉ là thứ tự ưu tiên đề xuất)

Nội dung mini-game/sự kiện mở rộng, asset thẩm mỹ thứ cấp, hiệu ứng trang trí, tùy chỉnh Admin nâng cao và hệ thống cạnh tranh ít tác động vòng gameplay đầu. Không xóa các tính năng này khỏi GDD; chỉ hoãn mức ưu tiên triển khai nếu chủ dự án chấp thuận.

## D4. Mâu thuẫn, ngoại lệ và hướng xử lý

| Mã | Điểm cần kiểm soát | Cách ghi nhận trong GDD |
|---|---|---|
| `CON-01` | “Hạt/nông sản không khóa” so với “vật phẩm không khóa có thể giao dịch” | Hạt và nông sản **không có phân loại khóa** và **cấm giao dịch P2P**; luật tổng quát chỉ dùng cho loại item được phép |
| `CON-02` | Thay giá/NPC tức thời so với chỉ sau bảo trì | Luật cũ bị thay thế; bắt buộc qua bảo trì |
| `CON-03` | Xu vượt trần bị mất so với hoàn trả khi lỗi | Giữ luật mất phần vượt trong hoạt động thường; phạm vi hoàn trả lỗi cần soát nguồn, **không tự nối hai luật** |
| `CON-04` | Túi đầy, item nhận chờ lâu, giới hạn dữ liệu | Không xóa/mất tài sản; thời gian giữ và storage cần nguồn xác minh |
| `CON-05` | Đóng danh sách chặn so với hủy bỏ chặn | Hủy fetch khác hủy mutation; mutation đã gửi vẫn phải giải quyết |
| `CON-06` | Câu trả lời A/B/C/D/E không kèm câu hỏi | **Không dùng để diễn giải quy tắc**; cần nguyên văn để đối chiếu |

---

# PHẦN E — KHUÔN CHUẨN CHO BƯỚC ĐẶC TẢ KỸ THUẬT

## E1. Điều kiện Ready for Dev

Một phân hệ chỉ được đánh dấu **Ready for Dev** khi đủ bảy thành phần:

1. Quy tắc hành vi người chơi đã chốt, và có ID truy xuất nguồn.
2. Luồng màn hình và sự kiện/transition theo từng trường hợp.
3. Input/output, điều kiện từ chối, lỗi mạng, lỗi quyền và hành vi retry.
4. Mô hình dữ liệu, giới hạn và kiểm tra phía server.
5. Tiêu chí nghiệm thu, test case và dữ liệu biên.
6. Quan hệ phụ thuộc, giao dịch xuyên phân hệ và điểm rollback.
7. Mọi mâu thuẫn được xác minh, xử lý và lưu change log.

**Tại thời điểm GDD v1.0:** chưa phân hệ nào được khẳng định đủ cả bảy tiêu chuẩn.

## E2. Mẫu đặc tả một tính năng

- **Feature ID/Tên:** `TH-XX-NNN` / tên tính năng.
- **Nguồn:** `SRC-01`, `SRC-02` và trích câu nguyên văn nếu có.
- **Trạng thái:** C/P/V/D/S.
- **Tác nhân:** người chơi/Admin/server.
- **Tiền điều kiện:** các trạng thái buộc đúng.
- **Luồng chính:** thao tác → kiểm tra server → kết quả → UI.
- **Luồng lỗi:** mất mạng, gửi lặp, túi đầy, giá đổi, tài sản tràn, reconnect, bảo trì.
- **Dữ liệu/API:** định danh, input/output, transaction ID/version.
- **Phụ thuộc:** danh sách TH liên quan.
- **Test cases/Acceptance:** kết quả kiểm chứng được.
- **Mức ưu tiên:** chỉ ghi khi MVP/V1 đã chốt.

## E3. Ma trận thử nghiệm xuyên hệ thống tối thiểu

| Test ID | Tình huống cần thử | Phạm vi |
|---|---|---|
| `QA-X-01` | Bán nông sản khi số dư Xu sát trần | TH-01/02/03/12/16 |
| `QA-X-02` | Thu hoạch/tặng thưởng trong lúc túi đầy | TH-01/02/16 |
| `QA-X-03` | Hai người mua cùng một item chợ | TH-02/08/12/16 |
| `QA-X-04` | Lệnh NPC trùng, mất mạng, gửi lại sát bảo trì | TH-03/04/12/16 |
| `QA-X-05` | Tin nhắn hết hạn riêng từng tin, danh sách 50+20, viewport | TH-05/16 |
| `QA-X-06` | Bị chặn gửi tin và không lộ người chặn | TH-05/06/14/16 |
| `QA-X-07` | Bỏ chặn trong lúc đóng danh sách/lỗi mạng | TH-06/16 |
| `QA-X-08` | Reset nhiệm vụ bang và nhận 5/10 trên hai thiết bị | TH-09/12/16 |
| `QA-X-09` | Cấp danh hiệu định kỳ chạy lại | TH-10/12/16 |
| `QA-X-10` | Mini-game hủy do bảo trì, hoàn đúng 100% | TH-04/13/12/16 |
| `QA-X-11` | Cập nhật NPC/giá/bản tin không xuất hiện trước bảo trì | TH-03/04/14/16 |
| `QA-X-12` | Khôi phục sau bảo trì thất bại không biến động tài sản hai lần | TH-04/12/16/17 |

## E4. Đề xuất lộ trình tiếp nối (chưa phê duyệt)

**Giai đoạn 1 — GDD baseline (tài liệu này):** khóa thuật ngữ, mã quy tắc, nguồn và khoảng trống; bổ sung khi nhận tài liệu bàn giao gốc.  
**Giai đoạn 2 — Duyệt MVP:** khoanh vòng chơi đầu tiên và ranh giới tính năng chưa làm.  
**Giai đoạn 3 — Technical Design Document:** thiết kế dữ liệu, API/event, client/server, bảo trì/Admin, UI flow và acceptance.  
**Giai đoạn 4 — Prototype/alpha:** một luồng chơi xuyên suốt, kiểm thử tài sản/mạng/đồng thời trước khi mở rộng bang, chợ, sự kiện và nội dung khác.

---

# PHẦN F — NHẬT KÝ THAY ĐỔI VÀ BẢO TOÀN QUYẾT ĐỊNH

## F1. Các quyết định mới đã ghi đè nguyên tắc cũ

- `OVR-01` [C, SRC-01] Điều chỉnh NPC, giá, công bố bản tin không còn triển khai tức thời; **chỉ áp dụng sau bảo trì**.
- `OVR-02` [C, SRC-01 + SRC-02] Cách bố trí NPC đa chức năng theo **menu trái/nội dung phải** và đổi thẳng nội dung (thay cho các thiết kế phải quay lại menu trước, nếu từng tồn tại).
- `OVR-03` [C, SRC-02] Mốc ô túi ban đầu được xác nhận ở mức **10 ô**, nâng cấp **+5 ô/lần**; không giữ một con số khác chỉ vì có đề xuất tạm thời trong hội thoại.

**Lưu ý:** Danh sách ghi đè trên chỉ chứa những điểm có dấu vết; không suy diễn toàn bộ thay đổi lịch sử 1–1198.

## F2. Những nguyên tắc không được “tối ưu” khác đi khi lập trình

- Không tự bật tính năng giao dịch hạt/nông sản, kể cả vì chúng “không khóa”.
- Không tự từ chối bán hoặc hoàn nông sản khi Xu tràn; cần đối chiếu/duyệt đổi quy tắc trước.
- Không tự xóa phần thưởng nhận chờ khi túi đầy.
- Không tự cập nhật NPC/giá/bản tin ngay khi Admin sửa.
- Không tự thông báo “bạn đã bị A chặn”.
- Không biến thao tác mở Chat thành đã đọc toàn bộ lịch sử.
- Không dùng trạng thái hiển thị thành công thay cho kết quả server.

## F3. Nhật ký phát hành tài liệu

| Phiên bản | Ngày | Nội dung |
|---|---|---|
| 1.0 | 19/10/2026 | Lập baseline GDD 17 phân hệ từ `SRC-01`, bổ sung chỉ những phát biểu có nghĩa rõ từ `SRC-02`; gắn mã quy tắc, nguồn, khoảng trống, rủi ro và ca test; giữ trạng thái chưa xác minh đối với phần thiếu nguyên văn. |

## F4. Tuyên bố tính đầy đủ

Bản GDD v1.0 này là **tài liệu tổng hợp có cấu trúc của toàn bộ nguồn thiết kế có thể sử dụng được trong phiên làm việc**, không phải biên bản xác thực đầy đủ cả 1198 câu và không phải cam kết tất cả cơ chế đã quyết định đều đã xuất hiện ở đây. Khi xuất hiện `SRC-03` và `SRC-04`, cần thực hiện quy trình **đối chiếu — bổ sung — ghi đè có chứng cứ**, nâng phiên bản 1.1/1.2; không sửa lén các mục `C`.

**Kết luận quản trị:** Có đủ cơ sở lập baseline 17 phân hệ và bắt đầu phân tách yêu cầu, nhưng **chưa đủ điều kiện đưa toàn bộ game vào lập trình**. Việc kế tiếp có giá trị nhất là xác minh nhóm P0 và khóa phạm vi MVP thay vì tiếp tục một chuỗi câu hỏi nhỏ.
