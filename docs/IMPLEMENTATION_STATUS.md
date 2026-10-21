# Project Thanh — Trạng thái hiện thực MVP

**Cập nhật:** 21/10/2026 · **Nhánh:** `feat/mvp-core-loop` · **Nguồn yêu cầu:** `docs/ai-handoff-v2/MASTER_SPEC_ALL_IN_ONE.md`

Tài liệu này ghi lại những gì đã được code, bằng chứng kiểm thử **đã chạy thật**, và những gì còn bị chặn. Kết quả `PASS` nghĩa là test tự động (hoặc kiểm tra thủ công được ghi rõ) đã chạy và đạt trên máy phát triển local (Docker Desktop, PostgreSQL 16). Không có kết quả nào trên VPS hay hạ tầng Beta.

## 1. Tóm tắt

- **Đã chạy được end-to-end trên local:** đăng ký/đăng nhập (1 phiên/tài khoản, 2FA tùy chọn), tạo nhân vật, Quảng trường nhiều phòng có di chuyển dự đoán + đối chiếu server, chat công khai/riêng, bạn bè/chặn, nông trại 5 cây + sâu thủ công, NPC mua/bán, túi đồ + hàng chờ, Xu/KC, thời trang cơ bản, nhiệm vụ ngày, Thẻ Đổi tên, trang Admin (TOTP bắt buộc, RBAC, điều chỉnh tài sản 2 người, cấu hình phiên bản + publish trong bảo trì, nhật ký), công cụ reset Closed Beta.
- **Không làm (đúng phạm vi MVP):** chim bắt sâu, chợ người chơi, nạp tiền thật.
- **Chưa thể làm/kiểm chứng:** HA/PITR/failover, nhiều instance Game Server, load test 300 người, CAPTCHA, TLS/WSS và DDoS (cần hạ tầng thật — xem §4). **Không** mở Beta công khai trên VPS Windows Server 2012 R2 (GAP-02).

## 2. Kiểm thử đã chạy (21/10/2026)

| Bộ | Lệnh | Kết quả |
|---|---|---|
| Go game-server | `gofmt -l`, `go vet ./...`, `go test -race -count=1 ./...` (golang:1.27) | PASS — admin, authn, proto, realtime, sim, social, world |
| Go transaction-service | như trên, `-p 1` | PASS — betareset, econ, farm, inventory, names |
| Client | `npm test` (12 test), `npm run typecheck`, `npm run build` | PASS |
| Admin end-to-end | `node tools/dev/admin-e2e.mjs` (qua origin web, TOTP thật) | PASS 13/13 |
| Trình duyệt (thủ công) | Thẻ Đổi tên: tên trùng bị từ chối và còn thẻ; tên hợp lệ trừ 1 thẻ, HUD và nhãn trên đầu nhân vật đổi ngay. W-06: nhấp đi xa rồi bấm W → dừng auto-path; cùng thao tác không bấm phím → đi hết đường | PASS |

Chạy lại tất cả: `deploy\windows\run-tests.bat` (cần stack local đang chạy). CI: `.github/workflows/ci.yml` (Postgres service, gofmt/vet/test -race, client test+build, kiểm tra sinh lại map, build Docker image).

## 3. Bảng nghiệm thu (MASTER_SPEC §Acceptance)

### Tài khoản & nhân vật

