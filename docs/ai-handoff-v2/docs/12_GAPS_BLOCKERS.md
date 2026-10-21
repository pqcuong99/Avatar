# 12 — Danh sách thiếu dữ liệu và điều kiện phải quyết định

**Đây không phải câu hỏi bắt người dùng phải trả lời ngay.** Chỉ chặn phần code thật sự phụ thuộc. AI có thể viết skeleton, unit tests và schema trước khi có master-data.

| GAP | Mức ưu tiên | Chưa rõ | Nếu không rõ thì làm gì |
|---|---|---|---|
| GAP-01 | CRITICAL | Quy tắc đầy đủ khi Xu vượt wallet cap, đối chiếu GDD cũ | Không triển khai chính sách tiêu hủy Xu tự động |
| GAP-02 | CRITICAL | VPS Win Server 2012 R2 runtime Go mới/Docker không hỗ trợ WSL2 | Không deploy public; dùng local/test host phù hợp |
| GAP-03 | CRITICAL | Hạ tầng HA/multi-host trước Open Beta | Không mở Open Beta không reset khi chưa đủ |
| GAP-04 | HIGH | Năng suất, NPC sale price, tỷ lệ sâu của 5 cây | Giá/cây có NULL, chặn gameplay cần config trong chế độ prod |
| GAP-05 | HIGH | Giá mở ô farm, giá nâng túi, giới hạn nâng tối đa | Xây config API/model; không chọn giá/giới hạn tùy ý |
| GAP-06 | HIGH | Hành vi bắt sâu khi túi đã đầy | Chặn thao tác? giữ pending? cần chủ dự án xác nhận |
| GAP-07 | HIGH | “Max possible inventory” trong ngoại lệ pending crop | Model config để quyết định max potential, không hard-code |
| GAP-08 | MEDIUM | Pending KC: claim partial khi entry > free cap? | Tách flow KC, không copy farm policy |
| GAP-09 | MEDIUM | Tên nhân vật đầu/cuối khoảng trắng, nhiều khoảng trắng liên tục, casefold chi tiết | Test Unicode; hỏi lại trước quy tắc normalization nâng cao |
| GAP-10 | HIGH | Nhiệm vụ ngày: count, chọn ngẫu nhiên, thưởng, reset timezone | Chỉ làm engine/config, chưa seed default rewards |
| GAP-11 | MEDIUM | Chat public retention, ký tự tối đa, emoji/file, public moderation | Configurable limits, không invent retention |
| GAP-12 | MEDIUM | PM 4 cách bỏ chặn chính xác, luật một số state | Tạm chỉ triển khai cách đơn giản có bằng chứng, không nhận là đủ |
| GAP-13 | HIGH | Danh mục/Tiled vị trí NPC, sprite map/character/fashion và license | Chờ bộ asset hợp lệ; placeholder trong dev được ghi là sample |
| GAP-14 | MEDIUM | Fashion equip slot, mặc có giữ ô bag hay không | Chọn thiết kế tạm và đánh nhãn cần UX review |
| GAP-15 | MEDIUM | WS Protobuf schema, tick rate, room cap | Load test/benchmark; không chốt giá trị production tùy ý |
| GAP-16 | HIGH | Bảo vệ account/Admin TOTP secret, email recovery service | M1 hoàn thiện security trước Internet |
| GAP-17 | MEDIUM | PM seen theo viewport nhiều browser, merge seen events | Thực hiện test S-05, cần định nghĩa cụ thể edge case |
| GAP-18 | MEDIUM | Chính sách lưu log, backup retention, RPO/RTO, DDoS vendor | Vận hành review trước Closed/Open Beta |
| GAP-19 | MEDIUM | Danh sách map/khu vực MVP, gateway transitions, spawn points | Xây Tiled map registry/config, không bịa map cuối |
| GAP-20 | HIGH | Bản nguồn nguyên văn 1–1198 chưa có, đặc biệt rule cũ thiếu ngữ cảnh | Đối chiếu bản gốc nếu chủ dự án cung cấp, không suy từ A/B đơn lẻ |
| GAP-21 | HIGH | Tình trạng thật `Avatar` repo và patch M1 được merge? | `git status`, inspect tree; không được ghi đè blindly |
| GAP-22 | HIGH | Người chơi Beta có được khuyến khích tạo password thật? | Cần cảnh báo về môi trường cũ; không thu thập mật khẩu đang dùng dịch vụ khác |

**Kiểm tra mỗi lần AI làm module**: đối chiếu GAP của module, nêu những điều không thể quyết định chỉ từ nguồn. Không làm ngưng toàn dự án vì thiếu một con số giá khi có thể triển khai khung cấu hình.
