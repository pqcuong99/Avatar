import { useState } from 'react';
import { itemName, useStore } from '../../state/store.ts';
import { Window, ConfirmDialog, type ConfirmRequest } from '../../ui/Window.tsx';
import { ItemIcon } from '../../ui/ItemIcon.tsx';
import { money, num } from '../../ui/format.ts';
import { act } from '../actions.ts';
import { RenameForm } from './RenameForm.tsx';

const CATEGORY: Record<string, string> = { seed: 'Hạt giống', crop: 'Nông sản', worm: 'Sâu', fashion: 'Thời trang', upgrade: 'Nâng cấp', other: 'Khác' };

// UI-09: numbered slots; a slot holds one stack (999 for farm items).
export function InventoryPanel({ onClose }: { onClose: () => void }) {
  const inv = useStore((s) => s.game?.inventory);
  const catalog = useStore((s) => s.catalog);
  const [selected, setSelected] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null);
  const [busy, setBusy] = useState(false);
  if (!inv) return null;
  const slots = new Map((inv.slots ?? []).map((s) => [s.slot_no, s]));
  const sel = selected ? slots.get(selected) : undefined;
  const def = sel ? catalog?.items?.find((i) => i.item_code === sel.item_code) : undefined;
  const up = catalog?.inventory_upgrade;
  const canUpgrade = up && up.max_slot_cap && inv.slot_cap + (up.slots_per_upgrade ?? 5) <= up.max_slot_cap;

  return (
    <Window title={`Túi đồ · ${slots.size}/${inv.slot_cap} ô`} onClose={onClose}>
      <div className="bag-grid">
        {Array.from({ length: inv.slot_cap }, (_, i) => i + 1).map((no) => {
          const s = slots.get(no);
          return (
            <button key={no} className={`bag-slot ${s ? '' : 'empty'} ${selected === no ? 'selected' : ''}`} onClick={() => setSelected(s ? no : null)} title={s ? itemName(s.item_code) : `Ô ${no} trống`}>
              <span className="slot-no">{no}</span>
              {s && <ItemIcon code={s.item_code} size={36} />}
              {s && s.quantity > 1 && <span className="qty">{num(s.quantity)}</span>}
            </button>
          );
        })}
      </div>
      <div className="bag-detail">
        {sel && def ? (
          <>
            <ItemIcon code={sel.item_code} size={48} />
            <div>
              <b>{def.display_name}</b> × {num(sel.quantity)}
              <div className="muted small">{CATEGORY[def.category]} · tối đa {num(def.stack_limit)}/ô · ô số {sel.slot_no}</div>
              {(def.category === 'seed' || def.category === 'crop') && <div className="muted small">Không giao dịch giữa người chơi.</div>}
              {sel.item_code === 'rename_card' && <RenameForm />}
            </div>
          </>
        ) : (
          <span className="muted">Chọn một ô để xem vật phẩm.</span>
        )}
      </div>
      <div className="row between">
        <span className="muted small">Túi đầy thì nông sản thu hoạch sẽ vào Hàng chờ.</span>
        {up ? (
          <button className="btn" disabled={!canUpgrade || busy} onClick={() => setConfirm({
            title: 'Nâng cấp túi',
            body: <>Thêm {up.slots_per_upgrade ?? 5} ô túi với giá <b>{money(up.price, up.currency)}</b>?</>,
            confirmLabel: 'Nâng cấp',
            onConfirm: async () => {
              setBusy(true);
              await act<{ slot_cap: number }>('/api/v1/inventory/upgrade', {}, { success: (r) => `Túi đã có ${r.slot_cap} ô.` });
              setBusy(false);
            },
          })}>
            {canUpgrade ? `+${up.slots_per_upgrade ?? 5} ô · ${money(up.price, up.currency)}` : 'Đã tối đa'}
          </button>
        ) : (
          <span className="muted small">Nâng cấp túi: chưa cấu hình</span>
        )}
      </div>
      {confirm && <ConfirmDialog req={confirm} onClose={() => setConfirm(null)} />}
    </Window>
  );
}
