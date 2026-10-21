# PROMPT TASK — PostgreSQL schema / migrations

Dựa trên `docs/06_DATABASE_SPEC.md` và `db/draft/0002_domain_PROPOSAL.sql` hãy:

1. Kiểm tra migration thật `database/migrations/0001_auth.sql` hiện có trong git và dữ liệu của DB local; không đổi hoặc drop bảng đã tồn tại.
2. Review đề xuất schema theo các bất biến: 1 account/character, unique Unicode display name, inventory stack/cap, wallets nonnegative + caps, farm snapshot, pending gộp FIFO, ledger append-only, request idempotency, Admin dual approve.
3. Chia draft thành migrations theo milestone M1b, M3a, M4, M5, M6; chỉ apply phần cần thiết cho milestone được giao.
4. Viết các migration FORWARD mới, rollback script có đánh giá tác động data, seed mẫu có nhãn DEMO, không điền ngầm yield/sale/pest chính thức.
5. Viết integration tests trên PostgreSQL tạm, concurrent writes và các FK/cascade; verify Closed Beta reset giả lập vẫn giữ Admin và config.
6. Đánh dấu rõ các business invariants chỉ kiểm qua Service: Unicode grapheme, inventory max theo catalog, friend max100, NPC collider, pest spawn window.

Không tự thực thi draft SQL trên DB thật, không xóa volume. Cung cấp ERD cập nhật, docs mô tả columns và indexes, nhật ký kiểm thử.
