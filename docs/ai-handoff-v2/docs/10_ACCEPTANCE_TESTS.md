# 10 — Ma trận test nghiệp vụ, bảo mật, tích hợp, performance

**Quy tắc:** Bất kỳ module nào được AI lập trình phải có unit + integration + API/WS contract tests, tối thiểu một kịch bản lỗi mạng/retry. Chỉ đánh PASS khi đã chạy thật, không dựa trên kiểm tra mã bằng mắt.

## 1. Tài khoản và nhân vật

| ID | Given | When | Then |
|---|---|---|---|
| A-01 | Account đăng nhập thiết bị A | login trên B | cookie/session A không còn thao tác hợp lệ |
| A-02 | Account chưa có character | tạo 2 character đồng thời | chính xác 1 record và 1 gói tài sản gốc |
| A-03 | Tên `Thanh` đã dùng | tạo `thanh` | từ chối trùng (casefold) |
| A-04 | Tên `Thanh` đã dùng | tạo `Thành` | hợp lệ nếu không có trùng khác (phân biệt dấu) |
| A-05 | Tên Unicode tổ hợp dấu | tạo và tìm unique | NFC / grapheme đúng, không né unique bằng combining marks |
| A-06 | Tên 2 hoặc 17 grapheme | tạo character | báo lỗi độ dài |
| A-07 | Account có rename card | rename thất bại do trùng | không trừ card |
| A-08 | Account yêu cầu reset pw | dùng code sau >10 phút | từ chối, không đổi pw |
| A-09 | Đã reset password | dùng cookie cũ | bị revoke |
| A-10 | Player chưa bật TOTP | login | không bắt 2FA; khi bật thì cần OTP |
| A-11 | Admin chưa bật TOTP | login Admin | không vào Admin |
| A-12 | Nhiều đăng nhập sai | brute force | tăng delay/rate limit/CAPTCHA, không khóa vĩnh viễn |

## 2. World và networking

| ID | Given | When | Then |
|---|---|---|---|
| W-01 | N phòng đã đầy | player mới vào map | tự tạo/chọn room còn chỗ, không vượt cap |
| W-02 | Bạn ở room đầy | bấm Đến cùng phòng | báo phòng đầy, player giữ trạng thái cũ |
| W-03 | Player mất mạng 29s | reconnect | đúng room/vị trí, 1 suất |
| W-04 | Player mất mạng 31s | reconnect | suất đã giải phóng, tự gán room, position hợp lệ |
| W-05 | Client gửi teleport/tăng tốc | Go nhận movement | reject, log, không auto ban |
| W-06 | Đang auto-path | WASD/joystick | hủy auto-path tức thì |
| W-07 | Go World Server chết | replacement healthy | phiên chịu ảnh hưởng reconnect từ durable state, Server khác không chết |
| W-08 | Client/server map version lệch | WS join | không dùng collider khác bản cho di chuyển hợp lệ |

## 3. Farm + inventory

| ID | Given | When | Then |
|---|---|---|---|
| F-01 | 6 ô và 1 hạt | trồng 1 | đúng 1 hạt bị trừ, planting tạo cùng transaction |
| F-02 | Trồng trước lần bảo trì thay yield/time | sau update thu | giữ snapshot yield/time cũ |
| F-03 | Cây trưởng thành 3h trước | reconnect/login | vẫn harvestable, không héo |
| F-04 | Thu lô 120, chỉ còn chỗ 50 | harvest | **120 vào pending**, 0 vào túi |
| F-05 | Hai thu hoạch cùng loại pending 50,70 | commit | một dòng 120, timestamp sớm nhất |
| F-06 | Pending dòng 120, còn chỗ 100 | claim | từ chối toàn bộ (trừ ngoại lệ > max inventory) |
| F-07 | Pending có 3 dòng FIFO, dòng 2 không vừa | claim all | nhận dòng 1, dừng ở dòng 2; dòng 3 không động |
| F-08 | Pending merged > max inventory capacity | claim | nhận tối đa có thể, giữ phần còn, không mất |
| F-09 | Mature plots [1,2,3], plot 2 overflow | harvest all | xử lý 1,2,3; plot2 pending không chặn plot3 |
| F-10 | Có cả non/mature plants | uproot all confirm | mọi cây bị nhổ; không có confirmation thì reject |
| F-11 | Seed quantity ít hơn số plot trống | plant all confirmed | ID plot tăng dần, không vượt seed |
| F-12 | Pest spawn tại 70% window | deadline 30% full growth | deadline không quá mature_at |
| F-13 | Pest penalty 5..10 | hoàn tất không bắt | `yield-floor(yield*pct/100)` đúng, 0-floor hợp lệ |
| F-14 | Logout/restart game | pest event persisted | không random lại hoặc sinh sâu lần hai |
| F-15 | Farm item 1000 cùng loại | cấp inventory | hai stack tối đa 999 mỗi ô |
| F-16 | Fashion item x2 cùng item code | inventory | 2 instance/2 ô, không stack |
| F-17 | Người chơi bắt sâu nhưng túi đầy | catch | **BLOCK until rule clarified**; không xóa hoặc bịa behavior |

