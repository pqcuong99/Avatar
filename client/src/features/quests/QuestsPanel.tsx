import { useCallback, useEffect, useState } from 'react';
import { get } from '../../api/http.ts';
import type { QuestsView } from '../../api/types.ts';
import { useStore } from '../../state/store.ts';
import { Window } from '../../ui/Window.tsx';
import { CurrencyIcon } from '../../ui/ItemIcon.tsx';
import { clock, num } from '../../ui/format.ts';
import { act } from '../actions.ts';
import { refreshQuestBadge } from './questApi.ts';

// UI-13: today's personal quests; progress only ever comes from the server.
export function QuestsPanel({ onClose }: { onClose: () => void }) {
  const [view, setView] = useState<QuestsView | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const game = useStore((s) => s.game);
  const load = useCallback(() => {
    get<QuestsView>('/api/v1/quests/daily').then((e) => setView(e.result)).catch(() => {});
    void refreshQuestBadge();
  }, []);
  useEffect(() => { load(); }, [load, game]);

  if (!view) return <Window title="Nhiệm vụ ngày" onClose={onClose}><p className="muted">Đang tải…</p></Window>;
  return (
    <Window title="Nhiệm vụ ngày" onClose={onClose}>
      {!view.enabled ? (
        <p className="muted">Nhiệm vụ ngày chưa được Admin cấu hình.</p>
      ) : (
        <>
          <p className="muted small">Làm mới lúc {view.resets_at ? clock(view.resets_at) : '—'} · phần thưởng nhận một lần mỗi ngày.</p>
          <ul className="quests">
            {view.quests.map((q) => {
              const pct = Math.min(100, (q.progress / q.target_quantity) * 100);
              const done = !!q.completed_at;
              return (
                <li key={q.quest_code} className={q.claimed_at ? 'claimed' : done ? 'done' : ''}>
                  <div className="grow">
                    <b>{q.title}</b>
                    <div className="quest-bar"><i style={{ width: `${pct}%` }} /></div>
                    <span className="muted small">{q.progress}/{q.target_quantity}</span>
                  </div>
                  <span className="reward"><CurrencyIcon currency={q.reward_currency} /> {num(q.reward_amount)}</span>
                  {q.claimed_at ? <span className="tag green">Đã nhận</span> : (
                    <button className="btn small primary" disabled={!done || busy === q.quest_code} onClick={async () => {
                      setBusy(q.quest_code);
                      await act<{ credited: number; pending: number }>(`/api/v1/quests/daily/${q.quest_code}/claim`, {}, {
                        success: (r) => `Nhận ${num(r.credited)} ${q.reward_currency === 'XU' ? 'Xu' : 'KC'}${r.pending ? `; ${num(r.pending)} KC vượt giới hạn vào Hàng chờ` : ''}.`,
                      });
                      setBusy(null);
                      load();
                    }}>Nhận</button>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}
    </Window>
  );
}
