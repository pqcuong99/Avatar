# TIÊU CHUẨN NGHIỆM THU & BẢO MẬT — MVP / BETA

## 1. Trạng thái và release gates [CHỐT]

**Closed Beta:** dùng thử, người chơi tự đăng ký, dữ liệu xóa toàn bộ ở cuối kỳ; giữ Admin/config. Phải thông báo reset trước. VPS cũ chỉ dùng **khi xác nhận runtime tương thích và hạn chế rủi ro an toàn**.

**Open Beta:** không reset; chỉ mở khi các chức năng MVP hoàn tất, không còn lỗi nghiêm trọng mất/nhân đôi tài sản, đạt kiểm thử tải tối thiểu **300 người đồng thời**, có phương án khôi phục/backup, an toàn giao dịch/bảo mật (P0-160–162). Một VPS hiện tại **không thỏa các yêu cầu failover độc lập đã chốt**.

## 2. Ma trận kiểm thử nghiệp vụ trọng yếu

| ID | Kiểm tra | Kết quả mong đợi |
|---|---|---|
| QA-A01 | Account đăng nhập hai thiết bị | Phiên mới có hiệu lực; phiên cũ không điều khiển được |
| QA-A02 | Mất mạng dưới 30s | Nhân vật giữ phòng và suất, reconnect phục hồi đúng vị trí |
| QA-A03 | Mất mạng quá 30s | Giải phóng suất, reconnect dùng vị trí server lưu |
| QA-A04 | Đặt tên có dấu/không dấu, hoa/thường, emoji cho phép, grapheme | Áp dụng duy nhất, whitelist và 3–16 grapheme chính xác |
| QA-M01 | Client gửi teleport/xuyên vật cản/tăng tốc | Server từ chối; log; không auto cấm tài khoản |
| QA-M02 | Phòng đầy, người bạn nhấn Đến cùng phòng | Từ chối và báo lý do, không nhân đôi phiên phòng |
| QA-F01 | Cây gieo trước cập nhật tăng trưởng/sản lượng | Giữ snapshot ban đầu; giá bán NPC dùng giá mới đã công bố |
| QA-F02 | Thu hoạch 1 cây khi túi chỉ chứa được một phần lô | Toàn bộ lô đưa pending; không thất thoát/nhân đôi |
| QA-F03 | Thu hoạch tất cả với một số lô không đủ chỗ | Duyệt plot ID tăng dần; lô tràn chuyển pending, tiếp tục plot khác |
| QA-F04 | Pending cùng crop, FIFO, Claim All, partial exceptional | Gộp cùng loại, giữ earliest date, tuân P0-075–085 |
| QA-F05 | Sâu xảy ra trước 70% growth, quá hạn 30% growth | Trừ yield bằng floor(base × 5..10%); không bốc lại khi reconnect |
| QA-F06 | Người chơi Offline, bảo trì, đăng nhập lại | Cây đủ thời gian trưởng thành theo mốc server |
| QA-I01 | Stack >999 cho vật phẩm farm/upgrade; thời trang 2 món | Tự chia ô farm/upgrade, fashion 1 ô/item |
| QA-T01 | Gửi cùng request_id NPC buy 100 lần đồng thời | Tài sản chỉ thay đổi 1 lần, trả cùng outcome |
| QA-T02 | Server crash giữa thay đổi Xu và cấp item | Nguyên tử; rollback hoặc commit trọn vẹn; ledger khớp |
| QA-T03 | Standby không bảo đảm durability | Reject/freeze giao dịch thay đổi tài sản, không success giả |
| QA-T04 | Admin đề xuất và tự phê duyệt tài sản | Reject ngay cả khi là Super Admin |
| QA-T05 | Admin không bật TOTP | Không được sử dụng Admin; RBAC chặn quyền trái phép |
| QA-C01 | Private chat sau 30 ngày | Tin hết hiệu lực theo GDD; không hồi sinh khi unblock |
| QA-C02 | Bị chặn chat riêng | Không lộ danh tính người chặn; lỗi chung |
| QA-C03 | 50 tin đầu +20 tin cũ; cuộn gần đầu | Phân trang đúng, không nhảy vị trí cuộn |
| QA-R01 | Restore bản backup theo PITR | Dữ liệu và ledger đối soát; nạp tiền hợp lệ không cấp trùng |
| QA-R02 | Game Server chết khi người chơi online | Các instance khỏe hoạt động, người lỗi reconnect sang phòng thay thế |
| QA-B01 | Reset Closed Beta | Xóa account/character/social/farm/assets thử nghiệm; giữ Admin/config |

## 3. QA vận hành & bảo mật [CHỐT + ĐỀ XUẤT]

- **[CHỐT]** No-critical/major data integrity bugs; không làm mất/nhân đôi tài sản; nguồn sự thật ở Server.
- **[CHỐT]** Backups daily + PITR + lưu ngoài host + diễn tập restore; failover an toàn theo P0-139/148/149/150.
- **[CHỐT]** DDoS/WAF, rate limits trên API/Gateway/action; Admin bắt buộc TOTP, người chơi tùy chọn.
- **[ĐỀ XUẤT]** Unit, integration, deterministic farm/pest tests, concurrency tests, migration tests, end-to-end mobile landscape, security review, build reproducibility.
- **[ĐỀ XUẤT]** Test tải 300 active sessions với movement/chat/farm/NPC, trong đó test ngắt kết nối, session churn và chống reconnect storm; báo cáo CPU/RAM/latency/DB locks.

## 4. Mức độ lỗi và nguyên tắc go/no-go [ĐỀ XUẤT]

- **P0/blocker:** thất thoát/tạo tài sản, chiếm tài khoản, truy cập Admin trái phép, ghi kép, hỏng DB, secret lộ, có thể reset sai Open Beta. Không mở Beta công khai.
- **P1/high:** không đăng nhập, phòng sập liên tục, chat rò rỉ dữ liệu riêng tư, crash phổ biến. Phải sửa trước Open Beta.
- **P2/normal:** lệch UI/animation không ảnh hưởng tiền tệ/khả năng chơi; có thể lên backlog có chấp thuận.
- **P3/low:** polish thẩm mỹ/thao tác phụ.

## 5. Biên bản kiểm thử bắt buộc trước Open Beta

Lưu phiên bản Client/Server, số commit Git, cấu hình tải, báo cáo concurrency và crash tests, backup/restore drill, báo cáo security, checklist admin sign-off và ngày thông báo bảo trì. Không ghi 'đạt 300' chỉ vì mô hình kiến trúc cho phép scale.
