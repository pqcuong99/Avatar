import { useCallback, useEffect, useState } from 'react';
import { get } from '../../api/http.ts';
import type { Offer, OffersView } from '../../api/types.ts';
import { useStore } from '../../state/store.ts';
import { Window } from '../../ui/Window.tsx';
import { CurrencyIcon, ItemIcon } from '../../ui/ItemIcon.tsx';
import { explain, money, num } from '../../ui/format.ts';
import { act } from '../actions.ts';

type Tab = 'buy' | 'sell' | 'talk';

const GREETINGS: Record<string, string> = {
  FARM_SHOP: 'Chào cháu! Hạt giống tươi đây. Trồng xong nhớ để ý sâu nhé — bắt kịp thì không mất mùa. Nông sản thu được bác mua lại hết!',
  FASHION_SHOP: 'Xin chào! Tiệm đang chuẩn bị bộ sưu tập đầu tiên, ghé lại sau nhé.',
};

// UI-08: menu on the left, content on the right; switching tabs never closes
// or reopens the window (docs/04 §3). Prices come from the published config
// version; a stale price is refused by the server and the list reloads.
export function NpcWindow({ code, onClose }: { code: string; onClose: () => void }) {
  const [tab, setTab] = useState<Tab>('buy');
  const [view, setView] = useState<OffersView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const game = useStore((s) => s.game);

  const load = useCallback(async () => {
    try {
      const env = await get<OffersView>(`/api/v1/npc/${code}/offers`);
      setView(env.result);
      setError(null);
    } catch (e) {
      setError(explain(e));
    }
  }, [code]);
  useEffect(() => { void load(); }, [load]);
  // Held quantities change with every trade: reload with the state.
  useEffect(() => { if (view) void load(); }, [game?.inventory]); // eslint-disable-line react-hooks/exhaustive-deps

  const offers = (view?.offers ?? []).filter((o) => (tab === 'buy' ? o.direction === 'NPC_SELLS' : o.direction === 'NPC_BUYS'));

  return (
    <Window title={view?.display_name ?? 'Cửa hàng'} onClose={onClose} wide className="npc-window">
      <div className="npc-layout">
        <nav className="npc-menu" aria-label="Chức năng NPC">
          <button className={tab === 'buy' ? 'active' : ''} onClick={() => setTab('buy')}>🛒 Mua</button>
          <button className={tab === 'sell' ? 'active' : ''} onClick={() => setTab('sell')}>💰 Bán</button>
          <button className={tab === 'talk' ? 'active' : ''} onClick={() => setTab('talk')}>💬 Trò chuyện</button>
          <div className="npc-wallet">
            <span><CurrencyIcon currency="XU" /> {num(game?.wallets.XU.balance ?? 0)}</span>
            <span><CurrencyIcon currency="DIAMOND" /> {num(game?.wallets.DIAMOND.balance ?? 0)}</span>
          </div>
        </nav>
        <div className="npc-content">
          {error && <p className="form-error">{error}</p>}
          {tab === 'talk' && <p className="npc-talk">“{GREETINGS[code] ?? 'Xin chào!'}”</p>}
          {tab !== 'talk' && !view && !error && <p className="muted">Đang tải bảng giá…</p>}
          {tab !== 'talk' && view && offers.length === 0 && (
            <p className="muted">{view.config_version === 0 ? 'Bảng giá chưa được Admin công bố.' : tab === 'buy' ? 'Chưa có hàng.' : 'NPC này không mua gì.'}</p>
          )}
          {tab !== 'talk' && view && offers.map((o) => <OfferRow key={o.offer_id} offer={o} code={code} configVersion={view.config_version} onStale={load} />)}
          {tab === 'sell' && <p className="muted small">Chỉ bán được vật phẩm đang ở trong túi; nông sản ở Hàng chờ phải nhận vào túi trước.</p>}
        </div>
      </div>
    </Window>
  );
}

function OfferRow({ offer, code, configVersion, onStale }: { offer: Offer; code: string; configVersion: number; onStale: () => void }) {
  const [qty, setQty] = useState(1);
  const [busy, setBusy] = useState(false);
  const buying = offer.direction === 'NPC_SELLS';
  const max = buying ? 999 : offer.held_quantity;
  const q = Math.max(1, Math.min(qty, Math.max(1, max)));

  async function trade() {
    setBusy(true);
    const res = await act<{ quantity: number; total: number }>(`/api/v1/npc/${code}/${buying ? 'buy' : 'sell'}`,
      { offer_id: offer.offer_id, quantity: q, config_version: configVersion },
      { success: (r) => `${buying ? 'Đã mua' : 'Đã bán'} ${num(r.quantity)} ${offer.item_name} ${buying ? '−' : '+'}${money(r.total, offer.currency)}.` });
    if (!res) onStale();
    setBusy(false);
  }

  return (
    <div className="offer">
      <ItemIcon code={offer.item_code} size={40} />
      <div className="grow">
        <b>{offer.item_name}</b>
        <div className="muted small">{money(offer.unit_price, offer.currency)} / cái{!buying && ` · trong túi: ${num(offer.held_quantity)}`}</div>
      </div>
      <div className="qty-box">
        <button className="icon-btn small" onClick={() => setQty(Math.max(1, q - 1))} aria-label="Bớt">−</button>
        <input type="number" min={1} max={max} value={q} onChange={(e) => setQty(Number(e.target.value) || 1)} aria-label="Số lượng" />
        <button className="icon-btn small" onClick={() => setQty(Math.min(max, q + 1))} aria-label="Thêm">+</button>
        {!buying && <button className="btn small ghost" onClick={() => setQty(max)} disabled={max < 1}>Hết</button>}
        {buying && <button className="btn small ghost" onClick={() => setQty(10)}>10</button>}
      </div>
      <button className={`btn ${buying ? 'primary' : ''}`} disabled={busy || (!buying && offer.held_quantity < 1)} onClick={() => void trade()}>
        {busy ? 'Đang xử lý…' : `${buying ? 'Mua' : 'Bán'} · ${money(offer.unit_price * q, offer.currency)}`}
      </button>
    </div>
  );
}
