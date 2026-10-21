import { useState } from 'react';
import { itemName, useStore } from '../../state/store.ts';
import { Window } from '../../ui/Window.tsx';
import { CurrencyIcon, ItemIcon } from '../../ui/ItemIcon.tsx';
import { clock, num } from '../../ui/format.ts';
import { act } from '../actions.ts';

interface ClaimResult { claimed: { crop_code: string; item_code: string; quantity: number; remaining: number; partial: boolean }[]; stopped_at?: string }

function claimText(r: ClaimResult): string {
  const parts = r.claimed.map((c) => `${num(c.quantity)} ${itemName(c.item_code)}${c.partial ? ` (còn ${num(c.remaining)} chờ)` : ''}`);
  return `Đã nhận ${parts.join(', ')}${r.stopped_at ? `. Dừng ở ${itemName('crop_' + r.stopped_at)}: túi không đủ chỗ cho cả dòng.` : '.'}`;
}

// UI-10: merged crop lines (oldest first) and Diamond entries per source.
// Nothing is ever claimed automatically (P0-077).
export function PendingPanel({ onClose }: { onClose: () => void }) {
  const crops = useStore((s) => s.game?.pending_crops ?? []);
  const gems = useStore((s) => s.game?.pending_diamonds ?? []);
  const [busy, setBusy] = useState<string | null>(null);

  async function run(key: string, path: string, success: (r: ClaimResult & { claimed: unknown[] }) => string) {
    setBusy(key);
    await act<ClaimResult>(path, {}, { success });
    setBusy(null);
  }

  return (
    <Window title="Hàng chờ nhận" onClose={onClose}>
      <h3 className="section-title">Nông sản</h3>
      {crops.length === 0 ? <p className="muted">Không có nông sản chờ nhận.</p> : (
        <>
          <ul className="pending-list">
            {crops.map((p) => (
              <li key={p.crop_code}>
                <ItemIcon code={p.item_code} size={36} />
                <div className="grow">
                  <b>{itemName(p.item_code)}</b> × {num(p.quantity)}
                  <div className="muted small">Từ {clock(p.earliest_created_at)}</div>
                </div>
                <button className="btn small" disabled={!!busy} onClick={() => void run(p.crop_code, `/api/v1/pending/crops/${p.crop_code}/claim`, claimText)}>
                  {busy === p.crop_code ? 'Đang nhận…' : 'Nhận'}
                </button>
              </li>
            ))}
          </ul>
          <div className="row between">
            <span className="muted small">Mỗi dòng phải vừa túi trọn vẹn; nhận tất cả đi từ dòng cũ nhất và dừng ở dòng đầu tiên không vừa.</span>
            <button className="btn primary" disabled={!!busy} onClick={() => void run('all-crops', '/api/v1/pending/crops/claim-all', claimText)}>Nhận tất cả</button>
          </div>
        </>
      )}
      <h3 className="section-title">Kim Cương thưởng</h3>
      {gems.length === 0 ? <p className="muted">Không có Kim Cương chờ nhận.</p> : (
        <>
          <ul className="pending-list">
            {gems.map((g) => (
              <li key={g.id}>
                <CurrencyIcon currency="DIAMOND" size={28} />
                <div className="grow">
                  <b>{num(g.remaining)} KC</b> · {g.source_kind}
                  <div className="muted small">{clock(g.created_at)}</div>
                </div>
                <button className="btn small" disabled={!!busy} onClick={() => void run(g.id, `/api/v1/pending/diamonds/${g.id}/claim`, () => `Đã nhận ${num(g.remaining)} KC.`)}>Nhận</button>
              </li>
            ))}
          </ul>
          <div className="row end">
            <button className="btn" disabled={!!busy} onClick={() => void run('all-gems', '/api/v1/pending/diamonds/claim-all', () => 'Đã nhận Kim Cương.')}>Nhận tất cả</button>
          </div>
        </>
      )}
    </Window>
  );
}
