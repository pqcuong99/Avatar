# PROJECT THANH

## KẾ HOẠCH PHÂN KỲ PHÁT TRIỂN — MVP / V1 / SAU V1

**Mã tài liệu:** TH-ROADMAP-01  
**Phiên bản:** 0.1 — đề xuất để chủ dự án phê duyệt  
**Ngày:** 19/10/2026  
**Đầu vào:** `Project_Thanh_GDD_v1.0_2026-10-19.md` và báo cáo kiểm kê thiết kế đến câu 1198  
**Trạng thái:** **CHƯA PHÊ DUYỆT PHẠM VI MVP / V1**; không phải kế hoạch thời gian, dự toán nhân lực hay đặc tả lập trình.

> **Lưu ý nền tảng:** Mọi lựa chọn **chia giai đoạn** trong tài liệu này là **[ĐX] ĐỀ XUẤT**, kể cả khi tính năng liên quan đã **[C] CHỐT** trong GDD. Việc chuyển một tính năng sang giai đoạn sau chỉ là **hoãn thời điểm triển khai**, không xóa/sửa quy tắc sản phẩm. Nguồn còn thiếu bản gốc liên tục câu 1–1198, nhất là đoạn 1–660 và 798–959; nội dung chưa xác minh ký hiệu **[V]**.

---

# I. NGUYÊN TẮC CHIA PHIÊN BẢN

1. **MVP = bản thử nội bộ có vòng chơi thật, không phải phiên bản phát hành công khai.** Mục tiêu xác nhận dữ liệu và vòng chơi: định danh nhân vật → vào map → tương tác NPC → trồng/thu hoạch → túi/chờ nhận → bán nông sản → nhận/tiêu Xu → gặp nhân vật khác và mở Profile sơ bộ. Đây là **[ĐX]**, không phải vòng chơi đã được chủ dự án chốt.
2. **V1 = mục tiêu đủ chức năng cốt lõi để xét beta công khai, không đồng nghĩa sẵn sàng phát hành ngay.** Tích hợp trọn bộ bạn bè, Chat riêng và chặn Chat riêng; chợ; bang; BXH. Phải qua kiểm thử, bổ sung nội dung và duyệt chính sách vận hành.
3. **Sau V1 = nội dung nâng cao, không xóa khỏi GDD.** Mini-game/sự kiện phức tạp, mở rộng câu cá, khu vực, tự động hóa sâu, Editor nâng cao, mỹ thuật phụ.
4. **Tính năng nào được bật phải giữ toàn bộ ràng buộc đang áp dụng.** Không dùng nhãn “MVP” để bỏ cơ chế chờ nhận, quy tắc tràn Xu, giao dịch server, quy tắc bảo trì, hoặc đơn giản hóa sai hành vi Chat riêng nếu Chat riêng được mở.
5. **Tầng an toàn chạy xuyên mọi giai đoạn.** TH-04 (bảo trì/version), TH-12 (tài sản), TH-14 (định danh/quản trị), TH-16 (server), TH-17 (QA) không thể dời toàn bộ sang V1.
6. **Không ấn định số NPC/map/cây, nền tảng, thời hạn, số người chơi hay giá khi nguồn chưa có.** Các con số và lựa chọn cụ thể phải hình thành trong master data/technical spec sau khi đối chiếu.

### Chú giải phạm vi

- **[C]** = quy tắc sản phẩm đã có trong GDD v1.0 (không có nghĩa phân hệ đã Ready for Dev).
- **[ĐX]** = cách đóng gói/ưu tiên phiên bản do tài liệu này đề xuất, **chưa phê duyệt**.
- **[V]** = thiếu nguồn hoặc chưa xác minh thông số/luồng.
- **[GATE]** = điều kiện bắt buộc phải đáp ứng trước khi cho giai đoạn tương ứng ra dùng.
- **Phụ thuộc kỹ thuật** = cần hợp đồng dữ liệu/API chung; không nhất thiết toàn bộ phân hệ phía trước phải hoàn thiện 100%.

---

# II. MA TRẬN PHÂN KỲ ĐỦ 17 PHÂN HỆ

