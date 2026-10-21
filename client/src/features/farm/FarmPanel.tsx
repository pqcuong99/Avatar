import { useEffect, useMemo, useState } from 'react';
import type { HarvestResult, Plot } from '../../api/types.ts';
import { serverNow, useStore } from '../../state/store.ts';
import { ConfirmDialog, Window, type ConfirmRequest } from '../../ui/Window.tsx';
import { ItemIcon } from '../../ui/ItemIcon.tsx';
import { countdown, duration, money, num } from '../../ui/format.ts';
import { act, refreshState } from '../actions.ts';

function useNow(ms = 1000) {
  const [now, setNow] = useState(serverNow());
  useEffect(() => {
    const t = setInterval(() => setNow(serverNow()), ms);
    return () => clearInterval(t);
  }, [ms]);
  return now;
}

function harvestText(results: HarvestResult[]): string {
  const bag = results.filter((r) => r.destination === 'BAG');
  const pending = results.filter((r) => r.destination === 'PENDING');
  const parts = [];
  if (bag.length) parts.push(`${bag.reduce((n, r) => n + r.quantity, 0)} vào túi`);
  if (pending.length) parts.push(`${pending.reduce((n, r) => n + r.quantity, 0)} vào Hàng chờ (túi không đủ chỗ cho cả lô)`);
  const lost = results.filter((r) => r.penalty_percent);
  if (lost.length) parts.push(`sâu làm giảm ${lost.map((r) => r.penalty_percent + '%').join(', ')}`);
  return `Thu hoạch ${results.length} ô: ${parts.join(', ')}.`;
}

