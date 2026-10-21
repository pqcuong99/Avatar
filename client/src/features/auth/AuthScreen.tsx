import { useState, type FormEvent } from 'react';
import { ApiError, post } from '../../api/http.ts';
import { explain } from '../../ui/format.ts';

type Mode = 'login' | 'register' | 'totp' | 'forgot' | 'reset';

// UI-01 / UI-02. Login names are ASCII for now (M1 v0.2); the name other
// players see is the character name chosen next.
export function AuthScreen({ onLoggedIn }: { onLoggedIn: () => void }) {
  const [mode, setMode] = useState<Mode>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [password2, setPassword2] = useState('');
  const [code, setCode] = useState('');
  const [challenge, setChallenge] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const bytes = (s: string) => new TextEncoder().encode(s).length;
  const usernameOk = /^[A-Za-z0-9_]{3,32}$/.test(username);
  const passwordOk = bytes(password) >= 8 && bytes(password) <= 72;
  const canSubmit = !busy && (
    mode === 'login' ? usernameOk && password.length > 0
      : mode === 'register' ? usernameOk && passwordOk && password === password2
        : mode === 'totp' ? code.trim().length >= 6
          : mode === 'forgot' ? usernameOk
            : usernameOk && code.trim().length === 6 && passwordOk && password === password2);

  function go(m: Mode) {
    setMode(m);
    setError(null);
    setInfo(null);
    setCode('');
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true);
    setError(null);
    try {
      if (mode === 'register') {
        await post('/api/v1/auth/register', { username, password });
        setInfo('Đã tạo tài khoản. Đang đăng nhập…');
      }
      if (mode === 'login' || mode === 'register') {
        const env = await post<{ totp_required?: boolean; challenge?: string }>('/api/v1/auth/login', { username, password });
        if (env.result.totp_required && env.result.challenge) {
          setChallenge(env.result.challenge);
          go('totp');
          return;
        }
        onLoggedIn();
      } else if (mode === 'totp') {
        await post('/api/v1/auth/login/totp', { challenge, code: code.trim() });
        onLoggedIn();
      } else if (mode === 'forgot') {
        const env = await post<{ message: string }>('/api/v1/auth/password/reset-request', { username });
        go('reset');
        setInfo(env.result.message);
      } else if (mode === 'reset') {
        await post('/api/v1/auth/password/reset-confirm', { username, code: code.trim(), new_password: password });
        go('login');
        setPassword('');
        setInfo('Đã đổi mật khẩu. Mọi phiên đăng nhập cũ đã bị đăng xuất — hãy đăng nhập lại.');
      }
    } catch (err) {
      if (err instanceof ApiError && err.code === 'USERNAME_TAKEN') setError('Tên đăng nhập đã có người dùng.');
      else if (err instanceof ApiError && err.code === 'INVALID_CREDENTIALS') setError('Sai tên đăng nhập hoặc mật khẩu.');
      else if (err instanceof ApiError && err.code === 'INVALID_CODE') setError('Mã không đúng hoặc đã hết hạn.');
      else if (err instanceof ApiError && err.code === 'ACCOUNT_BANNED') {
        const reason = err.details.reason ? ` Lý do: ${String(err.details.reason)}` : '';
        setError(`Tài khoản đang bị khóa.${reason}`);
      } else setError(explain(err));
    } finally {
      setBusy(false);
    }
  }

  const tabbed = mode === 'login' || mode === 'register';
  return (
    <main className="auth-page">
      <div className="auth-card">
        <div className="brand">
          <div className="brand-logo" aria-hidden="true">🌱</div>
          <h1>Project Thanh</h1>
          <p>Thị trấn nhỏ · nông trại · bạn bè</p>
        </div>
        {tabbed && (
          <div className="tabs" role="tablist">
            <button role="tab" aria-selected={mode === 'login'} className={mode === 'login' ? 'active' : ''} onClick={() => go('login')}>Đăng nhập</button>
            <button role="tab" aria-selected={mode === 'register'} className={mode === 'register' ? 'active' : ''} onClick={() => go('register')}>Đăng ký</button>
          </div>
        )}
        {mode === 'totp' && <h2 className="title">Xác thực 2 lớp</h2>}
        {(mode === 'forgot' || mode === 'reset') && <h2 className="title">Quên mật khẩu</h2>}
        <form onSubmit={submit} className="form">
          {mode !== 'totp' && (
            <label>
              Tên đăng nhập
              <input value={username} onChange={(e) => setUsername(e.target.value.trim())} autoComplete="username" maxLength={32} required />
              {mode === 'register' && username && !usernameOk && <small className="hint bad">3–32 ký tự: chữ không dấu, số, dấu gạch dưới.</small>}
            </label>
          )}
          {(mode === 'totp' || mode === 'reset') && (
            <label>
              {mode === 'totp' ? 'Mã 6 số từ ứng dụng xác thực (hoặc mã khôi phục)' : 'Mã 6 số trong email (hiệu lực 10 phút)'}
              <input value={code} onChange={(e) => setCode(e.target.value)} inputMode={mode === 'reset' ? 'numeric' : 'text'} autoComplete="one-time-code" maxLength={12} autoFocus required />
            </label>
          )}
          {mode !== 'totp' && mode !== 'forgot' && (
            <label>
              {mode === 'reset' ? 'Mật khẩu mới' : 'Mật khẩu'}
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete={mode === 'login' ? 'current-password' : 'new-password'} required />
              {(mode === 'register' || mode === 'reset') && password && !passwordOk && <small className="hint bad">Từ 8 đến 72 byte.</small>}
            </label>
          )}
          {(mode === 'register' || mode === 'reset') && (
            <label>
              Nhập lại mật khẩu
              <input type="password" value={password2} onChange={(e) => setPassword2(e.target.value)} autoComplete="new-password" required />
              {password2 && password2 !== password && <small className="hint bad">Mật khẩu nhập lại chưa khớp.</small>}
            </label>
          )}
          {mode === 'register' && (
            <p className="notice">
              Đây là bản thử nghiệm: dữ liệu có thể bị xóa khi kết thúc thử nghiệm. Đừng dùng lại mật khẩu của tài khoản khác.
              Có thể thêm email khôi phục trong Cài đặt sau khi vào game.
            </p>
          )}
          {mode === 'forgot' && (
            <p className="notice">Mã sẽ được gửi tới email khôi phục đã xác minh. Không có email? Liên hệ Admin để được hỗ trợ xác minh quyền sở hữu.</p>
          )}
          {error && <p className="form-error" role="alert">{error}</p>}
          {info && <p className="form-info">{info}</p>}
          <button className="btn primary big" disabled={!canSubmit}>
            {busy ? 'Đang xử lý…' : ({ login: 'Vào game', register: 'Tạo tài khoản', totp: 'Xác nhận', forgot: 'Gửi mã', reset: 'Đổi mật khẩu' } as const)[mode]}
          </button>
        </form>
        <div className="row between">
          {mode === 'login' ? <button className="link" onClick={() => go('forgot')}>Quên mật khẩu?</button> : !tabbed && <button className="link" onClick={() => go('login')}>← Quay lại đăng nhập</button>}
        </div>
      </div>
    </main>
  );
}
