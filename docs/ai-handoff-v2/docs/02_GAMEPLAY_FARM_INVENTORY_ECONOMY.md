# 02 — Luật hệ thống tài sản, túi đồ, nông trại, sâu bệnh, NPC

**Ưu tiên cao:** Tài liệu này mô tả yêu cầu đã chốt, còn kiểu dữ liệu/mã trạng thái trong SQL/API là bản thiết kế đề xuất. Không đơn giản hóa những luật pending đặc biệt.

## 1. Khởi tạo và giới hạn

| Dữ liệu | Giá trị | Nguồn |
|---|---|---|
| Xu tạo nhân vật | 5.000 | P0-030 |
| Kim Cương tạo nhân vật | 0 | P0-031 |
| Trần ví Xu ban đầu | 999.999.999 | P0-035 |
| Trần Kim Cương | Admin cấu hình | P0-033 |
| Ô túi ban đầu | 10 | TH-02/P0 context |
| Mỗi lần nâng cấp túi | +5 ô | TH-02/P0 context |
| Ô đất ban đầu | 6 | P0-068 |
| Mỗi lần mở đất | +1 ô theo ID tăng | P0-069–071 |
| Stack nông sản, vật phẩm nâng cấp, sâu | 999 / ô | TH-02, P0-104 |
| Thời trang | 1 món / ô | TH-02 |

**[CẦN ĐỐI CHIẾU]** Quy tắc Xu vượt trần: GDD cũ ghi khả năng bị tiêu hủy không cảnh báo, chưa đầy đủ trường hợp đặc biệt. Không tự xây logic phá hủy Xu vượt cap khi chưa đối chiếu.

## 2. Túi và inventory

- Túi hiển thị ô có ID ổn định, sức chứa là số ô, không phải tổng số item. Stack tối đa theo loại.
- Item nông trại và hạt giống không giao dịch người chơi và không mang `bound/unbound`; fashion có thể thuộc bound/unbound và từng món riêng. Không tự cho rằng mọi item nâng cấp có thể P2P.
- Tăng ô túi +5, giá/giới hạn tăng do Admin quyết định, **chưa có con số**.
- Khi vật phẩm chưa nhận không thể đặt vào túi, phải giữ pending theo luật nguồn. Không áp dụng một chính sách pending chung cho mọi loại không được chốt.
- Các lệnh item đều có `request_id` chống trùng và được serialize theo nhân vật/kho tài sản.

### Kiểm tra có thể chứa batch N

**[ĐỀ XUẤT thuật toán]** `can_fit(item_type, quantity, inventory_snapshot)` tính tổng vị trí còn dư của stack cùng loại + ô trống tương thích. `stack_limit` không được vượt 999 cho farm/upgrade. Nếu `N > space`, tuyệt đối không tách một lần thu hoạch vào túi/pending; **toàn lô vào pending**.

## 3. Nông trại — danh mục cây [CHỐT]

| cây | ID đề xuất | Giá hạt tham chiếu Xu | Thời gian | Nội dung |
|---|---|---:|---:|---|
| Cà rốt | `carrot` | 100 | 30 phút | MVP |
| Khoai tây | `potato` | 250 | 60 phút | MVP |
| Cà chua | `tomato` | 500 | 120 phút | MVP |
| Ngô | `corn` | 750 | 180 phút | MVP |
| Bắp cải | `cabbage` | 1.000 | 240 phút | MVP |

Giá hạt tham chiếu, Admin **chốt lại trước mở Server**. **Yield, giá bán NPC và xác suất sâu của từng cây phải được Admin điền**; mặc định để trống/không thể mua hoặc trồng khi chưa cấu hình. Các cây V1: dâu tây, bí ngô, dưa hấu, ớt, khoai lang, chưa có thông số — sau MVP.

### Trồng một ô

