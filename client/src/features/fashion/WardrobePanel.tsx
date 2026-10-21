import { useState } from 'react';
import { itemName, store, useStore } from '../../state/store.ts';
import { Window } from '../../ui/Window.tsx';
import { ItemIcon } from '../../ui/ItemIcon.tsx';
import { CharacterPreview } from '../character/CharacterPreview.tsx';
import type { Appearance } from '../../game/art/characterArt.ts';
import { act } from '../actions.ts';

const SLOT_NAME: Record<string, string> = { hat: 'Mũ', top: 'Áo', bottom: 'Quần/Váy' };

// UI-12: wear and take off owned fashion. Everyone in the room sees the change
// once the server has committed it.
export function WardrobePanel({ onClose }: { onClose: () => void }) {
  const game = useStore((s) => s.game);
  const catalog = useStore((s) => s.catalog);
  const me = useStore((s) => s.me);
  const [busy, setBusy] = useState<string | null>(null);
  if (!game || !me) return null;
  const fashion = new Map((catalog?.fashion ?? []).map((f) => [f.item_code, f]));
  const owned = (game.inventory.slots ?? []).filter((s) => s.instance_id && fashion.has(s.item_code));
  const equipment = game.equipment ?? {};
  const look: Appearance = { version: 0, items: {} };
  for (const [slot, id] of Object.entries(equipment)) {
    const s = owned.find((o) => o.instance_id === id);
    const f = s && fashion.get(s.item_code);
    if (s && f) look.items[slot] = { item_code: s.item_code, style: f.style };
  }

  async function run(key: string, path: string, body: Record<string, unknown>, ok: string) {
    setBusy(key);
    await act(path, body, { success: () => ok });
    setBusy(null);
  }

  return (
    <Window title="Tủ đồ" onClose={onClose} wide>
      <div className="wardrobe">
        <div className="wardrobe-preview">
          <CharacterPreview seed={me.id} name={me.name} look={look} />
          <ul className="worn">
            {(['hat', 'top', 'bottom'] as const).map((slot) => {
              const item = look.items[slot];
              return (
                <li key={slot}>
                  <span className="muted small">{SLOT_NAME[slot]}</span>
                  {item ? (
                    <>
                      <b>{itemName(item.item_code)}</b>
                      <button className="btn small ghost" disabled={!!busy} onClick={() => void run('off-' + slot, '/api/v1/fashion/unequip', { equip_slot: slot }, `Đã tháo ${SLOT_NAME[slot].toLowerCase()}.`)}>Tháo</button>
                    </>
                  ) : <span className="muted">—</span>}
                </li>
              );
            })}
          </ul>
        </div>
        <div className="wardrobe-items">
          {owned.length === 0 ? (
            <p className="muted">Chưa có món thời trang nào. Ghé <b>cửa hàng thời trang</b> ở góc trên bên trái bản đồ.</p>
          ) : (
            <div className="bag-grid">
              {owned.map((s) => {
                const f = fashion.get(s.item_code)!;
                const worn = equipment[f.equip_slot] === s.instance_id;
                return (
                  <button key={s.instance_id} className={`bag-slot ${worn ? 'selected' : ''}`} disabled={!!busy || worn}
                    title={`${itemName(s.item_code)} · ${SLOT_NAME[f.equip_slot]}${worn ? ' · đang mặc' : ''}`}
                    onClick={() => void run(s.instance_id!, '/api/v1/fashion/equip', { instance_id: s.instance_id }, `Đã mặc ${itemName(s.item_code)}.`)}>
                    <ItemIcon code={s.item_code} size={36} />
                    {worn && <span className="worn-tag">Đang mặc</span>}
                  </button>
                );
              })}
            </div>
          )}
          <p className="muted small">Bấm một món để mặc. Đồ đang mặc vẫn nằm trong túi. Mỗi món chiếm một ô túi riêng.</p>
          <button className="btn ghost small" onClick={() => store.set({ panel: 'inventory' })}>Mở túi đồ</button>
        </div>
      </div>
    </Window>
  );
}