| ID | Kết quả | Bằng chứng |
|---|---|---|
| A-01 | PASS | `TestA01NewLoginInvalidatesTheOldSession` (authn), `TestA01LoginElsewhereClosesTheOldSocket` (realtime) |
| A-02 | PASS | `TestA02ConcurrentCreateMakesExactlyOneCharacter` |
| A-03 | PASS | `TestKeyIsCaseInsensitiveButAccentSensitive`, `TestCreateCharacterReplayAndUniqueness` |
| A-04 | PASS | như A-03 (`Thành` hợp lệ khi đã có `Thanh`) |
| A-05 | PASS | `TestComposedAndDecomposedAreTheSameName`, A-05 trong `TestCreateCharacterReplayAndUniqueness` |
| A-06 | PASS | `TestValidateLengthCountsGraphemes`; client `nameRules.test.ts` cùng kết luận |
| A-07 | PASS | `TestA07RenameConsumesTheCardOnlyOnSuccess` + kiểm tra trình duyệt |
| A-08, A-09 | PASS | `TestA08A09PasswordResetExpiresAndRevokesSessions` |
| A-10 | PASS | `TestA10TOTPLoginNeedsTheSecondStep` |
| A-11 | PASS | `TestA11AdminLoginAlwaysNeedsTOTP`, admin-e2e |
| A-12 | **PARTIAL** | `TestA12LimiterSlowsDownButNeverLocksForever` (5 lần sai miễn phí, trễ tăng dần tối đa 5 phút, 30 lần/phút/IP, không khóa vĩnh viễn). **Chưa có CAPTCHA** — nhà cung cấp `[CẦN CẤU HÌNH]` (GAP-08 hồ sơ cũ / GAP-16). |

### Thế giới & di chuyển

| ID | Kết quả | Bằng chứng |
|---|---|---|
| W-01 | PASS | `TestW01RoomsNeverExceedCapacity` |
| W-02 | PASS | `TestW02JoinFriendRoomRespectsCapacity` |
| W-03 | PASS | `TestW03ReconnectWithinGraceKeepsRoomPositionAndSlot` (grace 30 s) |
| W-04 | PASS | `TestW04AfterGraceTheSlotIsFreedAndPositionSaved` |
| W-05 | PASS | `TestW05FloodedInputsCannotOutrunTheServer`, `TestStepNeverEntersWallsOrNPCs` (bỏ qua input vượt, ghi log, không tự ban) |
| W-06 | PASS (thủ công) | Kiểm tra trình duyệt ở §2; code `WorldScene` hủy path ở sự kiện phím xuống, kể cả nhấp phím rất ngắn |
| W-07 | **BLOCKED** | Chỉ có 1 instance Game Server. Vị trí được lưu bền (W-04) nhưng chưa có bộ điều phối/replacement nhiều server để kiểm (GAP-03). |
| W-08 | PASS | `realtime_test.go` (W-08: `MAP_VERSION_MISMATCH`, client hiện thông báo tải lại) |

### Nông trại, túi đồ, hàng chờ

| ID | Kết quả | Bằng chứng |
|---|---|---|
| F-01 | PASS | `TestF01PlantConsumesExactlyOneSeed` |
| F-02 | PASS | `TestT07T11NewPublishedPriceAppliesOnlyAfterPublish` (yield snapshot từ phiên bản cũ) |
| F-03 | PASS | `TestF03MatureCropNeverWithers` |
| F-04 | PASS | `TestF04WholeLotGoesToPendingWhenItDoesNotFit` |
| F-05 | PASS | `TestF05PendingLotsMergeKeepingEarliestTime` |
| F-06, F-07 | PASS | `TestF06F07ClaimWholeLinesFIFO` |
| F-08 | PASS | `TestF08PartialClaimWhenLineExceedsMaxPossibleBag` (max = `inventory.upgrade.max_slot_cap` × stack, GAP-07) |
| F-09 | PASS | `TestF09HarvestAllContinuesPastAnOverflowingPlot` |
| F-10 | PASS | `TestF10UprootAllNeedsConfirmationAndTakesEverything` |
| F-11 | PASS | `TestF11PlantAllUsesIncreasingPlotsUntilSeedsRunOut` |
| F-12 | PASS | `TestF12SpawnWindowAndDeadline`, `TestRollPestRandomizedInvariants` |
| F-13 | PASS | `TestF13YieldPenalty` |
| F-14 | PASS | `TestF12F13F14PestLifecycle` (sâu được quyết định một lần khi trồng và lưu DB) |
| F-15 | PASS | `TestF15ThousandItemsMakeTwoStacks` |
| F-16 | PASS | `TestF16InstancesNeverStack` |
| F-17 | PASS (chặn tạm) | `TestF17CatchPestWithFullBagIsBlocked` — từ chối bắt sâu khi túi không chứa được sâu, không xóa/không bịa luật. **Luật cuối chờ chủ dự án (GAP-06).** |

