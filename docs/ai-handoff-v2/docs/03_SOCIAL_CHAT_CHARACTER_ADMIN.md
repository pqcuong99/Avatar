# 03 — Account, Character, Chat, Friends, Fashion, Daily Quests, Admin

## 1. Account và đăng nhập [CHỐT]

- Đăng ký tên tài khoản và mật khẩu; email khôi phục có thể để trống. Nếu đã liên kết email và xác minh, reset qua mã/link một lần **hạn 10 phút**, reset mật khẩu phải revoke các session cũ.
- Không có email: hỗ trợ Admin theo chứng cứ sở hữu, đánh giá từng trường hợp; kênh xác minh có thể email xác thực, Zalo/Discord hoặc ticket an toàn do Admin chọn.
- Login mới vô hiệu phiên cũ của cùng account; logout/reconnect kiểm tra lại tại server. Có một nhân vật/account, không có chức năng xóa/tạo lại.
- Anti-bruteforce đa tầng: hạn tốc độ theo account/IP/dấu hiệu thiết bị, tăng delay/CAPTCHA khi rủi ro; không tự khóa vĩnh viễn do nhập sai.
- Người chơi có thể bật 2FA TOTP và recovery codes, mặc định không ép buộc. Đổi/tắt 2FA phải xác minh quyền sở hữu.

**[ĐỀ XUẤT]** Không dùng tài khoản login name làm character display name. Tách bảng character, username validation riêng; M1 v0.2 chỉ làm username ASCII tạm.

## 2. Đặt tên Character [CHỐT P0-018–029]

- Chính xác 1 character/account; không xóa để tạo lại.
- Tên **duy nhất toàn game**, so sánh **không phân biệt hoa/thường**, **vẫn phân biệt dấu tiếng Việt**.
- Độ dài 3–16 **Unicode grapheme clusters** (ký tự nhìn thấy), không dùng số byte hay rune để quyết định.
- Cho phép chữ tiếng Việt, chữ/số, dấu cách, và các ký tự `_`, `-`, `.`, `★`, `♥`, `☆`, `♡`. Tập/điều kiện khoảng trắng liên tiếp ở đầu/cuối **chưa chốt** — review nếu muốn chuẩn hóa.
- Đổi tên bằng một Thẻ Đổi tên khi thành công, không cooldown, không giới hạn số lần. Card source Admin cấu hình; rename thất bại không tiêu thẻ.
- **[ĐỀ XUẤT]** `normalized_key=NFC + Unicode Default Case Folding` ứng dụng tính, khóa UNIQUE database; không strip dấu tiếng Việt; lưu raw display NFC. Test cả chữ dạng composed/decomposed.

## 3. Hiện diện, map, friends [CHỐT]

- Tự chia room theo cap Admin; đầy tự tạo room; click `Đến cùng phòng` từ Profile bạn bè nếu room bạn còn chỗ, còn đầy từ chối rõ; không có chọn kênh thủ công.
- Disconnected trong 30 giây: đứng yên trong room, không icon offline, vẫn chiếm chỗ; reconnect trong grace về đúng room/vị trí. Sau 30 giây giải phóng; login sau dùng vị trí Server lưu và tự phân room.
- Friends: tối đa **100 người** từ GDD baseline TH-07; người chơi tìm thủ công, nhấn kết bạn; Profile hỗ trợ xem người và Chat riêng/Kết bạn.
- Các trạng thái lời mời bạn và điều kiện hủy/đồng thời cần tuân baseline nếu có; nếu không có chi tiết phải có nhãn đề xuất.

## 4. Public chat và Private chat

### Public chat [CHỐT MVP]

- Tin công cộng tới thành viên **cùng room**, không tự phát toàn server.
- Tin nhắn gửi thành công khi được server xác thực và ghi/đẩy đúng; rate-limit và moderation để tránh spam.
- **[CẦN CẤU HÌNH]** ký tự tối đa, rate limit/từ cấm, thời hạn giữ tin công cộng, emoji/file chưa chốt.

### Private chat [CHỐT từ TH-05/TH-06]

