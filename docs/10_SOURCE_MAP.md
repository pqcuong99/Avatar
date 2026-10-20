# BẢN ĐỒ NGUỒN & KIỂM SOÁT XUNG ĐỘT

## Nguồn đầu vào đang có trong gói

| Tệp | Thời điểm | Phạm vi | Quy tắc sử dụng |
|---|---|---|---|
| `docs/archive/DESIGN_AUDIT_1198_2026-10-18.md` | 18/10/2026 | Kiểm kê 17 phân hệ từ chuỗi cũ đến câu 1198 | Hợp lệ như **báo cáo kiểm kê**, không là transcript đầy đủ 1198 câu |
| `docs/archive/GDD_baseline_2026-10-19.md` | 19/10/2026 | GDD chuẩn hóa 17 phân hệ và các ràng buộc cũ | Giữ các quy tắc đã chốt khi không bị quyết định P0 mới thay thế |
| `docs/archive/ROADMAP_DRAFT_superseded_2026-10-19.md` | 19/10/2026 | Kế hoạch MVP/V1/Sau V1 **đề xuất chưa phê duyệt** tại thời điểm viết | **Không sử dụng làm phạm vi MVP hiện tại**; được P0-153–162 cập nhật |
| `docs/01_DECISION_REGISTER.md` + CSV/JSON | đến 20/10/2026 | P0-001–P0-183, ngữ cảnh lựa chọn mới | Ưu tiên cho các quyết định rõ có ngày mới hơn; lưu nhãn điều chỉnh/superseded |
| `docs/archive/STARTER_README_v0.1.md` | 20/10/2026 | Mô tả starter code v0.1 | Chỉ chứng minh starter đã được chuẩn bị; không suy ra game MVP đã hoạt động |

## Trạng thái chứng cứ và giới hạn

1. Có **bản tóm tắt/mã quyết định** P0-001–P0-183 và các lượt chọn được cung cấp trong dự án, không phải toàn bộ log chat nguyên văn dưới dạng file nguồn 183 câu.
2. Bộ câu hỏi cũ 1–1198 **thiếu transcript đầy đủ**. GDD đã ghi cảnh báo này từ 19/10; không thể xuất chính xác từng câu hỏi và đáp án cũ khi không có file gốc.
3. Phiên bản VPS 2012 R2 được xác nhận qua ảnh PowerShell người dùng; **không có kiểm chứng runtime trực tiếp trên VPS**.
4. Repository URL và tình trạng rỗng do người dùng cung cấp; **không có xác nhận đọc/ghi GitHub Repo hoặc push thành công** từ công cụ trong phiên bàn giao.
5. Các file `04_DATA_MODEL_DRAFT.md`, `05_API_EVENTS_DRAFT.md`, `07_ROADMAP.md` chứa nhiều đề xuất triển khai được gắn nhãn; không biến thành quyết định game mới.

## Thứ tự ưu tiên khi nguồn khác nhau

(a) Phát biểu mới, rõ ràng, đã chốt trong P0 → (b) quy tắc GDD 19/10 đã chốt mà không xung đột → (c) báo cáo kiểm kê 18/10 → (d) đề xuất kỹ thuật và roadmap. **Không tự ghi đè khi thiếu bằng chứng rõ**: gắn `CẦN ĐỐI CHIẾU` vào Git issue.

## Cập nhật về sau

- Thêm `P0-184+` vào file registry + CSV/JSON khi có **quyết định thực sự mới**.
- Bất kỳ thay đổi làm sai luật cũ phải có đề mục “thay thế quyết định nào, vì sao, áp dụng từ phiên bản nào”.
- Khi master data farm/NPC/nhiệm vụ được Admin duyệt, khóa version cho mỗi đợt bảo trì và lưu bản đã dùng để test.