### Giao dịch & tài sản

| ID | Kết quả | Bằng chứng |
|---|---|---|
| T-01 | PASS | `TestT01BuyDebitsAndGrantsWithLedger` |
| T-02 | PASS | `TestT02ConcurrentRetriesExecuteOnce` (100 request song song cùng `request_id`) |
| T-03 | PASS | `TestT03SameRequestDifferentPayloadConflicts` |
| T-04 | **PARTIAL** | Mỗi lệnh là **một** transaction PostgreSQL (trừ tiền + cấp đồ + ledger + request commit cùng nhau); `TestBuyRejectsWithoutPartialEffects`, `TestAddIsAllOrNothing`. **Chưa** giả lập sập DB giữa chừng rồi restore. |
| T-05 | PASS | `TestT05NoDurableStandbyStopsAssetWrites` (`REQUIRE_SYNC_STANDBY=true`, không có standby → `ASSET_WRITES_DISABLED`, ví không đổi) |
| T-06 | PASS | `TestT06DraftPricesAreIgnored` |
| T-07, T-11 | PASS | `TestT07T11NewPublishedPriceAppliesOnlyAfterPublish` |
| T-08 | PASS | `TestT08CannotSellFromPending` |
| T-09, T-10 | PASS | `TestT09T10TwoPersonAdjustments`, admin-e2e (Super Admin không tự duyệt; Admin khác duyệt → 1 lần thực thi, ledger `ADMIN_ADJUSTMENT`, audit) |

### Xã hội & chat

| ID | Kết quả | Bằng chứng |
|---|---|---|
| S-01 | PASS | `TestS01PublicChatStaysInTheRoom` |
| S-02 | PASS | `TestS02FriendLimitHoldsEvenWithConcurrentAccepts` |
| S-03 | PASS | `TestS03S04HistoryPagesOf50Then20WithoutGapsOrRepeats` |
| S-04 | PASS (server) | như S-03. Giữ vị trí viewport khi nạp thêm: đã code trong `PrivateChatPanel`, **chưa có test tự động** phía client. |
| S-05 | PASS (server) | `TestS05OnlyTheRecipientMarksVisibleMessagesSeen`; client chỉ báo seen tin đã vào viewport (IntersectionObserver). Edge case nhiều tab: GAP-17. |
| S-06, S-07 | PASS | `TestS06S07ExpiryAndOneWaySilentBlock` |
| S-08, S-09 | PASS | `TestS08S09BlockListPagesAndSearch` |
| S-10 | PASS | `TestS10RetrySameRequestStoresOneMessage` |

### Vận hành

| ID | Kết quả | Bằng chứng / lý do |
|---|---|---|
| O-01 | **BLOCKED** | Chưa có hạ tầng backup PITR/WAL archive để restore thử (GAP-03, GAP-18). |
| O-02 | **BLOCKED** | Chưa có nhiều World server. |
| O-03 | **BLOCKED** | Chưa có primary/standby; `REQUIRE_SYNC_STANDBY` sẵn sàng phía ứng dụng. |
| O-04 | PASS | `TestO04O05ClosedBetaReset` + `deploy/RUNBOOK_CLOSED_BETA_RESET.md` |
| O-05 | PASS | như O-04 (open_beta/production bị từ chối cứng) |
| O-06 | PASS | `TestO06PublishOnlyDuringMaintenance`, admin-e2e |
| O-07 | **BLOCKED** | Chưa load test 300 người; không có số đo thì không được ghi PASS. |
| O-08 | **PARTIAL** | Đã có: bcrypt, token băm, 1 phiên, CSRF (Origin + JSON), cookie Secure bắt buộc ngoài local, RBAC + TOTP Admin, TOTP mã hóa AES-GCM, rate limit, ledger append-only. Chưa có: TLS/WSS thật, WAF/DDoS, review bảo mật độc lập. |

## 4. Việc còn mở / cần chủ dự án quyết định