| Mã | Phân hệ | MVP nội bộ [ĐX] | V1 [ĐX] | Sau V1 [ĐX] |
|---|---|---|---|---|
| TH-01 | Nông trại, cây trồng, Auto Farm | Vòng trồng, nhổ, thu hoạch cốt lõi; tăng trưởng xuyên bảo trì; nhận nông sản an toàn | Thao tác hàng loạt và Auto Farm theo các quy tắc được xác minh | Nâng cao tự động hóa, thêm cây/hoạt động |
| TH-02 | Túi đồ, stack, thời trang, chờ nhận | Túi 10 ô, nâng +5, stack 999 đúng loại, hàng chờ khi đầy, ledger item | Trang phục/đồ vĩnh viễn để giao dịch; khóa/không khóa theo đúng loại item | Bộ lọc, tiện ích sắp xếp/kho mở rộng nếu có quyết định |
| TH-03 | NPC, mua bán, hội thoại, Map Editor | NPC cố định không chồng; không đi xuyên; menu trái/nội dung phải; mua hạt/bán nông sản bằng server | Thêm vai trò NPC, hội thoại, nhiệm vụ, dữ liệu hoạt động | Map Editor/template quản trị nâng cao nếu cần |
| TH-04 | Bảo trì, giá NPC, thông báo | Version và khóa áp dụng qua bảo trì; báo trước; an toàn cập nhật/rollback; thông báo không tự thay khi đang mở | Công cụ vận hành hoàn chỉnh, quy trình duyệt/công bố và audit sâu | Tự động hóa vận hành bổ sung |
| TH-05 | Chat riêng | **Chưa mở cho người chơi**; chỉ dự trù mô hình user-ID/server | **Toàn bộ Chat riêng đã chốt:** 30 ngày/tin, 50+20 tin, viewport đọc, seen, typing, nháp phiên, gửi xác nhận | Nâng chất lượng/vận hành theo dữ liệu tải thực tế, không tự đổi quy tắc |
| TH-06 | Chặn/bỏ chặn Chat riêng | Chưa mở vì chưa bật Chat riêng | Mở **cùng Chat riêng**, chặn một chiều/kín đáo; 4 đường bỏ chặn cần đối chiếu; danh sách vô hạn và tải 50+20 | Cải thiện hiệu năng/khả năng quản lý theo quy tắc |
| TH-07 | Bạn bè, Profile, Online | Chọn người chơi và mở Profile cơ bản để thử tương tác map | Kết bạn thủ công, trần 100, lời mời, Online trong Chat realtime, tích hợp Chat/Block | Tính năng cộng đồng nâng cao nếu có nguồn |
| TH-08 | Chợ người chơi | **Chưa mở giao dịch P2P**; giữ cấu trúc item và mã giao dịch để tương thích | Niêm yết/mua bán **thời trang vĩnh viễn không khóa**, mỗi tin một món, dùng Xu, không bán hạt/nông sản | Mở rộng UX/quản trị không làm thay đổi danh mục item được phép |
| TH-09 | Bang hội, nhiệm vụ bang | Chưa mở | Bang, nhiệm vụ riêng 10/ngày, tự nhận tối đa 5/ngày, góp quỹ và mở item theo thông số được xác minh | Hoạt động bang mới/bang chiến nếu đối chiếu xác nhận có |
| TH-10 | Danh hiệu, thành tích, BXH | Lưu số dư/timestamp chính xác để hỗ trợ xếp hạng sau | BXH Xu/Kim Cương theo **số dư**, Top 50, danh hiệu định kỳ theo quy tắc đã xác minh | Thành tích chuyên môn/sự kiện mới; thành tích câu cá khi mở câu cá |
| TH-11 | Map, khu vực, điều hướng | Map đủ chơi thử; spawn, di chuyển, va chạm/tìm đường tránh NPC, chọn avatar | Mở thêm khu vực, chuyển map, nhiều người trên map và UX đồng bộ tốt hơn | Mở rộng thế giới/kênh theo tải thực tế |
| TH-12 | Xu, Kim Cương, kinh tế | Ví Xu, giới hạn/tràn Xu theo nguồn gốc sau khi đối chiếu, giao dịch NPC và nhật ký Xu; mô hình Kim Cương nếu cần từ đầu | Đưa Kim Cương vào các hoạt động đã được duyệt; kinh tế chợ/bang/BXH | Cân bằng, nguồn/sink và nội dung kinh tế bổ sung thông qua bảo trì |
| TH-13 | Câu cá, mini-game, sự kiện | Chưa mở mini-game thu phí | Chưa bắt buộc để xét V1; chỉ chuẩn bị API sự kiện/thưởng nếu phụ thuộc | Câu cá, mini-game, đăng ký “Sẽ tham gia”, các sự kiện; **hủy do bảo trì hoàn 100% phí** |
| TH-14 | Tài khoản, ban, quản trị vi phạm | Định danh/đăng nhập, quyền vận hành tối thiểu, ban kèm **lý do Admin nhập nếu ban được bật** | Quản trị vi phạm và khôi phục/khiếu nại theo đặc tả đã đối chiếu | Bộ công cụ hỗ trợ/nâng cao |
| TH-15 | Giao diện, tài nguyên | UI đủ hành trình MVP, assets tối thiểu, tương tác NPC theo bố cục đã chốt | UI/UX đầy đủ cho Chat, chợ, bang, BXH và nền tảng được lựa chọn | Asset/animation/âm thanh và hiệu ứng mở rộng |
| TH-16 | Backend, database, chống gian lận | Server-authoritative, giao dịch nguyên tử, idempotency, logs, backups, reconnect cơ bản, cấu hình có version | Realtime chat/presence, chợ, nhiệm vụ bang, ranking, monitoring/load protection | Tối ưu scale, dashboard, anti-cheat chuyên sâu |
| TH-17 | QA, vận hành, phát hành | Test vòng chơi, túi đầy, đồng thời, Xu tràn, giao dịch/bảo trì/restore | Regression toàn game, load/stress, an toàn và beta gate | Theo dõi live, mở rộng thử nghiệm theo tính năng |