// UI-11: plots numbered in the order batch actions use. Times come from the
// server; the local clock only counts down between refreshes.
export function FarmPanel({ onClose }: { onClose: () => void }) {
  const game = useStore((s) => s.game);
  const catalog = useStore((s) => s.catalog);
  const now = useNow();
  const [seed, setSeed] = useState<string>('');
  const [busy, setBusy] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null);
  const [selected, setSelected] = useState<number | null>(null);
  const crops = useMemo(() => (catalog?.crops ?? []).slice().sort((a, b) => a.sort_order - b.sort_order), [catalog]);
  const seedCount = (code: string) => (game?.inventory.slots ?? []).filter((s) => s.item_code === code).reduce((n, s) => n + s.quantity, 0);
  const plots = game?.farm.plots ?? [];

  // Pests appear at unannounced times: refresh on open and while open.
  useEffect(() => {
    void refreshState().catch(() => {});
    const t = setInterval(() => void refreshState().catch(() => {}), 15000);
    return () => clearInterval(t);
  }, []);
  // Refresh right when the next plant matures or a pest deadline passes.
  const nextEvent = useMemo(() => {
    const times = plots.flatMap((p) => p.planting ? [Date.parse(p.planting.matures_at), p.planting.pest?.active ? Date.parse(p.planting.pest.deadline_at) : Infinity] : []);
    return Math.min(...times.filter((t) => t > serverNow()), Infinity);
  }, [plots]);
  useEffect(() => {
    if (!isFinite(nextEvent)) return;
    const t = setTimeout(() => void refreshState().catch(() => {}), Math.max(500, nextEvent - serverNow() + 300));
    return () => clearTimeout(t);
  }, [nextEvent]);

  useEffect(() => {
    if (seed && seedCount(seed) > 0) return;
    const firstOwned = crops.find((c) => c.configured && seedCount(c.seed_item_code) > 0);
    if (firstOwned) setSeed(firstOwned.seed_item_code);
  }, [game, crops]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!game) return null;
  const crop = crops.find((c) => c.seed_item_code === seed);
  const emptyPlots = plots.filter((p) => !p.planting).length;
  const mature = plots.filter((p) => p.planting?.mature).length;
  const planted = plots.filter((p) => p.planting).length;
  const unlock = catalog?.plot_unlock;

  async function run<T>(key: string, path: string, body: Record<string, unknown>, success?: (r: T) => string) {
    setBusy(key);
    await act<T>(path, body, { success });
    setBusy(null);
  }

  function plant(p: Plot) {
    if (!crop) return;
    if (seedCount(seed) === 0) return;
    void run(`plot-${p.plot_no}`, `/api/v1/farm/plots/${p.plot_no}/plant`, { crop_code: crop.crop_code }, () => `Đã trồng ${crop.display_name} ở ô ${p.plot_no}.`);
  }

  function plotAction(p: Plot) {
    setSelected(p.plot_no);
    const pl = p.planting;
    if (!pl) return plant(p);
    if (pl.pest?.active) {
      return void run(`plot-${p.plot_no}`, `/api/v1/farm/plots/${p.plot_no}/catch-pest`, {}, () => 'Đã bắt sâu! +1 Sâu vào túi.');
    }
    if (pl.mature) {
      return void run<HarvestResult>(`plot-${p.plot_no}`, `/api/v1/farm/plots/${p.plot_no}/harvest`, {}, (r) => harvestText([r]));
    }
  }

  function uproot(p: Plot) {
    const pl = p.planting!;
    const name = crops.find((c) => c.crop_code === pl.crop_code)?.display_name ?? pl.crop_code;
    setConfirm({
      title: `Nhổ cây ở ô ${p.plot_no}?`,
      body: <>Cây <b>{name}</b> {pl.mature ? 'đã chín' : 'đang lớn'} sẽ bị bỏ đi. Không hoàn lại hạt, Xu hay nông sản.</>,
      confirmLabel: 'Nhổ cây',
      danger: true,
      onConfirm: () => void run(`plot-${p.plot_no}`, `/api/v1/farm/plots/${p.plot_no}/uproot`, { confirmed: true }, () => `Đã nhổ cây ở ô ${p.plot_no}.`),
    });
  }

  return (
    <Window title="Nông trại của tôi" onClose={onClose} wide>
      <div className="farm-toolbar">
        <label className="seed-pick">
          Hạt giống:
          <select value={seed} onChange={(e) => setSeed(e.target.value)}>
            <option value="">— chọn —</option>
            {crops.map((c) => (
              <option key={c.crop_code} value={c.seed_item_code} disabled={!c.configured}>
                {c.display_name} · có {num(seedCount(c.seed_item_code))}{c.configured ? ` · ${duration(c.config!.growth_seconds * 1000)}` : ' · chưa cấu hình'}
              </option>
            ))}
          </select>
        </label>
        <div className="row gap wrap">
          <button className="btn" disabled={!crop || seedCount(seed) === 0 || emptyPlots === 0 || !!busy} onClick={() => setConfirm({
            title: 'Trồng tất cả',
            body: <>Trồng <b>{crop!.display_name}</b> vào {Math.min(emptyPlots, seedCount(seed))} ô trống theo thứ tự số ô, dùng {Math.min(emptyPlots, seedCount(seed))} / {num(seedCount(seed))} hạt.</>,
            confirmLabel: 'Trồng',
            onConfirm: () => void run<{ seeds_used: number }>('plant-all', '/api/v1/farm/plant-all', { crop_code: crop!.crop_code, confirmed: true }, (r) => `Đã trồng ${r.seeds_used} ô.`),
          })}>Trồng tất cả</button>
          <button className="btn primary" disabled={mature === 0 || !!busy} onClick={() => void run<{ harvests: HarvestResult[] }>('harvest-all', '/api/v1/farm/harvest-all', {}, (r) => harvestText(r.harvests))}>
            Thu hoạch tất cả{mature ? ` (${mature})` : ''}
          </button>
          <button className="btn danger ghost" disabled={planted === 0 || !!busy} onClick={() => setConfirm({
            title: 'Nhổ tất cả?',
            body: <>Nhổ <b>{planted}</b> cây, cả cây non lẫn cây đã chín. Không hoàn lại gì.</>,
            confirmLabel: 'Nhổ tất cả',
            danger: true,
            onConfirm: () => void run<{ uprooted: number[] }>('uproot-all', '/api/v1/farm/uproot-all', { confirmed: true }, (r) => `Đã nhổ ${r.uprooted.length} cây.`),
          })}>Nhổ tất cả</button>
        </div>
      </div>

      <div className="farm-grid">
        {plots.map((p) => {
          const pl = p.planting;
          const c = pl ? crops.find((x) => x.crop_code === pl.crop_code) : undefined;
          const total = pl ? Date.parse(pl.matures_at) - Date.parse(pl.planted_at) : 1;
          const left = pl ? Date.parse(pl.matures_at) - now : 0;
          const progress = pl ? Math.min(1, Math.max(0, 1 - left / total)) : 0;
          const state = !pl ? 'empty' : pl.pest?.active ? 'pest' : pl.mature ? 'mature' : 'growing';
          return (
            <div key={p.plot_no} className={`plot ${state} ${selected === p.plot_no ? 'selected' : ''}`}>
              <button className="plot-main" onClick={() => plotAction(p)} disabled={busy === `plot-${p.plot_no}` || (state === 'empty' && (!crop || seedCount(seed) === 0)) || state === 'growing'}
                title={state === 'empty' ? (crop ? `Trồng ${crop.display_name}` : 'Chọn hạt giống để trồng') : ''}>
                <span className="plot-no">{p.plot_no}</span>
                {pl && c && (
                  <span className="plot-crop" style={{ transform: `scale(${0.45 + 0.55 * progress})` }}>
                    <ItemIcon code={c.crop_item_code} size={44} />
                  </span>
                )}
                {pl?.pest?.active && <span className="plot-pest" aria-label="Có sâu">🐛</span>}
                <span className="plot-label">
                  {state === 'empty' && (crop && seedCount(seed) > 0 ? `Trồng ${crop.display_name}` : 'Đất trống')}
                  {state === 'growing' && `${c?.display_name} · ${countdown(left)}`}
                  {state === 'mature' && `Thu hoạch ${c?.display_name}!`}
                  {state === 'pest' && `Bắt sâu! ${countdown(Date.parse(pl!.pest!.deadline_at) - now)}`}
                </span>
                {pl && !pl.mature && <span className="plot-bar"><i style={{ width: `${progress * 100}%` }} /></span>}
                {pl?.pest?.missed && <span className="plot-missed">Sâu phá −{pl.pest.penalty_percent}%</span>}
                {pl?.pest?.caught_at && <span className="plot-caught">Đã bắt sâu</span>}
              </button>
              {pl && <button className="plot-uproot" onClick={() => uproot(p)} title="Nhổ cây" aria-label={`Nhổ cây ô ${p.plot_no}`}>✕</button>}
            </div>
          );
        })}
        {unlock && (
          <button className="plot locked" disabled={!!busy || plots.length >= (unlock.max_plots ?? 0)} onClick={() => setConfirm({
            title: 'Mở thêm ô đất',
            body: <>Mở ô số {plots.length + 1} với giá <b>{money(unlock.price, unlock.currency)}</b>?</>,
            confirmLabel: 'Mở ô',
            onConfirm: () => void run<{ plot_no: number }>('unlock', '/api/v1/farm/unlock-plot', {}, (r) => `Đã mở ô ${r.plot_no}.`),
          })}>
            <span className="plot-no">{plots.length + 1}</span>
            <span className="plot-label">{plots.length >= (unlock.max_plots ?? 0) ? 'Đã tối đa' : `🔒 Mở ô · ${money(unlock.price, unlock.currency)}`}</span>
          </button>
        )}
      </div>
      <p className="muted small">
        Cây chín không héo. Sâu có thể xuất hiện một lần khi cây đang lớn — bắt kịp thời để không bị giảm 5–10% sản lượng.
        Thu hoạch mà túi không đủ chỗ cho cả lô thì cả lô vào Hàng chờ.
      </p>
      {confirm && <ConfirmDialog req={confirm} onClose={() => setConfirm(null)} />}
    </Window>
  );
}
