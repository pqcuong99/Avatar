# PROJECT THANH — BỘ ĐẶC TẢ BÀN GIAO CHO AI LẬP TRÌNH

**Phiên bản:** AI Handoff v2.0 • **Ngày:** 21/10/2026 • **Repository:** https://github.com/pqcuong99/Avatar.git  
**Mục đích:** Cung cấp nguồn yêu cầu và hợp đồng kỹ thuật để AI/đội dev khác xây game browser 2D Project Thanh, cơ sở dữ liệu PostgreSQL, Go Server, React/Phaser Client, vận hành và QA **mà không cần đọc lại toàn bộ cuộc trò chuyện**.

## Đọc theo thứ tự

1. `docs/00_CONVENTIONS_AND_SOURCES.md` — độ tin cậy, cách tránh tự chế luật.
2. `docs/01_PRODUCT_GDD_MVP.md` — đặc tả sản phẩm, luồng chính, phạm vi MVP.
3. `docs/02_GAMEPLAY_FARM_INVENTORY_ECONOMY.md` — quy tắc cây trồng, sâu, túi, tiền, NPC.
4. `docs/03_SOCIAL_CHAT_CHARACTER_ADMIN.md` — danh tính, xã hội, thời trang, nhiệm vụ, Admin.
5. `docs/04_UI_UX_SCREENS.md` — màn hình, trạng thái và quy trình UI.
6. `docs/05_TECHNICAL_ARCHITECTURE.md` — service, giao tiếp, phân phòng, di chuyển.
7. `docs/06_DATABASE_SPEC.md` + `db/draft/0002_domain_PROPOSAL.sql` — mô hình dữ liệu chi tiết.
8. `docs/07_HTTP_API_SPEC.md` + `contracts/openapi-draft.yaml` — endpoints và mã lỗi.
9. `docs/08_WEBSOCKET_PROTOCOL.md` — handshake, JSON/binary, sequence/reconnect.
10. `docs/09_SECURITY_OPERATIONS.md` — bảo mật, giao dịch, backups, Beta.
11. `docs/10_ACCEPTANCE_TESTS.md` — kiểm thử và test cases.
12. `docs/11_ROADMAP_AND_BACKLOG.md` — thứ tự làm và Definition of Done.
13. `docs/12_GAPS_BLOCKERS.md` — phần chưa có quyết định/cấu hình.
14. `ai-prompts/` — prompt điều phối và prompt triển khai từng nhóm module.
15. `reference/` — tài liệu nền, sổ P0 và chứng cứ nguồn; `master-data/` — dữ liệu mẫu, không tự xem mọi số liệu là đã chốt.

## Quy ước bắt buộc

- `[CHỐT]`: quyết định người dùng đã xác nhận, phải thực hiện đúng.
- `[ĐỀ XUẤT]`: chi tiết thiết kế kỹ thuật để AI có thể xây dựng; có thể sửa sau review, không được quảng bá là luật game do người dùng chốt.
- `[CẦN CẤU HÌNH]`: yêu cầu đã chốt nhưng giá trị chưa được điền (ví dụ năng suất, giá NPC, xác suất sâu).
- `[CẦN ĐỐI CHIẾU]`: thiếu bằng chứng nguyên văn hoặc xung đột cũ–mới; AI không được tự quyết định.
- `[CHƯA LẬP TRÌNH]`: mô tả hành vi yêu cầu, không có nghĩa code đã tồn tại.

**Bản chất gói này:** đặc tả + **SQL/API đề xuất kỹ thuật có gắn nhãn**. Không phải toàn bộ game đã lập trình. **Không chạy trực tiếp migration đề xuất lên DB có dữ liệu mà chưa review và test trên bản sao.**

## Trạng thái triển khai thực tế tại thời điểm bàn giao

- Máy cá nhân Windows đã chạy được giao diện Phaser/React demo và nhận được phản hồi hợp lệ từ hai `/healthz` của Go services (theo xác nhận của chủ dự án).
- Có patch M1 v0.2 tạo API đăng ký/đăng nhập và migration `0001_auth.sql`, **chưa được người dùng xác nhận đã áp dụng/chạy thành công**. Gói này lưu bản `0001_auth.sql` thực trong `reference/`, không tự nhận đã deploy.
- Git Repository đã được clone về máy người dùng, nhưng **chưa xác minh đã push hoặc merge patch M1**.
- VPS hiện tại: **Windows Server 2012 R2 Datacenter build 9600**, một VPS; dự kiến Closed Beta tạm thời; **không chạy WSL2 hoặc Docker Linux như Ubuntu**. Go runtime hiện đại/HA/HA database chưa được chứng minh hoạt động trên VPS cũ.
- Open Beta không reset, chỉ phát hành khi đạt bài test hệ thống và tối thiểu **300 CCU**; Closed Beta đăng ký tự do và xóa toàn bộ player data khi kết thúc (giữ Admin/config).

## Quy tắc chuyển giao cho AI khác

1. Đọc tài liệu theo thứ tự; lấy P0 gần nhất ưu tiên hơn baseline cũ khi có xung đột rõ.
2. Không tự chọn các con số đang `[CẦN CẤU HÌNH]` để làm dữ liệu production.
3. Mỗi pull request phải chỉ rõ mã P0/TH, migration tương ứng, test và chứng cứ nghiệm thu.
4. Không khẳng định QA/triển khai nếu chưa có output thực tế.
5. Không chạy `docker compose down -v` hoặc reset Database nếu chưa được phép; không đưa `.env`, token, password vào Git.
6. Giữ tương thích với `accounts`/`auth_sessions` trong migration M1 v0.2, hoặc viết migration chuyển đổi an toàn.

## Gợi ý đưa tài liệu vào Repository

Sao chép toàn bộ nội dung của gói này vào `docs/ai-handoff-v2/` của repository `Avatar`, **không đè** các tài liệu nguồn đang có. `db/draft/` là SQL dự thảo để review, không đặt ngay trong thư mục migration được tự động chạy.
