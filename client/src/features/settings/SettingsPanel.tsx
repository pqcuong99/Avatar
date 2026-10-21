import { useCallback, useEffect, useState } from 'react';
import { get, post } from '../../api/http.ts';
import { toast } from '../../state/store.ts';
import { Window } from '../../ui/Window.tsx';
import { explain } from '../../ui/format.ts';

interface Status {
  email: string | null;
  email_verified: boolean;
  totp_enabled: boolean;
  recovery_codes_left: number;
  email_available: boolean;
}

// Account security: optional 2FA (P0-146) and the optional recovery email
// (P0-011). Logout lives here too.
export function SettingsPanel({ onClose, onLogout }: { onClose: () => void; onLogout: () => void }) {
  const [st, setSt] = useState<Status | null>(null);
  const [enroll, setEnroll] = useState<{ secret: string; otpauth_url: string; qr_png: string } | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [email, setEmail] = useState('');
  const [emailCode, setEmailCode] = useState('');
  const [busy, setBusy] = useState(false);
  const load = useCallback(() => get<Status>('/api/v1/auth/security').then((e) => setSt(e.result)).catch(() => {}), []);
  useEffect(() => { void load(); }, [load]);

  async function run<T>(fn: () => Promise<T>, ok?: string): Promise<T | null> {
    setBusy(true);
    try {
      const r = await fn();
      if (ok) toast(ok, 'success');
      return r;
    } catch (e) {
      toast(explain(e), 'error');
      return null;
    } finally {
      setBusy(false);
      void load();
    }
  }

  return (
    <Window title="Cài đặt tài khoản" onClose={onClose}>
      {!st ? <p className="muted">Đang tải…</p> : (
        <div className="settings">
          <section>
            <h3 className="section-title">Xác thực 2 lớp (TOTP)</h3>
            {st.totp_enabled ? (
              <>
                <p>Đang bật. Còn {st.recovery_codes_left} mã khôi phục chưa dùng.</p>
                <div className="row gap wrap">
                  <input type="password" placeholder="Mật khẩu" value={password} onChange={(e) => setPassword(e.target.value)} />
                  <input placeholder="Mã 6 số hoặc mã khôi phục" value={code} onChange={(e) => setCode(e.target.value)} maxLength={12} />
                  <button className="btn danger ghost" disabled={busy || !password || code.length < 6} onClick={() => void run(async () => {
                    await post('/api/v1/auth/totp/disable', { password, code });
                    setPassword('');
                    setCode('');
                  }, 'Đã tắt xác thực 2 lớp.')}>Tắt 2 lớp</button>
                </div>
              </>
            ) : enroll ? (
              <>
                <p>Quét mã QR bằng ứng dụng xác thực (Google Authenticator, Aegis, …) hoặc nhập khóa: <code>{enroll.secret}</code></p>
                <img src={enroll.qr_png} alt="Mã QR TOTP" width={160} height={160} className="qr" />
                <div className="row gap">
                  <input placeholder="Mã 6 số" value={code} onChange={(e) => setCode(e.target.value)} maxLength={6} inputMode="numeric" />
                  <button className="btn primary" disabled={busy || code.length !== 6} onClick={() => void run(async () => {
                    const e = await post<{ recovery_codes: string[] }>('/api/v1/auth/totp/enable', { code });
                    setCodes(e.result.recovery_codes);
                    setEnroll(null);
                    setCode('');
                  }, 'Đã bật xác thực 2 lớp.')}>Bật</button>
                </div>
              </>
            ) : (
              <>
                <p className="muted">Không bắt buộc. Khi bật, đăng nhập cần thêm mã từ ứng dụng xác thực.</p>
                <button className="btn" disabled={busy} onClick={() => void run(async () => {
                  const e = await post<{ secret: string; otpauth_url: string; qr_png: string }>('/api/v1/auth/totp/enroll');
                  setEnroll(e.result);
                })}>Thiết lập 2 lớp</button>
              </>
            )}
            {codes && (
              <div className="notice">
                <b>Mã khôi phục — chỉ hiện một lần, hãy lưu lại:</b>
                <div className="codes">{codes.map((c) => <code key={c}>{c}</code>)}</div>
                <button className="btn small" onClick={() => setCodes(null)}>Tôi đã lưu</button>
              </div>
            )}
          </section>
          <section>
            <h3 className="section-title">Email khôi phục</h3>
            {!st.email_available ? (
              <p className="muted">Chức năng email chưa được cấu hình trên máy chủ này. Khi quên mật khẩu, hãy liên hệ Admin.</p>
            ) : (
              <>
                <p>{st.email ? <>{st.email} · {st.email_verified ? '✅ đã xác minh' : '⏳ chưa xác minh'}</> : <span className="muted">Chưa có (không bắt buộc).</span>}</p>
                <div className="row gap wrap">
                  <input type="email" placeholder="email@vi-du.vn" value={email} onChange={(e) => setEmail(e.target.value)} />
                  <button className="btn" disabled={busy || !email} onClick={() => void run(() => post('/api/v1/auth/email', { email }), 'Đã gửi mã xác minh.')}>Gửi mã</button>
                </div>
                {st.email && !st.email_verified && (
                  <div className="row gap">
                    <input placeholder="Mã xác minh" value={emailCode} onChange={(e) => setEmailCode(e.target.value)} maxLength={6} inputMode="numeric" />
                    <button className="btn primary" disabled={busy || emailCode.length !== 6} onClick={() => void run(() => post('/api/v1/auth/email/verify', { code: emailCode }), 'Đã xác minh email.')}>Xác minh</button>
                  </div>
                )}
              </>
            )}
          </section>
          <section className="row between">
            <span className="muted small">Đăng nhập ở nơi khác sẽ đăng xuất phiên này.</span>
            <button className="btn danger" onClick={onLogout}>Đăng xuất</button>
          </section>
        </div>
      )}
    </Window>
  );
}