**Tiền điều kiện**: chủ có plot đã mở, plot trống, còn ≥1 hạt; loại cây có config active.  
**Giao dịch**: khóa plot + inventory, trừ 1 hạt, tạo planting (`planted_at`, `matures_at`, `base_yield_snapshot`, `growth_seconds_snapshot`, `config_version`); commit cùng transaction + audit.  
**Hậu điều kiện**: plot có cây; tên/growth snapshot không bị sửa bởi config bảo trì sau đó.  
**Lỗi**: thiếu hạt, plot đã có cây, plot bị khóa, cấu hình chưa hợp lệ, trùng `request_id` → từ chối không mất item.

### Trồng tất cả

- Chọn **một loại hạt**, yêu cầu xác nhận hiển thị loại/số hạt.
- Duyệt ô trống theo plot ID tăng dần, trồng tới khi hết hạt hoặc hết ô.
- Thứ tự commit, kết quả cụ thể cần hiển thị. **[ĐỀ XUẤT]** thao tác batch nguyên tử cùng `request_id` để chống nửa chừng.
- Trồng một ô không cần hộp xác nhận.

### Cây đủ thời gian và nhổ cây

- Cây trưởng thành **không héo**, không bị tự xóa dù Offline hoặc bảo trì.
- Nhổ cây khi chưa chín hoặc đã chín đều **mất toàn bộ cây/hạt/sản lượng tiềm năng**, không hoàn Xu/nông sản.
- Mọi lần nhổ cây đơn đều xác nhận; nhổ tất cả bao gồm cả cây non và cây chín, có xác nhận.
- Không có tưới nước/phân bón.

### Thu hoạch một ô

- Chỉ cây đủ `matures_at` mới được thu.
- Một planting chỉ được thu hoạch **một lần**; thu xong cây biến mất, plot trống.
- `yield_final = base_yield_snapshot - floor(base_yield_snapshot * pest_penalty_percent/100)` nếu sâu quá hạn, còn không `yield_final = base_yield_snapshot`.
- Nếu `inventory_can_fit(yield_final)`: cấp toàn bộ lô vào túi; nếu không: cấp **toàn bộ lô** vào `pending_crop`.
- Không tự xóa tài sản dù túi đầy; hoàn tất commit atomically.
- Nút Thu hoạch tất cả: **chỉ cây đã chín**, không xác nhận; duyệt plots theo ID tăng, cây không đủ chỗ chuyển toàn lô pending và tiếp tục ô sau.

## 4. Pending nông sản — luật đặc biệt P0-075–085

1. Chỉ nông sản đã thu không đủ chỗ mới ở pending; không được bán NPC khi đang pending.
2. **Không tự động chuyển** vào túi khi có ô trống; người chơi bấm nhận thủ công.
3. Gộp pending **cùng loại nông sản** thành một dòng (`crop_id`, `quantity`), giữ thời điểm pending sớm nhất để sắp FIFO.
4. Mặc định muốn nhận phải chứa **đủ toàn bộ dòng**. Nếu không đủ → không nhận, không thay đổi tồn.
5. Nút **Nhận tất cả** xử lý các dòng theo FIFO, **dừng ở dòng đầu tiên không nhận nguyên vẹn**.
6. Ngoại lệ P0-083: nếu **số lượng dòng gộp lớn hơn sức chứa túi tối đa có thể có** (không chỉ số chỗ trống tức thời), được nhận từng phần. Chọn tối đa số đang chứa được, trừ vào pending. Nút Nhận tất cả gặp ngoại lệ này cũng nhận tối đa rồi dừng.
7. Nếu không thể chứa con nào ở ngoại lệ → giữ nguyên, dừng. Không ghi nhận thành công giả.
8. `pending` hợp nhất phải khóa theo `character_id,crop_id` trong transaction để tránh lost update khi 2 lần thu đồng thời.