**Kết luận ma trận:** 11 phân hệ có **lõi kỹ thuật hoặc tính năng xuất hiện tại MVP** (TH-01/02/03/04/07/11/12/14/15/16/17); 5 phân hệ xã hội và kinh tế mở rộng (**TH-05/06/08/09/10**) đề xuất chủ yếu cho V1; **TH-13** đề xuất triển khai chức năng người chơi sau V1. Tất cả 17 phân hệ vẫn có chỗ trong kế hoạch.

---

# III. PHẠM VI NGHIỆP VỤ VÀ RANH GIỚI KHÔNG CẮT QUY TẮC

## TH-01 — Farm [ĐX MVP lõi; V1 Auto Farm]

- **MVP:** Một luồng đầy đủ trồng → thời gian tăng trưởng → thu hoạch/nhổ → đưa item vào túi hoặc chờ nhận khi đầy → bán qua NPC. Bảo trì không làm biến mất hoặc làm sai tăng trưởng cây [C: TH-01-001/003/006].
- **V1:** Chỉ mở thao tác hàng loạt/Auto Farm sau khi đối chiếu điều kiện, giới hạn và ưu tiên [V: TH-01-002].
- **Không thay đổi:** Hạt giống/nông sản không giao dịch P2P và không phân loại khóa/không khóa [C: TH-01-004/005].

## TH-02 — Inventory [ĐX MVP bắt buộc]

- **MVP:** Túi mở đầu 10 ô; nâng +5/lần; farm/nâng cấp 999/stack; thời trang nếu đã tồn tại là một món riêng; nhận chờ bảo toàn tài sản [C: TH-02-001…005].
- **V1:** Thực hiện đường đi item thời trang không khóa và vĩnh viễn vào chợ. Không suy ra mọi item không khóa đều được bán trên chợ [C: TH-08-001].
- **Cần đối chiếu [V]:** giá và trần nâng túi; thời hạn/thứ tự hàng chờ; chủng loại item khởi tạo; cơ chế stack khi vượt giới hạn.

## TH-03 — NPC [ĐX MVP bắt buộc]

- **MVP:** NPC thu mua nông sản và NPC bán hạt (có thể một NPC nhiều dịch vụ **nếu dữ liệu thiết kế sau đối chiếu cho phép**), menu trái/nội dung phải; NPC cố định, không chồng và có collider. Cập nhật giá qua bảo trì [C: TH-03-001…005/007].
- **V1:** thêm chức năng NPC khác theo catalog thực tế; phát triển dữ liệu hội thoại/nghiệp vụ.
- **Sau V1:** công cụ Editor nâng cao [ĐX], **không** hoãn việc lưu dữ liệu NPC có version cần thiết cho MVP.

## TH-04 — Bảo trì [ĐX nền tảng ngay MVP]

- **MVP:** Admin thay đổi trong bản nháp; bản mới chỉ có hiệu lực **sau bảo trì**, thông báo trước, snapshot giá giao dịch, rollback an toàn [C: TH-04-001…004]. Cách làm bản nháp/snapshot/rollback là **[ĐX kỹ thuật]**, không phải lựa chọn UI của chủ dự án.
- **V1:** công cụ duyệt, audit, lịch phát hành và hỗ trợ khôi phục phức tạp.
- **Cần đối chiếu [V]:** ngưỡng thông báo trước; ranh giới giao dịch cố thực hiện sát giờ bảo trì, không tự mở rộng luật “không hoàn” sang trường hợp hệ thống làm lỗi.

## TH-05 + TH-06 — Chat riêng và Block [ĐX V1 phát hành cùng nhau]

