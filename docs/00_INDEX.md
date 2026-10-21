# AVATAR — HỒ SƠ BÀN GIAO & KHỞI ĐỘNG PHÁT TRIỂN

**Ngày chốt bản:** 20/10/2026  
**Repo do chủ dự án xác nhận:** `https://github.com/pqcuong99/Avatar.git` (được xác nhận là repo mới, chưa có code)  
**Định dạng:** Markdown/CSV/JSON, lưu được bằng Git, đọc được bằng GitHub.  
**Trạng thái:** Đã có bộ khung mã nguồn, **chưa hoàn tất MVP, chưa triển khai lên VPS**.

## Cách đọc (theo thứ tự)

| Ưu tiên | File | Dùng để làm gì |
|---|---|---|
| 1 | `02_PRODUCT_SPEC_MVP.md` | Yêu cầu chức năng của MVP; những gì chưa làm |
| 2 | `01_DECISION_REGISTER.md` | 183 quyết định P0 đã ghi nhận; lựa chọn đã bị thay thế |
| 3 | `03_ARCHITECTURE.md` | Kiến trúc Client–Server, giao dịch, mạng, failover |
| 4 | `04_DATA_MODEL_DRAFT.md` | Mô hình dữ liệu dự kiến, bất biến và migration |
| 5 | `05_API_EVENTS_DRAFT.md` | Hợp đồng API/WebSocket nháp để bắt đầu code |
| 6 | `06_ACCEPTANCE_QA.md` | Checklist kiểm thử, lỗi P0/P1, điều kiện ra Beta |
| 7 | `07_ROADMAP.md` | Giai đoạn phát triển, backlog, tiêu chí bàn giao |
| 8 | `08_GIT_DEPLOYMENT.md` | Cách đưa starter lên repo, chạy local, lưu ý VPS |
| 9 | `09_OPEN_ISSUES.md` | Dữ liệu/luật chưa rõ, xung đột tài liệu và quyết định cần bổ sung |
| 10 | `10_SOURCE_MAP.md` | Chỉ rõ tài liệu nào là nguồn gốc, đâu là suy luận/đề xuất |
| phụ trợ | `master-data/*.csv` | Bảng cấu hình gốc và mẫu để Admin điền |
| lưu trữ | `docs/archive/` | GDD 17 phân hệ, kiểm kê 1198, roadmap cũ, README starter |

## Trạng thái tin cậy

- **[CHỐT]**: Quyết định được chủ dự án chọn trong cuộc hội thoại có ngữ cảnh hoặc nguồn lịch sử xác nhận.
- **[ĐẶC TẢ TRIỂN KHAI]**: Diễn đạt kỹ thuật nhằm hiện thực hóa một quy tắc đã chốt; phải kiểm thử.
- **[ĐỀ XUẤT]**: Mô hình bảng, API, cấu trúc module, tổ chức thứ tự thực hiện; chưa có nghĩa đã được chủ dự án duyệt từng chi tiết.
- **[CẦN ĐỐI CHIẾU]**: Nguồn hiện có chưa đủ để khẳng định; không đặt giá trị mặc định im lặng.
- **[ĐÃ BỊ THAY THẾ]**: Ý định cũ không còn khả thi hoặc bị quyết định mới ghi đè.

## Phân biệt bộ hồ sơ cũ và mới

Báo cáo `archive/DESIGN_AUDIT_1198_2026-10-18.md` và `archive/GDD_baseline_2026-10-19.md` ghi nhận lịch sử đến câu **1198** theo hệ đánh số cũ. Không có transcript nguyên văn đầy đủ của toàn bộ 1–1198. Đừng suy diễn từ số thứ tự thành 1.198 luật riêng biệt. GDD 19/10 **chưa chốt MVP/công nghệ** vào thời điểm đó; các quyết định P0 về sau đã cập nhật/ghi đè phần này.

## Tài sản hiện có

- Bộ khung Client: React + Phaser + TypeScript, có bản đồ demo cục bộ; **không có multiplayer thật**.
- Game Server Go: HTTP `/healthz`, `/readyz`; chưa có WSS/phòng chơi thật.
- Transaction Service Go: HTTP health; API giao dịch chưa được hiện thực và trả 501.
- Compose cho máy lập trình tương thích Linux containers; **không phải bản deploy Windows Server 2012 R2**.
- Các tài liệu đặc tả + dữ liệu gốc trong gói này.

## Thứ tự làm ngay

1. Giải nén repo-ready, xem `09_OPEN_ISSUES.md`, không đẩy bất kỳ secret nào lên Git.
2. Clone repo đã tạo; copy nội dung gói vào repo; chạy test Go + Client trong môi trường máy cá nhân đủ hỗ trợ.
3. Commit/push bản đầu; thiết lập nhánh, CI và GitHub Issues theo backlog.
4. Thực hiện **M1 tài khoản + PostgreSQL + session**; sau đó M2 map multiplayer.
5. **Không mở Closed Beta trên Windows Server 2012 R2 trước khi có đường chạy tương thích và đánh giá bảo mật thực tế.**
