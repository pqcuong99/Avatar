import { ApiError } from '../api/http.ts';
import { itemName } from '../state/store.ts';
import { PROBLEM_TEXT, type NameProblem } from '../features/character/nameRules.ts';

export const num = (n: number) => n.toLocaleString('vi-VN');

export const money = (n: number, currency: 'XU' | 'DIAMOND') => `${num(n)} ${currency === 'XU' ? 'Xu' : 'KC'}`;

export function duration(ms: number): string {
  if (ms <= 0) return '0 giây';
  const s = Math.ceil(ms / 1000);
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
  if (h > 0) return m ? `${h} giờ ${m} phút` : `${h} giờ`;
  if (m > 0) return sec ? `${m} phút ${sec} giây` : `${m} phút`;
  return `${sec} giây`;
}

/** Compact timer: 1:05:09, 29:59, 0:07. */
export function countdown(ms: number): string {
  const s = Math.max(0, Math.ceil(ms / 1000));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`;
}

export function clock(iso: string): string {
  return new Date(iso).toLocaleString('vi-VN', { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit' });
}

/** Player-facing explanation of an API error, using its details when useful. */
export function explain(e: unknown): string {
  if (!(e instanceof ApiError)) return 'Có lỗi xảy ra, hãy thử lại.';
  const d = e.details;
  switch (e.code) {
    case 'INSUFFICIENT_FUNDS':
      return `Không đủ tiền: cần ${money(Number(d.required), d.currency as 'XU')}, đang có ${money(Number(d.balance), d.currency as 'XU')}.`;
    case 'INVENTORY_CAPACITY_INSUFFICIENT':
      return `Túi không đủ chỗ${d.required ? ` (cần ${num(Number(d.required))}, còn chứa được ${num(Number(d.free_capacity))})` : ''}.`;
    case 'PENDING_WHOLE_BATCH_REQUIRED':
      return `Túi phải chứa được cả dòng ${num(Number(d.required))} ${itemName('crop_' + d.crop_code)} (còn chỗ cho ${num(Number(d.free_capacity))}).`;
    case 'ITEM_INSUFFICIENT':
      if (d.in_pending) return `Vật phẩm đang ở Hàng chờ (${num(Number(d.in_pending))}). Hãy nhận vào túi trước khi bán.`;
      return `Không đủ ${itemName(String(d.item_code))} (cần ${num(Number(d.required))}, có ${num(Number(d.held))}).`;
    case 'RATE_LIMITED':
      return d.retry_after_seconds ? `Thử lại sau ${d.retry_after_seconds} giây.` : e.message;
    case 'FEATURE_NOT_CONFIGURED':
      return 'Chức năng này chưa được Admin cấu hình.';
    case 'CHARACTER_NAME_TAKEN':
      return 'Tên này đã có người dùng (không phân biệt hoa/thường).';
    case 'CHARACTER_NAME_INVALID':
      return PROBLEM_TEXT[(d.problem as NameProblem) ?? 'invalid_char'] ?? e.message;
    case 'WALLET_CAP_EXCEEDED':
      return `Ví đã chạm giới hạn ${num(Number(d.cap))}; giao dịch bị từ chối để không mất tiền.`;
    default:
      return e.message;
  }
}