- **MVP:** chỉ định nghĩa user-ID và endpoint/hợp đồng tương thích; **không đưa vào UI sử dụng** để tránh bản Chat riêng bị thiếu cơ chế bảo vệ.
- **V1:** Chat riêng 30 ngày theo từng tin, tải 50/20, không tự đánh dấu đọc ngoài viewport, seen/typing, nháp trong phiên, server-confirmed; Block một chiều kín đáo, 4 đường bỏ chặn (cần nguồn để xác minh), không giới hạn, 50/20, tìm kiếm tên không phân biệt dấu/hoa-thường hoặc ID đầy đủ [C: TH-05, TH-06].
- **Điều kiện bật [GATE]:** TH-05 và TH-06 đã tích hợp cùng chính sách riêng tư, reconnect, kiểm thử truy cập trái phép; **không** mở Chat riêng công khai trước khi Block sẵn sàng.

## TH-07 — Profile và bạn bè [ĐX MVP một phần, V1 đầy đủ]

- **MVP:** avatar có ID, có thể chọn người khác trên map và mở Profile sơ bộ. Không tự coi Profile sơ bộ là hoàn tất chức năng bạn bè.
- **V1:** kết bạn thủ công, trần 100, lời mời và tình huống đồng thời; Online trong Chat cập nhật realtime [C: TH-07-001…004].
- **Cần đối chiếu [V]:** tập trường Profile, quyền riêng tư/hiển thị, trạng thái lời mời và tác động của block đối với quan hệ bạn bè.

## TH-08 — Chợ [ĐX V1]

- **MVP:** item schema có trạng thái phù hợp, chưa cho giao dịch P2P.
- **V1:** chỉ thời trang **vĩnh viễn, không khóa**, một món/tin, thanh toán Xu; xem item, giá, người bán và mở Profile; giữ item, chuyển quyền sở hữu và Xu an toàn [C: TH-08-001…006].
- **Cần đối chiếu [V]:** phí, thời hạn, hủy, hoàn, giữ hàng và giá giới hạn trước khi bật.

## TH-09 — Bang [ĐX V1]

- **V1:** lõi bang, 10 nhiệm vụ ngẫu nhiên **riêng mỗi thành viên mỗi ngày**, tự nhận tối đa 5, góp quỹ và mở khóa item riêng cho bang [C: TH-09-001/002/005].
- **Điều kiện bật [GATE]:** xác minh quyền/role, reset nhiệm vụ, thưởng và nội dung nhiệm vụ; cách hiểu câu “phần thưởng ... giống nhau theo từng nhiệm vụ” cần đối chiếu [V: TH-09-004].
- **Sau V1:** hoạt động bang mới/bang chiến **nếu** có nguồn xác nhận; chưa được tự xem là tính năng đã chốt.

## TH-10 — BXH, danh hiệu [ĐX V1]

- **MVP:** lưu giao dịch/số dư đủ tin cậy; chưa cần hiển thị BXH cho người chơi.
- **V1:** Xu/Kim Cương theo số dư, Top 50, chốt danh hiệu tháng khi chu kỳ cụ thể đã được xác minh [C: TH-10-001/002; V: TH-10-003].
- **Sau V1:** thành tích câu cá khi mở cơ chế câu cá; tránh màn hình thành tích trống không có nguồn gameplay.

## TH-11 — Map, di chuyển [ĐX MVP]

- **MVP:** khu vực/màn hình tối thiểu để chứng minh di chuyển, spawn hợp lệ, NPC collider, tự tìm đường và gặp avatar người chơi khác [C: TH-11-001…003].
- **V1:** kết nối khu vực, capacity/kênh nếu catalog yêu cầu.
- **Cần đối chiếu [V]:** danh sách map, điều khiển và nền tảng PC/mobile, giới hạn người/phòng; không tự chọn nền tảng chỉ từ ví dụ UI.

## TH-12 — Tiền tệ [ĐX lõi Xu MVP; mở rộng V1]

- **MVP:** giao dịch Xu từ mua/bán NPC, số dư, trần Xu, xử lý tràn theo **nguồn** đã xác minh và tách quy tắc hoàn tiền khi giao dịch lỗi [C/P: TH-12-001…004].
- **V1:** Kim Cương và nguồn/sink liên quan bảng xếp hạng, chợ/bang đã đủ spec.
- **Không thay đổi:** phần Xu vượt trần trong hoạt động thông thường có thể mất vĩnh viễn không cảnh báo; không tự từ chối giao dịch hay trả nông sản [P: TH-12-003].
- **Gate:** phải xác minh **giá trị trần, từng nguồn nhận Xu, ngoại lệ hoàn/rollback** trước giao dịch tiền tệ thật, kể cả MVP nội bộ.

## TH-13 — Câu cá/mini-game/sự kiện [ĐX sau V1]

- **V1:** chỉ dự trù compatibility nếu cần cho thành tích sau này, không mở game thu phí chưa có thông số.
- **Sau V1:** câu cá, mini-game, đăng ký tham gia sự kiện, danh sách “Sẽ tham gia” sau khi khôi phục bối cảnh tính năng [P/V: TH-13-001/003/004].
- **Gate mở mini-game:** nếu hủy do bảo trì phải **hoàn đúng 100% phí** [C: TH-13-002], có cơ chế chống trả hai lần.

