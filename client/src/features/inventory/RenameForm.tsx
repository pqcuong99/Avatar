import { useMemo, useState } from 'react';
import { store } from '../../state/store.ts';
import { act } from '../actions.ts';
import { checkName, MAX_GRAPHEMES, PROBLEM_TEXT } from '../character/nameRules.ts';

// Rename Card (P0-026..029): same rules as at creation, no cooldown; the card
// is spent only when the new name is accepted.
export function RenameForm() {
  const [raw, setRaw] = useState('');
  const [busy, setBusy] = useState(false);
  const check = useMemo(() => checkName(raw), [raw]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (check.problem || busy) return;
    setBusy(true);
    const r = await act<{ old_name: string; new_name: string }>('/api/v1/characters/rename', { display_name: check.name },
      { success: (r) => `Đã đổi tên thành ${r.new_name}.` });
    setBusy(false);
    if (r) {
      const me = store.get().me;
      if (me) store.set({ me: { ...me, name: r.new_name } });
      setRaw('');
    }
  }

  return (
    <form className="rename-form" onSubmit={(e) => void submit(e)}>
      <div className="row gap">
        <input placeholder="Tên mới" value={raw} onChange={(e) => setRaw(e.target.value)} maxLength={64} aria-label="Tên mới" />
        <button className="btn primary" disabled={!!check.problem || busy}>{busy ? 'Đang đổi…' : 'Đổi tên'}</button>
      </div>
      <small className={`hint ${raw && check.problem ? 'bad' : ''}`}>
        {raw && check.problem ? PROBLEM_TEXT[check.problem] : `${check.length}/${MAX_GRAPHEMES} ký tự · dùng 1 thẻ khi đổi thành công`}
      </small>
    </form>
  );
}