- Người chơi chọn người, nhấn `Chat riêng`; hiển thị đúng hội thoại.
- Lưu tin từng tin **30 ngày kể từ lúc gửi thành công**. Chỉ những tin chưa hết hạn được trả về; xóa/ẩn có quy trình riêng, không hồi sinh sau unblock.
- Tải lần đầu **50 tin**; khi cuộn gần đầu lấy thêm **20 tin cũ**, giữ đúng vị trí scroll, phân trang theo cursor để tránh lặp.
- `seen/read` gắn với tin thực sự đi vào **viewport**; không tự đánh dấu đọc toàn bộ khi mở hội thoại. Có typing indicator, seen, draft trong phiên.
- Block chat riêng **một chiều**, không công khai cho người bị chặn; khi họ gửi không thành công hiện lỗi chung, không lộ nguyên nhân. Unblock không tự chat/kết bạn/khôi phục message hết hạn.
- Danh sách chặn **không giới hạn**, mới chặn trước, tìm tên không phân biệt hoa/thường/**dấu** hoặc ID đầy đủ; trang đầu 50 rồi thêm 20.
- GDD nhắc **4 đường bỏ chặn** nhưng chưa có tên/hành vi chính xác của tất cả; **[CẦN ĐỐI CHIẾU]** trước thiết kế nút UI chi tiết.
- Đóng màn danh sách chặn có thể hủy load dữ liệu chưa xong, **không hủy lệnh unblock** server đã nhận.

## 5. Fashion [CHỐT MVP phạm vi cơ bản]

- NPC bán một số đồ trang phục/phụ kiện; người chơi mua, sở hữu trong túi, mặc, tháo, thay món.
- Mỗi fashion item là **một instance** (không stack), nhân vật khác trong room nhìn thấy trang phục đang mặc.
- Bound/unbound và hạn dùng theo rule từng item, không đồng nhất với seed/crop.
- **[CẦN CẤU HÌNH]** item catalog MVP, ảnh/tile sprite, slot trang bị, giá, layer hiển thị/priority, collider/animation.
- **[ĐỀ XUẤT]** entity `fashion_item_instance`, `equipment_slot`, sự kiện `appearance.changed` sau commit để sync room.

## 6. Daily personal quests [CHỐT MVP]

- Nhiệm vụ ngày cơ bản theo hoạt động có trong MVP (trồng, thu, bán NPC, chat và hoạt động tương thích).
- Tiến độ được tính bởi sự kiện server, tránh client giả report; thưởng 1 lần khi đạt; tránh trùng qua idempotency.
- **[CẦN CẤU HÌNH]** số nhiệm vụ/ngày, danh sách, điều kiện, thưởng, thời điểm reset/ngày theo timezone; không tự dùng chính sách 10 nhiệm vụ/5 nhận của nhiệm vụ **bang hội** cho cá nhân.
- **[ĐỀ XUẤT]** khóa unique `(character_id, quest_id, reset_date)`, earned/claimed_at; reset an toàn khi qua ngày.

## 7. Admin & các thao tác nhạy cảm [CHỐT]

| Vai trò | Phạm vi ví dụ | Lưu ý |
|---|---|---|
| Super Admin | Quản lý quyền và hệ thống | Không tự approve yêu cầu tài sản của mình |
| Game Admin | NPC, catalog, bảo trì, nội dung, xử lý vi phạm | Chỉ quyền được cấp |
| Hỗ trợ | Xem/tra cứu, hỗ trợ người chơi | Không mặc định có quyền sửa số dư |
| Tài chính | Đơn nạp/đối soát (sau MVP) | Không tự động có quyền mọi tài sản |

- RBAC đến từng permission; Admin **bắt buộc 2FA TOTP**, có recovery code một lần.
- Mọi thay đổi thủ công Xu, KC, item: `proposer` tạo yêu cầu, Admin khác có quyền `approver` quyết định. **Không cho cùng một tài khoản tạo và duyệt**, kể cả Super Admin. Sau approve mới commit ledger; ghi lý do, before/after và audit.
- Các giao dịch gameplay tự động bình thường không yêu cầu hai Admin duyệt.
- Bản tin, giá, NPC/config thay đổi chỉ phát hành sau bảo trì có thông báo, staging/test/backup/QA, rollback theo nguyên tắc không làm mất giao dịch.
- Bị ban hiển thị lý do do Admin nhập theo baseline cũ; **[CẦN ĐỐI CHIẾU]** các loại/độ dài/thời hạn ban nếu muốn triển khai đầy đủ.

## 8. Sau MVP — không triển khai trong các module MVP

- Nuôi chim bắt sâu: chim non 12h + phải đủ worm food; trưởng thành sống 720h; 4 captures/rolling 60m/bird; ưu tiên deadline sớm/plot thấp; ăn sâu ngay; giao dịch cho NPC. Chi tiết P0-103–133.
- Chợ player: vật phẩm vĩnh viễn đủ điều kiện, không khóa, mua bán theo baseline; nông sản/hạt giống không thể lên chợ.
- Nạp tiền thật QR: 1.000 VND/KC, đơn QR unique, 15 phút, webhook và đối soát, chỉ khi mở module thanh toán sau MVP.
- Bang hội/leaderboard/câu cá/mini-games vẫn có baseline 17 hệ; **chưa chốt tất cả thời điểm phát hành**.