## TH-14 — Định danh và quản trị vi phạm [ĐX lớp nền MVP]

- **MVP:** tài khoản/nhân vật ổn định, session/role quản trị tối thiểu; nếu bật ban thì lý do người chơi thấy phải là lý do Admin nhập [P: TH-14-001].
- **V1:** khôi phục tài khoản, khiếu nại, nhật ký thao tác và quản trị vi phạm theo nội dung được xác minh.
- **Gate:** phương thức đăng ký/đăng nhập, quyền Admin và chính sách ban chưa có đặc tả đủ tin cậy [V: TH-14-003].

## TH-15 — UX và tài nguyên [ĐX xây theo mỗi luồng]

- **MVP:** UI tạo/đăng nhập, map, farm, túi, NPC và thông báo bảo trì, đủ thao tác end-to-end.
- **V1:** Profile/bạn bè, Chat/Block, chợ, bang, BXH, giao diện thiết bị đã xác định; NPC đa chức năng vẫn menu trái/nội dung phải trên mobile nếu hỗ trợ [C: INV-03].
- **Sau V1:** mỹ thuật, animation, audio mở rộng theo độ ưu tiên thực tế; không thay thế UX gameplay đang chạy.

## TH-16 — Backend và bảo mật [ĐX xuyên mọi đợt]

- **MVP:** dữ liệu bền vững, server là nguồn xác nhận; giao dịch idempotent, audit Xu/item, giao dịch nguyên tử, snapshot giá, session, backup/restore/reconnect tối thiểu [C: INV-10 và các yêu cầu bảo toàn; kỹ thuật chi tiết là ĐX].
- **V1:** chat/presence/sự kiện theo thời gian thực, giao dịch marketplace, reset bang/BXH, giám sát và phân tải.
- **Sau V1:** scale và phòng lạm dụng nâng cao theo chỉ số thực; không hoãn biện pháp an toàn thiết yếu.

## TH-17 — QA, vận hành [ĐX cổng chất lượng xuyên mọi đợt]

- **MVP:** test giao dịch lặp, mất mạng, túi đầy, Xu vượt trần, bảo trì rollback, di chuyển/NPC và đồng bộ người chơi.
- **V1:** bổ sung test Chat/Block riêng tư, chợ mua đồng thời, bang reset 10/5, Top 50, an toàn vận hành, load/stress.
- **Sau V1:** mini-game hoàn phí, event đăng ký, tăng tải, live operations và kiểm thử hồi quy liên tục.

---

# IV. SƠ ĐỒ PHỤ THUỘC (THỨ TỰ KỸ THUẬT, KHÔNG PHẢI PHÂN CẤP GIÁ TRỊ)

```text
MÓNG DỮ LIỆU / HẠ TẦNG
TH-16 Backend ───────┬── TH-14 Tài khoản/định danh
                      ├── TH-02 Túi/item/chờ nhận
                      ├── TH-12 Xu/ledger/giới hạn
                      └── TH-04 Version/maintenance/rollback
                                   │
TH-15 UI nền ── TH-11 Map & điều hướng ──┐
                                         ├─ TH-03 NPC / mua-bán / collider
TH-02 + TH-12 + TH-04 ───────────────────┘           │
                                                     └─ TH-01 Farm → harvest → inventory → NPC → Xu
TH-17 QA bám tất cả nút và mũi tên ở trên.

MỞ RỘNG V1
TH-11 + TH-14 + TH-16 ─────────→ TH-07 Profile/bạn bè
TH-07 + TH-16 + TH-14 ─────────→ TH-05 Chat riêng + TH-06 Block (BẬT CÙNG LÚC)
TH-02 + TH-12 + TH-07 + TH-16 → TH-08 Chợ
TH-02 + TH-07 + TH-12 + TH-16 → TH-09 Bang
TH-12 + TH-16 ────────────────→ TH-10 BXH/danh hiệu

SAU V1
TH-04 + TH-12 + TH-16 (+ TH-10 nếu có thành tích) → TH-13 Câu cá/mini-game/sự kiện
```

**Ghi chú về phụ thuộc vòng:** Trong GDD, Farm/Túi/NPC/Tiền có phụ thuộc hai chiều về **nghiệp vụ**. Để tránh vòng khi lập trình, triển khai trước **hợp đồng dữ liệu** và **service nhỏ nhất** của TH-02/12/16; sau đó tích hợp từng luồng TH-03 và TH-01. Cũng tương tự, Chat và Block cần thiết kế hợp đồng cùng nhau nhưng không nhất thiết viết tất cả UI một lần.

---

