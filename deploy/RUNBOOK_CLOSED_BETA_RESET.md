# Runbook — Reset dữ liệu người chơi cuối Closed Beta

Áp dụng P0-160, P0-177..179 và `docs/09_OPEN_ISSUES.md` §7: cuối Closed Beta xóa **toàn bộ dữ liệu người chơi**, **giữ** Admin, cấu hình, danh mục và nhật ký Admin. Công cụ là lệnh `beta-reset` (nằm trong image `transaction-service`), **không có endpoint HTTP**.

> ⚠️ Không bao giờ chạy trên Open Beta / production. Lệnh tự chặn khi `THANH_ENV` khác `closed_beta` (hoặc `local` để diễn tập) — kiểm thử O-05.
> ⚠️ Không dùng `docker compose down -v` thay cho reset: `-v` xóa cả Admin và cấu hình.

## Điều kiện bắt buộc (lệnh kiểm tra tất cả, thiếu một là từ chối)

| Điều kiện | Cách đáp ứng |
|---|---|
| `THANH_ENV=closed_beta` (hoặc `local` khi diễn tập) | biến môi trường của tiến trình chạy lệnh |
| Server đang bảo trì | Admin → Bảo trì → Bật (không còn giao dịch đang chạy) |
| Cụm xác nhận đúng | `-confirm "RESET <tên database>"` |
| Có tham chiếu bản backup | `-backup-ref` = tên file/checksum/ID lưu trữ của backup vừa tạo |
| Người thực hiện | `-operator` (ghi vào `admin_audit_logs`) |

## Các bước

1. **Thông báo** người chơi thời điểm kết thúc Closed Beta.
2. **Bật bảo trì** trong trang Admin (quyền `server.maintenance`). Kiểm tra người chơi nhận thông báo và các lệnh tài sản trả `MAINTENANCE`.
3. **Backup** database và kiểm tra backup đọc được, ví dụ:
   ```bash
   pg_dump --format=custom --file=thanh_closed_beta_end.dump "$DATABASE_URL"
   pg_restore --list thanh_closed_beta_end.dump > /dev/null && sha256sum thanh_closed_beta_end.dump
   ```
   Cất file ở nơi lưu trữ riêng (không commit vào Git).
4. **Chạy thử (dry run)** — chỉ đếm, không xóa:
   ```bash
   beta-reset
   ```
   Đọc báo cáo `before` (bảng người chơi) và `kept` (bảng giữ lại).
5. **Thực hiện**:
   ```bash
   beta-reset -execute -confirm "RESET thanh_beta" -backup-ref "thanh_closed_beta_end.dump sha256:<...>" -operator "<tên>"
   ```
   Toàn bộ chạy trong **một transaction**: mọi bảng người chơi về 0 hoặc không có gì thay đổi. Sổ cái/lịch sử giao dịch append-only chỉ cho xóa trong transaction này (`thanh.allow_player_data_purge`).
6. **Kiểm tra** báo cáo: `after` toàn 0, `kept` không đổi; trong Admin → Nhật ký có dòng `closed_beta.reset` (người thực hiện, backup-ref, số liệu trước reset).
7. **Tắt bảo trì** khi sẵn sàng mở lại (nếu có đợt test mới).

Diễn tập trên máy local (stack `compose.local.yml`, `THANH_ENV=local`):

```bash
docker compose -f deploy/compose.local.yml run --rm -e THANH_ENV=local transaction-service /usr/local/bin/beta-reset
```

Bằng chứng tự động: `TestO04O05ClosedBetaReset` (transaction-service/internal/betareset).