| Mục | Hiện trạng | Cần gì |
|---|---|---|
| GAP-01 Xu vượt trần | Lệnh làm Xu vượt trần bị **từ chối** (`WALLET_CAP_EXCEEDED`), không tiêu hủy Xu `[ĐỀ XUẤT]` | Xác nhận luật GDD cũ |
| GAP-02/03 Hạ tầng | Chỉ chạy local Docker; `deploy/README_VPS_WIN2012.md` giữ cảnh báo | Chọn host hỗ trợ, HA, backup/PITR |
| GAP-04/05/10 Số liệu | Seed `DEV SAMPLE` chỉ cho local; production không có config → `FEATURE_NOT_CONFIGURED` | Admin nhập & publish giá/yield/tỷ lệ sâu/nâng cấp/nhiệm vụ |
| GAP-06 Bắt sâu túi đầy | Từ chối `[ĐỀ XUẤT]` | Xác nhận luật |
| GAP-08 KC chờ nhận | Nhận nguyên dòng, không nhận một phần `[ĐỀ XUẤT]` | Xác nhận |
| GAP-09 Khoảng trắng tên | Từ chối khoảng trắng đầu/cuối và liên tiếp `[ĐỀ XUẤT]` | Xác nhận |
| GAP-11 Chat công khai | Giới hạn kỹ thuật dev (200 ký tự, 800 ms) trong config mẫu | Chốt giới hạn/retention |
| GAP-12 Bỏ chặn | Một cách bỏ chặn (danh sách chặn) | Xác nhận 4 cách trong GDD |
| GAP-13 Hình ảnh | Nhân vật/ô/biểu tượng vẽ bằng code (placeholder) | Bộ asset có license |
| GAP-14 Trang phục | Đồ đang mặc vẫn chiếm ô túi `[ĐỀ XUẤT]` | UX review |
| GAP-15 Tick/room cap | Tick 50 ms, snapshot mỗi 2 tick; sức chứa phòng lấy từ config (mẫu 30) | Benchmark |
| Thẻ Đổi tên | Item `rename_card` (chính sách giao dịch `bound` `[ĐỀ XUẤT]`); nguồn thẻ do Admin cấu hình (P0-027) — hiện cấp qua điều chỉnh tài sản 2 người | Chốt nguồn thẻ & chính sách giao dịch |
| Khác `[ĐỀ XUẤT]` | Hiện diện (online) chỉ bạn bè thấy; được hủy kết bạn; ban vô thời hạn; stack hạt giống 999; nhiệm vụ reset 0h `Asia/Ho_Chi_Minh` và không nhận thưởng ngày trước | Xác nhận |
| Redis | Có trong compose nhưng chưa dùng (rate limit/presence trong 1 tiến trình) | Cần khi chạy nhiều instance |
| Email | Local ghi mã ra log; môi trường khác tắt tính năng email khi chưa có nhà cung cấp | Chọn dịch vụ email (GAP-16) |

## 5. Tài khoản thử nghiệm local

- **Người chơi:** tự đăng ký tại `http://localhost:5173` (không có tài khoản mặc định).
- **Admin:** tạo bằng CLI, kết quả (mật khẩu sinh ngẫu nhiên, TOTP secret, mã khôi phục) **chỉ hiện một lần**. Trên máy local được lưu vào `deploy/local-secrets/admin-<login>.json` — thư mục này nằm trong `.gitignore`, **không commit**:
  ```bash
  docker compose -f deploy/compose.local.yml exec -T game-server /usr/local/bin/admin create -login ops_lead -roles SUPER_ADMIN > deploy/local-secrets/admin-ops_lead.json
  docker compose -f deploy/compose.local.yml exec -T game-server /usr/local/bin/admin create -login game_ops -roles GAME_ADMIN > deploy/local-secrets/admin-game_ops.json
  ```
  `tools/dev/admin-e2e.mjs` đọc hai file này để tự tính mã TOTP.
- **Đẩy nhanh cây trồng khi thử local:** `tools/dev/fast-forward-farm.sql` (chỉ database dev).