# V. THỨ TỰ LẬP TRÌNH ĐỀ XUẤT — KHÔNG GẮN NGÀY/TIẾN ĐỘ GIẢ ĐỊNH

| Làn công việc | Thành phần | Kết quả đo được | Điều kiện qua cổng |
|---|---|---|---|
| **G0 — Chuẩn hóa đầu vào** | Đối chiếu GDD P0; master data cơ bản; chọn nền tảng; phê duyệt phạm vi | Backlog có ID, danh mục gameplay cơ bản, sơ đồ màn hình và ma trận ngoại lệ Xu | Không để quy tắc mơ hồ được mã hóa như luật đã chốt |
| **G1 — Khung backend an toàn** | TH-16 + TH-14 + lớp TH-02/12/04 | Tạo/lưu user, item, số dư, transaction ID, data version; backup và restore thử nghiệm | Giao dịch lặp không cộng/trừ hai lần; không báo thành công giả |
| **G2 — Thế giới và tương tác** | TH-11 + TH-15 + TH-03 lõi | Vào map, di chuyển, tránh NPC, mở menu trái/nội dung phải | Không chồng NPC, không đi xuyên collider; giá không đổi tức thời |
| **G3 — Vòng kinh tế nông trại** | TH-01 + TH-02 + TH-03 + TH-12 | Trồng → thu → nhận item → NPC thu mua → nhận Xu; nâng túi | Túi đầy không mất item; Xu tràn theo quy tắc đã xác minh; retry an toàn |
| **G4 — Maintenance & alpha gate** | TH-04 + TH-17 + TH-14 vận hành | Bảo trì báo trước, version mới, snapshot giá, audit, rollback và một bản MVP nội bộ chạy được | Đạt kiểm thử bảo toàn tài sản + lỗi mạng + hồi phục; duyệt giới hạn người thử |
| **G5 — Xã hội đầy đủ** | TH-07 + TH-05 + TH-06 | Kết bạn, chat riêng, block, đọc/đã xem/typing, phân trang | Block kín đáo hoạt động; Chat không ra mắt thiếu Block; 4 đường bỏ chặn đã xác minh |
| **G6 — Chợ an toàn** | TH-08 + TH-02 + TH-12 + TH-16 | Rao bán thời trang hợp lệ, mua, hủy/hoàn theo quy tắc đã đối chiếu | 2 người mua 1 món: chỉ 1 thành công; không mất/nhân đôi tài sản |
| **G7 — Bang và BXH** | TH-09 + TH-10 + TH-12 + TH-16 | 10 nhiệm vụ riêng/ngày, tự nhận ≤5; quỹ bang; BXH Top 50 theo số dư | Chống thưởng trùng, reset và quyền bang hợp lệ |
| **G8 — Beta gate V1** | TH-14/15/16/17 toàn diện | Reconnect, tải đồng thời, quản trị, UI các nền tảng, tài liệu support | Đạt QA an toàn/hiệu năng, có quy trình phản hồi/rollback và chủ dự án duyệt |
| **G9 — Nội dung sau V1** | TH-13 + các mở rộng TH-01/03/11/15 | Câu cá/mini-game/event và nội dung bổ sung | Mini-game hủy do bảo trì hoàn 100%; không làm hỏng các flow đã chạy |

**Không được suy ra mốc thời gian từ số G:** G0–G9 là thứ tự ưu tiên tích hợp, một số việc có thể chạy song song sau khi hợp đồng dữ liệu được chốt. Không có ước lượng ngày/tuần, nhân lực hay ngân sách được xác minh.

---

# VI. CÁC PHỤ THUỘC CHẶN TRIỂN KHAI VÀ NHÓM CẦN QUYẾT ĐỊNH

## P0 — Phải chốt trước khi coi MVP đủ phạm vi, hoặc trước chức năng tương ứng đi vào code chuẩn

| ID gốc | Phải có | Tác động trực tiếp | Trạng thái |
|---|---|---|---|
| GAP-P0-01 | Nguồn câu hỏi/câu trả lời đầy đủ, ưu tiên các đoạn thiếu | Tránh áp sai quy tắc cũ/mới trong mọi phân hệ | **[V]** chưa có bản gốc |
| GAP-P0-02 | Quyết định phạm vi MVP/V1 và nền tảng thử đầu | Toàn bộ backlog/UI/QA | **[ĐX]** bản này đang đề xuất |
| GAP-P0-03 | Bảng cây/hạt/giá/năng suất/thời gian và giá nâng túi | TH-01/02/03/12 | **[V]** chưa có dữ liệu số |
| GAP-P0-04 | Ma trận trần Xu/tràn Xu/hoàn tiền theo nguồn | TH-03/12/16; rủi ro mất tài sản | **[V]** chưa khóa ngoại lệ |
| GAP-P0-05 | Contract giao dịch idempotent, item chờ nhận, xử lý race | TH-01/02/03/12/16 | **[ĐX kỹ thuật]** cần thiết kế và test |
| GAP-P0-06 | Danh mục map/NPC/item/quest/đồ tối thiểu có ID/version | TH-01/03/11/15 | **[V]** thiếu master catalog |
| GAP-P0-07 | Client PC/mobile ưu tiên, realtime, reconnect, Online | TH-07/11/15/16 | **[V]** chưa có lựa chọn nền tảng |
| GAP-P0-08 | Bảo trì, audit, staging và rollback | TH-03/04/12/14/16/17 | **[ĐX kỹ thuật]** chưa được duyệt |

