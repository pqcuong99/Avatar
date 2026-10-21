# HƯỚNG DẪN GIAO CHO AI KHÁC

## Nếu AI hỗ trợ upload file

1. Upload ZIP `Project_Thanh_AI_Handoff_v2_2026-10-21.zip` (toàn bộ tài liệu), và cung cấp quyền đọc Repository `https://github.com/pqcuong99/Avatar.git` nếu có. AI phải kiểm tra mã nguồn thật vì ZIP chỉ gồm tài liệu, không thay thế repo.
2. Gửi nguyên văn nội dung `ai-prompts/00_MASTER_PROMPT_FOR_OTHER_AI.md`, thêm câu "Hãy triển khai milestone M1b sau khi xác minh M1a đã tồn tại" hoặc task mong muốn.
3. Với mỗi module, đọc prompt trong `ai-prompts/` và docs tương ứng, yêu cầu AI trả code + migration + tests, không chỉ kế hoạch.
4. Nếu AI không đọc ZIP, upload tệp `MASTER_SPEC_ALL_IN_ONE.md`, rồi SQL draft và OpenAPI khi làm backend, kèm `reference/GDD_BASELINE_17_MODULES.md` cho module sau MVP.

## Nếu dùng Git trong dự án

Sao chép **cả thư mục** tài liệu này vào `F:\Avatar\docs\ai-handoff-v2\`. Chỉ thêm file tài liệu, **không ghi đè code**. Chạy `git status` kiểm tra trước khi commit/push. SQL trong `db/draft` là thiết kế đề xuất **không đặt trong thư mục auto-migration**.

## Kết quả AI cần cung cấp mỗi lần

1. Nguồn P0/TH đã dùng.
2. File được tạo/sửa và migration có tác động gì.
3. Lệnh chạy trên Windows local, cấu hình .env.example không chứa secret.
4. Kết quả go test/React build/test đã chạy thật hoặc nêu lý do chưa chạy.
5. Báo cáo kiểm thử theo mã `docs/10_ACCEPTANCE_TESTS.md`, trạng thái PASS/FAIL/BLOCKED.
6. Trường hợp còn thiếu trong `docs/12_GAPS_BLOCKERS.md` và câu hỏi tối quan trọng nếu cần.

## Điểm dừng không được tự làm

Không chạy SQL draft trên DB thật, không xóa volume Docker, không reset người chơi chưa có xác nhận, không cấu hình thanh toán tiền thật, không mở Beta công khai trên Windows Server 2012 R2 chưa đánh giá an toàn.
