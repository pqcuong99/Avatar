# ĐẶC TẢ KIẾN TRÚC KỸ THUẬT — bản định hướng

**Nhãn:** [CHỐT] là yêu cầu phải giữ; [ĐỀ XUẤT] là thiết kế triển khai cần test/review.

## 1. Technology stack [CHỐT]

| Thành phần | Công nghệ | Ghi chú |
|---|---|---|
| Web Client | Phaser + React + TypeScript | PC và mobile landscape |
| Map editor | Tiled Map Editor → JSON | Cùng phiên bản logic vật cản ở Client/Server |
| Real-time Game Server | Go | Chia phòng, tick di chuyển, kiểm tra va chạm |
| Transaction Service | Go | Trung tâm trách nhiệm ghi tài sản, scale nhiều replica |
| Durable state | PostgreSQL | Nguồn sự thật cho account/farm/inventory/ledger |
| Transient state | Redis | Cache, session hỗ trợ, điều phối và rate limits; không là nguồn tài sản |
| Live transport | WebSocket Secure (WSS) | JSON sự kiện chậm, binary chuyển động |
| Development deployment | Docker Compose nơi hệ điều hành hỗ trợ | Windows Server 2012 R2 không hỗ trợ WSL2/Linux containers |
| Repository | Git monorepo | `https://github.com/pqcuong99/Avatar.git` |

## 2. Sơ đồ logic [ĐẶC TẢ TRIỂN KHAI]

```text
Browser (Phaser canvas + React UI)
         |
         | HTTPS / WSS
         v
API gateway / TLS termination / rate limits
         |              |
         v              v
Account/API Go     Real-time Game Server Go (instances)
         |              | room registry/presence
         |              v
         |         movement, map, chat, NPC interaction
         |              |
         +--------------+----> Transaction Service Go (instances)
                                      |
                                      v
                              PostgreSQL primary
                                      |
                           synchronous durability policy
                                      v
                              PostgreSQL standby

Redis: session/cache/coordination — not canonical balance.
Offsite backup + WAL/PITR + monitoring + independent access control.
```

**[CHỐT]** Server quyết định tính đúng của di chuyển và tài sản. **[ĐỀ XUẤT]** `Account/API Go` có thể là module/process riêng; chưa chọn chính thức mức tách vi dịch vụ hay port.

## 3. Phân phòng và di chuyển

- Server tự phân phòng và tạo phòng khi đầy; Admin cấu hình cap mỗi phòng (P0-134).
- Nút `Đến cùng phòng` trên profile bạn bè: nếu còn suất thì chuyển, nếu đầy thì báo lỗi; một nhân vật không được ở hai phòng cùng lúc (P0-135).
- 30s reconnect grace giữ nhân vật đứng yên và chiếm suất, reconnect trong hạn về phòng/vị trí cũ; quá hạn giải phóng suất (P0-136).
- Client prediction + server reconciliation (P0-137); nội suy những nhân vật khác. Client sử dụng pathfinding Tiled, Server validate từng bước (P0-168).
- Cheating: reject move, reconcile, log, flag cho Admin; không auto khóa vì một hay nhiều thông điệp di chuyển sai (P0-138).
- **[ĐỀ XUẤT]** Binary messages dùng schema version + sequence IDs + server tick; ngưỡng tick/bandwidth cần benchmark chứ không ấn định trước.

## 4. Asset transaction [CHỐT]

1. Client gửi yêu cầu với `request_id`/idempotency token và dữ liệu đầu vào.
2. Game Server/API xác thực phiên/quyền, NPC/range/farm state.
3. Transaction Service đọc nguồn sự thật trong PostgreSQL, kiểm tra giá đang có hiệu lực, số dư, điều kiện, cap.
4. Bắt đầu database transaction, lock/optimistic version hợp lý, ghi state + ledger + idempotency result nguyên tử.
5. Commit bền vững theo chính sách P0-150 mới trả thành công. Client retry chỉ thấy lại **cùng kết quả**.
6. Nếu trạng thái chưa rõ do crash/timeout, tra cứu theo request_id; không tự ý cộng/trừ lần nữa.