**Quy tắc làm việc:** Có thể dựng giao diện, mock và bộ khung để thử sớm; không được coi logic Xu/item đã chuẩn hoặc cho người chơi thật giao dịch khi các rủi ro dữ liệu/giao dịch ở trên chưa được giải quyết.

## P1 — Trước khi công bố beta V1

| Nhóm | Tính năng đang chặn | Cần hoàn tất |
|---|---|---|
| Chợ | TH-08 | Quy trình niêm yết, khóa hàng, phí, hết hạn, hủy, hoàn, tranh chấp |
| Bạn bè / Chat / Block | TH-05/06/07 | Bốn đường bỏ chặn, trạng thái yêu cầu, đồng bộ nhiều thiết bị, privacy |
| Bang | TH-09 | Quyền bang, reset/ngày, mốc quỹ và nghĩa đầy đủ của luật thưởng |
| UI & nội dung | TH-11/15 | Bộ màn hình, asset, điều khiển nền tảng, mục tiêu máy tối thiểu |
| Quản trị | TH-14/04 | Lý do cấm, phân quyền, logs, quyền kháng nghị, phục hồi |
| Hiệu năng | TH-16/17 | Chỉ số mục tiêu phải được chốt; đo tải/map/chat/chợ/bang/BXH thực tế |

## P2 — Sau V1

Danh mục mini-game/câu cá/sự kiện, cơ chế đăng ký “Sẽ tham gia”, Map Editor/Auto Farm nâng cao và nội dung trang trí chỉ thực hiện khi đã có quyết định nguồn đủ rõ và bản lõi đủ ổn định. **Không đưa các mục thiếu nguồn thành thông số đã phê duyệt.**

---

# VII. TIÊU CHÍ NGHIỆM THU CHO TỪNG CỘT MỐC

## MVP nội bộ [ĐX]

MVP **chỉ được coi là đạt** khi một người thử có thể hoàn thành đúng luồng dưới đây bằng dữ liệu server, không có thao tác giả:

1. Có user/nhân vật hợp lệ, vào khu vực được cấu hình, gặp NPC không chồng vị trí và không đi xuyên NPC.
2. Trồng và thu hoạch nông sản; item tăng đúng một lần; trường hợp túi đầy giữ chờ để nhận tiếp, không mất/nhân đôi.
3. Giao dịch với NPC theo bảng giá phiên bản đã công bố, Xu/item cập nhật nguyên tử; Xu vượt trần theo quyết định được đối chiếu.
4. Nâng túi từ 10 lên 15 ô khi đủ điều kiện; mọi stack farm/nâng cấp tối đa 999 trên một ô.
5. Mở Profile cơ bản của một người chơi khác mà không bị nhầm nhân vật.
6. Tình huống mất mạng/retry và bảo trì/rollback không gây trạng thái hiển thị thành công giả hoặc mất tài sản.
7. Quyền Admin và nhật ký giao dịch căn bản được kiểm tra; nếu có ban, hiện lý do do Admin nhập.

**MVP không bắt buộc** có Chat riêng, chặn, chợ, bang, BXH, mini-game hay sự kiện vì các tính năng này chưa được bật ở giai đoạn MVP theo đề xuất.

## V1 trước beta công khai [ĐX]

Ngoài việc **hồi quy toàn bộ tiêu chí MVP**, V1 cần đạt: Chat riêng/Block đúng toàn bộ quy tắc C; kết bạn tối đa 100; chợ không bán vật phẩm sai loại và mua đồng thời an toàn; bang có 10 nhiệm vụ cá nhân/ngày và tự nhận tối đa 5; BXH Xu/Kim Cương theo số dư, Top 50; kiểm thử tải, bảo mật, quản trị vi phạm, kế hoạch backup/rollback. Mọi lỗ hổng privacy/mất tài sản là điều kiện chặn mở beta, không được chấp nhận “để sửa sau”.

## Sau V1 [ĐX]