## 4. NPC + assets + concurrency

| ID | Given | When | Then |
|---|---|---|---|
| T-01 | 500 KC | mua item 100 KC | còn 400 KC + đúng 1 item + ledger đầy đủ |
| T-02 | request_id X | gửi 100 concurrent identical calls | chỉ execute một lần; same result |
| T-03 | cùng request_id X payload khác | gửi lại | `IDEMPOTENCY_CONFLICT` |
| T-04 | DB crash giữa debit và item grant | restore | cả hai commit hoặc cả hai rollback, không nửa chừng |
| T-05 | Postgres standby không confirm durability | mua NPC | `ASSET_WRITES_DISABLED`, không báo success |
| T-06 | Admin giá mới ở draft | NPC buy | server lấy giá published cũ |
| T-07 | Admin thay giá qua bảo trì | bán crop trồng trước | giá bán mới published, không đổi yield snapshot |
| T-08 | Nông sản còn trong pending | bán NPC | từ chối, yêu cầu claim vào bag |
| T-09 | Admin 1 tạo adjustment | chính Admin 1 approve | từ chối kể cả Super Admin |
| T-10 | Admin 1 + Admin 2 đủ quyền | approve | đúng một commit, ledger/audit đầy đủ |
| T-11 | Giá NPC đổi giữa 2 requests | một giao dịch đã commit | không hoàn/sửa giao dịch cũ |

## 5. Chat / privacy / social

| ID | Given | When | Then |
|---|---|---|---|
| S-01 | Player A cùng room với B | chat public | B nhận; phòng khác không nhận |
| S-02 | Bạn bè đúng 100 | gửi accept friend 101 | Server từ chối không vượt cap |
| S-03 | PM đầu tiên | mở hội thoại | 50 tin mới nhất |
| S-04 | PM >50 | cuộn lên | nạp thêm 20 và giữ đúng vị trí viewport |
| S-05 | Tin chưa vào viewport | người nhận mở chat | chưa tự đánh seen tin đó |
| S-06 | PM 30d +1s | query | không xuất hiện, không phục hồi khi unblock |
| S-07 | A chặn B | B nhắn A | lỗi chung không tiết lộ block |
| S-08 | Block list 55 người | page load | 50 + 20, mới chặn trước |
| S-09 | Tên có dấu | block list search bỏ dấu | tìm được theo quy tắc search; ID phải chính xác |
| S-10 | Chat sending timeout | retry cùng request_id | không phát 2 messages |

## 6. Vận hành / Beta

| ID | Case | Expected |
|---|---|---|
| O-01 | Restore PITR | wallets/items/ledger matched, no duplicates; admin audit available |
| O-02 | One World server down | healthy servers still serve; replacement rooms recovered |
| O-03 | Database primary fails | failover only to eligible standby, no split-brain |
| O-04 | Reset Closed Beta | 0 player data, Admin/Config retained, backup exists |
| O-05 | Reset requested against Open Beta | hard-block, cannot execute |
| O-06 | Config publish during no maintenance | reject/queue until maintenance |
| O-07 | 300 active players mixed workloads | load report with latency/error/CPU/RAM/DB locks; gate PASS only after measured |
| O-08 | Security review | no known critical auth/asset exploit, TLS/WSS, RBAC/TOTP and DDoS layers |

## 7. Gating

- **Unit**: logic farm yield/pest, Unicode names, `pending` FIFO, transaction atomic invariants.
- **Integration**: PostgreSQL transaction/retries, Go services, WS state synchronization, Redis outage fallback.
- **E2E**: mobile landscape, PC keyboard/mouse, full cycle NPC buy→plant→pest→harvest→sell→claim.
- **Stress**: 300 CCU (not 300 accounts) with realistic movement/chat/farm/NPC events and reconnect storms.
- **Recovery**: actual PITR restore and failover test with deliberately interrupted writes.
- **Release decision**: No P0/critical data integrity/account compromise vulnerabilities; test reports stored with commit ID.

**Không tô PASS cho các test chưa thực thi.** Gói tài liệu chỉ định tiêu chuẩn, không chứng nhận game đã qua QA.
