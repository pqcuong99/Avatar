import { useMemo, useRef, useState, type FormEvent } from 'react';
import { ApiError, command, newRequestId } from '../../api/http.ts';
import { explain } from '../../ui/format.ts';
import { checkName, MAX_GRAPHEMES, PROBLEM_TEXT, type NameProblem } from './nameRules.ts';
import { CharacterPreview } from './CharacterPreview.tsx';

// UI-03: one character per account, never deleted or recreated (P0-018/019).
export function CreateCharacter({ onCreated, onLogout }: { onCreated: () => void; onLogout: () => void }) {
  const [raw, setRaw] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // One request id per attempt with the same name, so a retry after a network
  // error can never create a second character (A-02).
  const requestId = useRef<{ name: string; id: string } | null>(null);
  const check = useMemo(() => checkName(raw), [raw]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (check.problem || busy) return;
    setBusy(true);
    setError(null);
    if (requestId.current?.name !== check.name) requestId.current = { name: check.name, id: newRequestId() };
    try {
      await command('/api/v1/characters', { display_name: check.name }, requestId.current.id);
      onCreated();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'CHARACTER_NAME_TAKEN') setError('Tên này đã có người dùng (không phân biệt hoa/thường).');
      else if (err instanceof ApiError && err.code === 'CHARACTER_NAME_INVALID') setError(PROBLEM_TEXT[(err.details.problem as NameProblem) ?? 'invalid_char']);
      else if (err instanceof ApiError && err.code === 'CHARACTER_ALREADY_EXISTS') onCreated();
      else setError(explain(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="auth-page">
      <div className="auth-card">
        <h1 className="title">Tạo nhân vật</h1>
        <p className="muted">Mỗi tài khoản có một nhân vật. Tên là duy nhất trong game, không phân biệt hoa/thường nhưng phân biệt dấu (Thanh ≠ Thành).</p>
        <CharacterPreview seed={check.name || 'preview'} name={check.name || 'Tên của bạn'} />
        <form onSubmit={submit} className="form">
          <label>
            Tên nhân vật
            <input value={raw} onChange={(e) => setRaw(e.target.value)} maxLength={64} autoFocus placeholder="Ví dụ: Thành ★" />
            <small className={`hint ${raw && check.problem ? 'bad' : ''}`}>
              {raw && check.problem ? PROBLEM_TEXT[check.problem] : `${check.length}/${MAX_GRAPHEMES} ký tự · chữ tiếng Việt, số, dấu cách, _ - . ★ ♥ ☆ ♡`}
            </small>
          </label>
          {error && <p className="form-error" role="alert">{error}</p>}
          <button className="btn primary big" disabled={!!check.problem || busy}>{busy ? 'Đang tạo…' : 'Tạo nhân vật'}</button>
          <p className="muted small">Khởi đầu: 5.000 Xu, 10 ô túi, 6 ô đất.</p>
        </form>
        <button className="link" onClick={onLogout}>Đăng xuất</button>
      </div>
    </main>
  );
}