Mỗi mini-game được phát hành phải có bảng phí/thưởng/ràng buộc riêng và test trường hợp bảo trì hủy: hoàn **đúng 100% phí**; các hệ thống mới không được sửa lén các quy tắc cũ của MVP/V1.

---

# VIII. RỦI RO LỚN VÀ QUY TẮC QUẢN TRỊ THAY ĐỔI

| Rủi ro | Tác động | Biện pháp trước khi bật tính năng |
|---|---|---|
| Xu tràn bị tiêu hủy không cảnh báo; bán nông sản không nhất thiết hoàn | Khiếu nại/mất kỳ vọng tài sản | Xác minh ngoại lệ theo từng nguồn; test nghiệp vụ đúng quyết định, không tự sửa |
| Túi đầy/hàng chờ nhận | Mất item, nhân đôi, dữ liệu tồn lâu | Schema trạng thái; invariant giữ chờ; test retry/restore |
| Bảo trì áp dụng giá/NPC/bản tin | Lệch phiên bản, tranh chấp giao dịch | Version/snapshot, cổng bảo trì, staging/rollback/audit |
| DM phát hành thiếu Block đầy đủ | Lộ riêng tư, gây phiền nhiễu | G5 là một đợt tích hợp Chat + Block, chưa đủ thì không bật DM |
| Chợ cạnh tranh đồng thời | Bán trùng, âm Xu hoặc mất đồ | Khóa quyền bán và transaction IDs, test 2 người mua |
| Nhiệm vụ bang reset | Nhận thưởng nhiều lần | Khóa theo thành viên/ngày/nhiệm vụ, server clock, test concurrent |
| Danh hiệu/BXH số dư | Lệch hạng/cấp thưởng trùng | Snapshot thời điểm chốt, idempotent ranking jobs |
| Thiếu nguồn 1–1198 | Đọc sai lựa chọn đã chốt | Truy vết mã GDD, phiên bản tài liệu và sổ đối chiếu |

### Kiểm soát phạm vi

- **Thay đổi thứ tự phát hành:** Chủ dự án phê duyệt roadmap phiên bản mới; GDD giữ nguyên.
- **Thay đổi hành vi đã chốt:** Bắt buộc ghi `Change Request` nêu mã quy tắc cũ, nội dung mới, lý do, tác động và phiên bản GDD cập nhật. Không dùng kế hoạch MVP để phủ quyết luật đã chốt.
- **Bổ sung số liệu master data:** Lưu nguồn và version; điều chỉnh NPC/giá/thông báo chỉ có hiệu lực **sau bảo trì**.
- **Chưa đủ nguồn:** Giữ [V], không được điền giá trị đoán mò thành [C].

---

# IX. NHỮNG VIỆC THỰC HIỆN TIẾP SAU TÀI LIỆU NÀY

| Thứ tự | Công việc đầu tiên | Kết quả cần tạo | Chủ sở hữu đề xuất |
|---|---|---|---|
| 1 | Gắn nhãn phạm vi chính thức cho 17 phân hệ | Bản phân kỳ `APPROVED` hoặc danh sách chỉnh sửa cấp hệ thống | Chủ dự án |
| 2 | Đối chiếu phần nguồn thiếu | GDD v1.1 + change log, giữ ID quy tắc | Biên tập GDD/chủ dự án |
| 3 | Chốt *một* nền tảng thử nghiệm đầu và thông số tải mục tiêu | Client/platform brief | Chủ dự án + kỹ thuật |
| 4 | Tạo master data tối thiểu | Danh mục item, cây, NPC, map, bảng giá, thời gian, version | Game design |
| 5 | Chốt chính sách kinh tế Xu và chờ nhận | Ma trận nguồn/sink/overflow/rollback + bộ test | Game design + backend |
| 6 | Viết TDD theo thứ tự G1 → G4 | Database, API, luồng màn hình, giao dịch, lỗi, acceptance | Kỹ thuật |
| 7 | Dựng MVP nội bộ rồi chạy QA gate | Bản chơi thử có vòng farm → NPC → Xu an toàn | Dev + QA |
| 8 | Đặc tả và phát triển theo G5 → G8 | V1 và bộ điều kiện để xét beta công khai | Dev + QA + vận hành |

**Không đặt một chuỗi câu hỏi nhỏ.** Bản phân kỳ này có thể được phê duyệt hoặc sửa **theo nhóm** (MVP / V1 / Sau V1, nền tảng ưu tiên, hoặc lựa chọn đưa Chat riêng vào MVP). Nếu không có phê duyệt, mọi nhãn phiên bản trong tài liệu tiếp tục là **[ĐX]**.

**Nhật ký tài liệu:** v0.1 ngày 19/10/2026 — đề xuất 3 cấp phát triển, 17 phân hệ, các giai đoạn G0–G9 và điều kiện kiểm thử; **chưa phê duyệt**.