**[CHỐT]** Khi không bảo đảm synchronous durability/standby theo P0-150, **tạm dừng giao dịch tài sản**. Một VPS không thể đáp ứng dự phòng vật lý của P0-149; không tuyên bố phù hợp Open Beta khi chỉ có một VPS.

## 5. Offline timing

Mọi mốc thời gian authoritative của cây và sự kiện dùng server UTC + timestamp PostgreSQL; các mốc được ghi lúc giao dịch, khi truy vấn tính trạng thái hiện tại từ mốc đã lưu. Không tạo timer lâu dài riêng cho từng cây. Sâu random phát sinh phải có lần quyết định ổn định/lưu DB; nếu mất/restart không bốc lại. Quy tắc snapshot yield/growth được kiểm soát ở backend.

## 6. Failover và sao lưu — kiến trúc mục tiêu

- Daily backup + WAL/PITR; lưu bản sao **ngoài VPS chính**; mã hóa, quyền truy cập tối thiểu, test restore định kỳ.
- Game Server lỗi: phát hiện, loại khỏi phân tải, khởi phòng thay thế, reconnect và khôi phục trạng thái bền vững, đối soát giao dịch chưa rõ.
- PostgreSQL HA: dự phòng, tự failover có fencing/quorum tránh split-brain; đảm bảo độ bền theo P0-150. Tự failover đa máy là **đích phải đạt về sau**, hiện tại chưa có.
- Thay giá/config/tin công khai theo maintenance version. Không đổi giá giữa giao dịch. Migrate/schema backward compatibility/rollback được đánh giá từng bản cập nhật.

## 7. Bảo mật và vận hành

- Đăng nhập rate limit theo nhiều tín hiệu; mật khẩu hash theo chuẩn mạnh (ví dụ Argon2id, cần review tham số), TOTP tùy chọn user/bắt buộc admin, backup codes hash một chiều.
- RBAC ở backend; điều chỉnh asset thủ công yêu cầu **two-person approval** kể cả Super Admin; đầy đủ audit/event.
- WSS/HTTPS, xác thực Origin, CSRF khi dùng cookie, không log bí mật; reject input không hợp lệ và giới hạn kích thước message.
- Rate limits Game action, WAF/DDoS provider, chặn truy cập trực tiếp origin, alerting.
- [ĐỀ XUẤT] CI `go test`, static checks, TypeScript check/build, migration test, unit/e2e.

## 8. Sơ đồ môi trường và ràng buộc VPS

| Môi trường | Nền tảng dự kiến | Trạng thái |
|---|---|---|
| Dev cá nhân | Máy cá nhân, Docker hỗ trợ Linux containers hoặc chạy service native | Starter hỗ trợ bản demo, chưa xác minh trên máy người dùng |
| Closed Beta tạm | **Một VPS Windows Server 2012 R2 Datacenter (6.3.9600)** | **CHƯA CÓ runtime Go hiện đại/WSL2/Docker Linux được xác minh** |
| Open Beta | Nhiều máy/hạ tầng phù hợp, PostgreSQL HA, offsite PITR | **Chưa triển khai**; yêu cầu trước Open Beta |

**Chặn quan trọng:** Windows Server 2012 R2 đã hết hỗ trợ; không hỗ trợ WSL2, không dùng Docker Desktop/Linux containers. Không mặc định `git clone` + `docker compose up` trên VPS cũ chạy được. Các bản Go hiện đại cũng không bảo đảm hỗ trợ hệ điều hành này. Trước khi mở Closed Beta thật cần chọn runtime/hạ tầng tương thích và kiểm thử bảo mật; không hạ phiên bản công nghệ đến mức không còn bản vá chỉ để chạy cho bằng được.

## 9. Non-functional acceptance (mức đang được chốt)

- Mục tiêu Open Beta: **300 người online đồng thời** với hỗn hợp tải movement/chat/farm/NPC, cần thử tải thực tế.
- Giao dịch tài sản chính xác và idempotent; không được thông báo success giả.
- Không làm mất pending item khi túi đầy.
- Backup/restore, failover, bảo mật phải chứng minh qua thử nghiệm. Các thông số p95 latency, TPS, RAM theo phòng, RPO/RTO **chưa được chủ dự án chốt**, cần đo qua benchmark.
