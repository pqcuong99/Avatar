# 00 — Quy tắc nguồn, phạm vi và quản lý thay đổi

## 0.1 Nguồn được sử dụng

- `reference/DECISIONS_P0_001_183.md`: bảng tổng hợp 183 lựa chọn rõ ràng trong các phiên mới. Quyết định sau thay thế quyết định trước nếu có bằng chứng. Dữ liệu P0 là bản tóm tắt; không phải transcript nguyên văn.
- `reference/GDD_BASELINE_17_MODULES.md`: GDD chuẩn hóa 17 phân hệ, tổng hợp nguồn cũ và kiểm kê 1.198 câu thiết kế.
- `reference/LEGACY_DESIGN_AUDIT_1198.md`: bản kiểm kê tiến độ của 1.198 câu; **không có đủ nguyên văn câu hỏi và đáp án**, không được coi tất cả chi tiết đều có bằng chứng.
- `reference/M1_AUTH_PROTOTYPE_STATUS.md`, `reference/0001_auth_ACTUAL.sql`: trạng thái bản mã nguồn M1 v0.2 (đã tạo, chưa xác minh user chạy).
- Các xác nhận người dùng sau P0-183: clone Git về máy; Client demo chạy; Go Game Server + Transaction Service `/healthz` trả về đúng; người dùng muốn handoff chi tiết cho AI khác.

## 0.2 Chính sách ưu tiên

1. Chốt P0 có nội dung rõ và mới hơn > chốt cũ xung đột, có ghi lý do.
2. GDD baseline TH-xx chỉ áp dụng khi không bị P0 mới thay thế.
3. Trường hợp **không rõ là chốt hay chưa**: `CẦN ĐỐI CHIẾU`, không suy từ một chữ lựa chọn A/B/C thiếu ngữ cảnh.
4. Chi tiết data/API/DB trong gói là **đề xuất triển khai**. AI được viết code theo thiết kế này nếu không xung đột luật game, nhưng cần review schema trước production.

## 0.3 Những thay đổi phải ghi nhận

- P0-153–162 chốt MVP/Closed Beta/Open Beta; thay thế roadmap cũ 19/10 khi mâu thuẫn.
- P0-154: nạp tiền thật **sau MVP**; không triển khai webhook nhận tiền production ở Closed Beta.
- P0-156: sâu bệnh trong MVP, **nuôi chim sau MVP**.
- P0-157: chợ người chơi sau MVP.
- P0-160/177/178: Closed Beta được **đăng ký tự do, reset dữ liệu**; Open Beta không reset.
- P0-174 chọn WSL2, nhưng P0-175 xác minh VPS Windows 2012 R2 => không tương thích; P0-176 chuyển thành sử dụng VPS cũ tạm để kiểm thử, hạ tầng Linux/Docker khi nâng cấp.
- P0-180–182: Git monorepo `https://github.com/pqcuong99/Avatar.git`; Backend giao dịch viết Go.

## 0.4 Từ điển và ID

`account`: bản ghi đăng nhập; `character`: 1 nhân vật/account; `room`: bản thể hiện một bản đồ; `map`: cấu trúc Tiled; `slot`: ô túi; `stack`: lô vật phẩm đồng loại trong 1 ô; `pending`: tài sản chờ nhận; `crop planting`: lần trồng 1 hạt ở 1 ô; `NPC`: nhân vật hệ thống; `ledger`: sổ cái bất biến; `idempotency key`: khóa chống thực thi trùng; `CCU`: online đồng thời toàn hệ thống.

## 0.5 Tiêu chuẩn “đã chốt” vs “đã chạy được”

- **Thiết kế đã chốt:** có mã P0/TH rõ.
- **Đã code:** có commit mã nguồn thực; chưa chắc đã build/test.
- **Đã tích hợp:** chương trình chạy ở môi trường phát triển qua end-to-end test.
- **Đủ Closed Beta:** đã kiểm thử các nghiệp vụ đang mở, có backup/giới hạn rủi ro và môi trường chạy phù hợp.
- **Đủ Open Beta:** đáp ứng P0-139–152, 160–162, HA/khôi phục/giao dịch bảo toàn, test 300 CCU.

Không để AI khác gộp các trạng thái này thành “hoàn thành”.