**[CẦN CẤU HÌNH]** “sức chứa túi tối đa có thể có” khi Admin chưa chốt giới hạn nâng cấp túi => kỹ thuật phải có hàm config, không gán cứng 10, 999 hoặc số tùy ý. Nếu giới hạn túi không xác định, phải làm rõ trước khi kích hoạt ngoại lệ nhận một phần.

## 5. Sâu bệnh — P0-096–102, P0-156

- Mỗi lần trồng có **tối đa 1 sự kiện sâu**; xác suất sâu khác nhau theo loại cây (Admin config).
- Sự kiện có thể phát sinh ở thời điểm ngẫu nhiên trong **70% đầu** thời gian trồng; hạn bắt sâu là **30% toàn bộ thời gian tăng trưởng** kể từ lúc sâu xuất hiện (không phải 30% phần thời gian còn lại).
- Bắt sâu miễn phí qua nút `Bắt sâu`; được 1 item sâu đưa vào túi theo luật đã chốt, tối đa 999/ô. **[CẦN ĐỐI CHIẾU]** nếu túi không đủ chỗ nhận sâu, cơ chế pending sâu chưa được chọn rõ; không tự xóa hay cấp trùng.
- Khi hết hạn mà không bị bắt, tỷ lệ phạt chọn là một **số nguyên trong {5,6,7,8,9,10}**; trừ `floor(base_yield_snapshot × pct/100)`; `floor=0` chấp nhận.
- Phải lưu **đã phát sinh hay không**, `spawn_at`, `deadline_at`, `penalty_pct`, `caught_at` (nếu có). Không gieo random lại sau reconnect/restore; phải có cơ chế deterministic persist/scheduled processing.
- Chim tự bắt sâu/kho thức ăn **sau MVP**, không cài vào M4; tham chiếu P0-103–133 để mở rộng.

## 6. Kim Cương pending và trần ví

- Giới hạn Kim Cương do Admin cấu hình; nạp tiền thật chưa triển khai MVP.
- Phần **Kim Cương thưởng** vượt cap đưa vào hàng chờ, tách từng entry theo nguồn thưởng; không hết hạn.
- Nhận thủ công, Nhận tất cả theo thời gian FIFO từ cũ nhất; nhận bao nhiêu tới khi đủ chỗ, không tạo quá trần. **[CẦN ĐỐI CHIẾU]** nếu entry lớn hơn chỗ trống, phương án nhận một phần áp dụng cho KC chưa được chốt tương đương pending crop; cần review trước khi code.
- Không gộp pending KC cùng nguồn giống pending nông sản.

## 7. Giao dịch NPC và bảo trì

- NPC collider cố định, không chồng NPC; bản đồ Tiled cùng version trên Client/Go Server.
- NPC nhiều menu: **menu dọc bên trái / cửa sổ bán-hội thoại bên phải**. Click menu đổi nội dung bên phải, **không mở lại cửa sổ**.
- Giá NPC và cấu hình chỉ áp dụng sau đợt bảo trì; `effective_version` do Server quyết định. Đơn đã commit không bị thay đổi bởi bản giá mới.
- Bán nông sản: dùng **giá mới nhất có hiệu lực** tại thời điểm giao dịch, kể cả cây trồng từ cấu hình cũ. Chỉ bán nông sản trong túi.
- Mua: phải có Xu/KC đủ, túi đủ chỗ theo item rules; không cho Client báo giá tùy ý.
- Giao dịch thật luôn Server-authoritative; ghi số dư, item, ledger và idempotency trong 1 PostgreSQL transaction.

## 8. Dữ liệu gốc vs master data chưa chốt

- `master-data/crop_catalog.csv`: có 5 cây, giá tham chiếu và thời gian đã chốt; cột yield, sell, pest **trống có chủ ý**.
- `master-data/economy_initial.csv`: tham số tiền và ô đã chốt, thiếu config giao dịch nâng cấp.
- Không thay số trống bằng `0` hay giá demo một cách im lặng; dùng trạng thái `DRAFT_INCOMPLETE` để block phát hành.
