import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { ApiError, get, post } from '../api/http.ts';
import type { Catalog, GameState } from '../api/types.ts';
import { explain, money, num } from '../ui/format.ts';

// UI-14: Admin console. Every action is checked again on the server (RBAC,
// TOTP session, two-person rule); hiding a button here is only convenience.

const API = '/admin/api/v1';

interface Me { id: string; login: string; roles: string[]; permissions: Record<string, boolean> }

export default function AdminApp() {
  const [me, setMe] = useState<Me | null | undefined>(undefined);
  const load = useCallback(() => get<Me>(`${API}/me`).then((e) => setMe(e.result)).catch(() => setMe(null)), []);
  useEffect(() => { void load(); }, [load]);
  if (me === undefined) return <div className="center-screen"><div className="spinner" /></div>;
  if (!me) return <AdminLogin onDone={() => void load()} />;
  return <Console me={me} onLogout={async () => { await post(`${API}/logout`).catch(() => {}); setMe(null); }} />;
}

function AdminLogin({ onDone }: { onDone: () => void }) {
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [challenge, setChallenge] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (!challenge) {
        const r = await post<{ challenge: string }>(`${API}/login`, { login, password });
        setChallenge(r.result.challenge);
      } else {
        await post(`${API}/login/totp`, { challenge, code });
        onDone();
      }
    } catch (err) {
      setError(explain(err));
      if (err instanceof ApiError && challenge) { setChallenge(null); setCode(''); }
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="auth-page admin-bg">
      <div className="auth-card">
        <h1 className="title">Quản trị Project Thanh</h1>
        <p className="muted small">Tài khoản Admin tách biệt với người chơi. Bắt buộc xác thực 2 lớp.</p>
        <form className="form" onSubmit={submit}>
          {!challenge ? (
            <>
              <label>Tên đăng nhập Admin<input value={login} onChange={(e) => setLogin(e.target.value)} autoComplete="username" /></label>
              <label>Mật khẩu<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" /></label>
            </>
          ) : (
            <label>Mã TOTP 6 số (hoặc mã khôi phục)<input value={code} onChange={(e) => setCode(e.target.value)} autoFocus autoComplete="one-time-code" maxLength={12} /></label>
          )}
          {error && <p className="form-error">{error}</p>}
          <button className="btn primary big" disabled={busy}>{busy ? 'Đang xử lý…' : challenge ? 'Xác nhận' : 'Tiếp tục'}</button>
        </form>
      </div>
    </main>
  );
}

type Tab = 'players' | 'adjustments' | 'ops' | 'audit';

function Console({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const can = (p: string) => !!me.permissions[p];
  const [tab, setTab] = useState<Tab>(can('player.read') ? 'players' : can('asset.adjust.propose') ? 'adjustments' : 'audit');
  const [target, setTarget] = useState<string>('');
  return (
    <div className="admin">
      <header className="admin-head">
        <b>🛠 Project Thanh · Admin</b>
        <nav className="admin-tabs">
          {can('player.read') && <button className={tab === 'players' ? 'active' : ''} onClick={() => setTab('players')}>Người chơi</button>}
          {(can('asset.adjust.propose') || can('asset.adjust.approve')) && <button className={tab === 'adjustments' ? 'active' : ''} onClick={() => setTab('adjustments')}>Điều chỉnh tài sản</button>}
          {(can('maintenance.manage') || can('config.publish')) && <button className={tab === 'ops' ? 'active' : ''} onClick={() => setTab('ops')}>Bảo trì &amp; cấu hình</button>}
          {can('audit.read') && <button className={tab === 'audit' ? 'active' : ''} onClick={() => setTab('audit')}>Nhật ký</button>}
        </nav>
        <span className="muted small">{me.login} · {me.roles.join(', ')}</span>
        <button className="btn small" onClick={onLogout}>Đăng xuất</button>
      </header>
      <main className="admin-body">
        {tab === 'players' && <Players me={me} onAdjust={(id) => { setTarget(id); setTab('adjustments'); }} />}
        {tab === 'adjustments' && <Adjustments me={me} target={target} />}
        {tab === 'ops' && <Ops me={me} />}
        {tab === 'audit' && <Audit />}
      </main>
    </div>
  );
}

interface PlayerRow { character_id: string; display_name: string; account_id: string; username: string; status: string; ban_reason: string | null; created_at: string }
interface LedgerRow { id: number; asset_kind: string; asset_ref: string | null; location: string; amount_delta: number; balance_after: number; reason_code: string; created_at: string }

function Players({ me, onAdjust }: { me: Me; onAdjust: (id: string) => void }) {
  const [q, setQ] = useState('');
  const [rows, setRows] = useState<PlayerRow[]>([]);
  const [sel, setSel] = useState<PlayerRow | null>(null);
  const [detail, setDetail] = useState<{ state: GameState; ledger: LedgerRow[] } | null>(null);
  const [reason, setReason] = useState('');
  const search = useCallback(() => get<{ players: PlayerRow[] }>(`${API}/players?q=${encodeURIComponent(q)}`).then((e) => setRows(e.result.players)).catch(() => {}), [q]);
  useEffect(() => { const t = setTimeout(() => void search(), 250); return () => clearTimeout(t); }, [search]);
  useEffect(() => {
    if (!sel) return;
    setDetail(null);
    get<{ state: GameState; ledger: LedgerRow[] }>(`${API}/players/${sel.character_id}`).then((e) => setDetail(e.result)).catch(() => {});
  }, [sel]);
  async function ban(action: 'ban' | 'unban') {
    try {
      await post(`${API}/accounts/${sel!.account_id}/${action}`, { reason });
      setReason('');
      await search();
      setSel((s) => s && { ...s, status: action === 'ban' ? 'banned' : 'active', ban_reason: action === 'ban' ? reason : null });
    } catch (e) { alert(explain(e)); }
  }
  return (
    <div className="admin-split">
      <section>
        <input className="search" placeholder="Tên nhân vật, tên đăng nhập hoặc ID" value={q} onChange={(e) => setQ(e.target.value)} />
        <table className="admin-table">
          <thead><tr><th>Nhân vật</th><th>Tài khoản</th><th>Trạng thái</th></tr></thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.character_id} className={sel?.character_id === r.character_id ? 'sel' : ''} onClick={() => setSel(r)}>
                <td>{r.display_name}</td><td>{r.username}</td><td>{r.status === 'banned' ? '⛔ khóa' : 'hoạt động'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      <section>
        {!sel ? <p className="muted">Chọn một người chơi.</p> : (
          <>
            <h3>{sel.display_name} <span className="muted small">({sel.username})</span></h3>
            <p className="muted small">Character ID: <code>{sel.character_id}</code></p>
            {detail ? (
              <>
                <p>
                  Ví: {money(detail.state.wallets.XU.balance, 'XU')} · {money(detail.state.wallets.DIAMOND.balance, 'DIAMOND')} ·
                  Túi {(detail.state.inventory.slots ?? []).length}/{detail.state.inventory.slot_cap} ô ·
                  Hàng chờ {(detail.state.pending_crops ?? []).length} dòng
                </p>
                <details>
                  <summary>Túi đồ</summary>
                  <ul>{(detail.state.inventory.slots ?? []).map((s) => <li key={s.slot_no}>Ô {s.slot_no}: {s.item_code} × {num(s.quantity)}</li>)}</ul>
                </details>
                <h4>Sổ cái (100 dòng gần nhất)</h4>
                <table className="admin-table small">
                  <thead><tr><th>#</th><th>Tài sản</th><th>Vị trí</th><th>Thay đổi</th><th>Sau</th><th>Lý do</th><th>Lúc</th></tr></thead>
                  <tbody>
                    {detail.ledger.map((l) => (
                      <tr key={l.id}><td>{l.id}</td><td>{l.asset_kind}{l.asset_ref ? `:${l.asset_ref}` : ''}</td><td>{l.location}</td>
                        <td className={l.amount_delta < 0 ? 'neg' : 'pos'}>{l.amount_delta > 0 ? '+' : ''}{num(l.amount_delta)}</td>
                        <td>{num(l.balance_after)}</td><td>{l.reason_code}</td><td>{new Date(l.created_at).toLocaleString('vi-VN')}</td></tr>
                    ))}
                  </tbody>
                </table>
              </>
            ) : <p className="muted">Đang tải…</p>}
            <div className="row gap wrap">
              {me.permissions['asset.adjust.propose'] && <button className="btn" onClick={() => onAdjust(sel.character_id)}>Đề xuất điều chỉnh tài sản</button>}
              {me.permissions['player.ban'] && (sel.status === 'banned' ? (
                <button className="btn" onClick={() => void ban('unban')}>Mở khóa ({sel.ban_reason})</button>
              ) : (
                <>
                  <input placeholder="Lý do khóa (hiển thị cho người chơi)" value={reason} onChange={(e) => setReason(e.target.value)} />
                  <button className="btn danger" disabled={reason.trim().length < 5} onClick={() => void ban('ban')}>Khóa tài khoản</button>
                </>
              ))}
            </div>
          </>
        )}
      </section>
    </div>
  );
}

interface Adjustment {
  id: string; target: { display_name?: string; username?: string; character_id?: string }; asset_kind: string; item_code: string | null;
  amount_delta: number; reason: string; proposer: string; approver: string | null; state: string; decision_note: string | null;
  result: unknown; created_at: string;
}

function Adjustments({ me, target }: { me: Me; target: string }) {
  const [list, setList] = useState<Adjustment[]>([]);
  const [filter, setFilter] = useState('pending');
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [form, setForm] = useState({ character_id: target, asset_kind: 'XU', item_code: '', amount_delta: 0, reason: '' });
  const load = useCallback(() => get<{ adjustments: Adjustment[] }>(`${API}/adjustments?state=${filter}`).then((e) => setList(e.result.adjustments)).catch(() => {}), [filter]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { get<Catalog>(`${API}/catalog`).then((e) => setCatalog(e.result)).catch(() => {}); }, []);
  async function propose(e: FormEvent) {
    e.preventDefault();
    try {
      await post(`${API}/adjustments`, { ...form, item_code: form.asset_kind === 'ITEM' ? form.item_code : '' });
      setForm({ ...form, amount_delta: 0, reason: '' });
      setFilter('pending');
      await load();
    } catch (err) { alert(explain(err)); }
  }
  async function decide(id: string, action: 'approve' | 'reject') {
    const note = prompt(action === 'approve' ? 'Ghi chú duyệt (tùy chọn)' : 'Lý do từ chối') ?? '';
    try {
      await post(`${API}/adjustments/${id}/${action}`, { note });
      await load();
    } catch (err) { alert(explain(err)); }
  }
  return (
    <div className="admin-split">
      {me.permissions['asset.adjust.propose'] && (
        <section>
          <h3>Đề xuất điều chỉnh</h3>
          <p className="muted small">Một Admin khác có quyền duyệt phải phê duyệt; người đề xuất không thể tự duyệt (P0-144).</p>
          <form className="form" onSubmit={propose}>
            <label>Character ID<input value={form.character_id} onChange={(e) => setForm({ ...form, character_id: e.target.value.trim() })} /></label>
            <label>Loại tài sản
              <select value={form.asset_kind} onChange={(e) => setForm({ ...form, asset_kind: e.target.value })}>
                <option value="XU">Xu</option><option value="DIAMOND">Kim Cương</option><option value="ITEM">Vật phẩm</option>
              </select>
            </label>
            {form.asset_kind === 'ITEM' && (
              <label>Vật phẩm
                <select value={form.item_code} onChange={(e) => setForm({ ...form, item_code: e.target.value })}>
                  <option value="">— chọn —</option>
                  {(catalog?.items ?? []).map((i) => <option key={i.item_code} value={i.item_code}>{i.display_name} ({i.item_code})</option>)}
                </select>
              </label>
            )}
            <label>Số lượng (+ cộng, − trừ)<input type="number" value={form.amount_delta} onChange={(e) => setForm({ ...form, amount_delta: Number(e.target.value) })} /></label>
            <label>Lý do (bắt buộc, lưu nhật ký)<input value={form.reason} onChange={(e) => setForm({ ...form, reason: e.target.value })} /></label>
            <button className="btn primary" disabled={!form.character_id || !form.amount_delta || form.reason.trim().length < 5}>Gửi đề xuất</button>
          </form>
        </section>
      )}
      <section>
        <div className="row gap">
          <h3 className="grow">Yêu cầu</h3>
          <select value={filter} onChange={(e) => setFilter(e.target.value)}>
            <option value="pending">Chờ duyệt</option><option value="executed">Đã thực hiện</option>
            <option value="rejected">Từ chối</option><option value="failed">Thất bại</option><option value="">Tất cả</option>
          </select>
        </div>
        <table className="admin-table">
          <thead><tr><th>Đối tượng</th><th>Thay đổi</th><th>Lý do</th><th>Đề xuất</th><th>Trạng thái</th><th /></tr></thead>
          <tbody>
            {list.map((a) => (
              <tr key={a.id}>
                <td>{a.target.display_name}<div className="muted small">{a.target.username}</div></td>
                <td className={a.amount_delta < 0 ? 'neg' : 'pos'}>{a.amount_delta > 0 ? '+' : ''}{num(a.amount_delta)} {a.asset_kind === 'ITEM' ? a.item_code : a.asset_kind}</td>
                <td>{a.reason}</td>
                <td>{a.proposer}{a.approver && <div className="muted small">duyệt: {a.approver}</div>}</td>
                <td>{a.state}{a.decision_note && <div className="muted small">{a.decision_note}</div>}</td>
                <td>
                  {a.state === 'pending' && me.permissions['asset.adjust.approve'] && (a.proposer === me.login
                    ? <span className="muted small">không tự duyệt</span>
                    : <div className="row gap"><button className="btn small primary" onClick={() => void decide(a.id, 'approve')}>Duyệt</button><button className="btn small" onClick={() => void decide(a.id, 'reject')}>Từ chối</button></div>)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}

interface Version { id: number; label: string; state: string; notes: string; created_at: string; published_at: string | null }

function Ops({ me }: { me: Me }) {
  const [m, setM] = useState<{ active: boolean; message: string } | null>(null);
  const [msg, setMsg] = useState('');
  const [versions, setVersions] = useState<Version[]>([]);
  const [editing, setEditing] = useState<{ id: number; label: string; text: string; readOnly: boolean } | null>(null);
  const loadAll = useCallback(() => {
    get<{ active: boolean; message: string }>(`${API}/maintenance`).then((e) => setM(e.result)).catch(() => {});
    if (me.permissions['config.publish']) get<{ versions: Version[] }>(`${API}/config/versions`).then((e) => setVersions(e.result.versions)).catch(() => {});
  }, [me]);
  useEffect(() => { loadAll(); }, [loadAll]);
  async function act(fn: () => Promise<unknown>) {
    try { await fn(); loadAll(); } catch (e) { alert(explain(e)); }
  }
  async function open(v: Version, asNewDraft: boolean) {
    const e = await get<unknown>(`${API}/config/versions/${v.id}`);
    setEditing({ id: asNewDraft ? 0 : v.id, label: asNewDraft ? `${v.label}-next` : v.label, text: JSON.stringify(e.result, null, 2), readOnly: !asNewDraft && v.state !== 'draft' });
  }
  return (
    <div className="admin-split">
      <section>
        <h3>Bảo trì</h3>
        {m && <p>Trạng thái: <b>{m.active ? `🔧 Đang bảo trì — ${m.message}` : '✅ Đang mở'}</b></p>}
        {me.permissions['maintenance.manage'] && m && (m.active ? (
          <button className="btn primary" onClick={() => void act(() => post(`${API}/maintenance`, { active: false, message: '' }))}>Kết thúc bảo trì</button>
        ) : (
          <div className="row gap wrap">
            <input placeholder="Thông báo cho người chơi" value={msg} onChange={(e) => setMsg(e.target.value)} />
            <button className="btn danger" onClick={() => void act(() => post(`${API}/maintenance`, { active: true, message: msg }))}>Bắt đầu bảo trì</button>
          </div>
        ))}
        <p className="muted small">Trong lúc bảo trì: giao dịch tài sản của người chơi bị tạm dừng, có thể phát hành cấu hình mới. Hãy sao lưu DB trước khi phát hành.</p>
        {me.permissions['config.publish'] && (
          <>
            <h3>Phiên bản cấu hình</h3>
            <table className="admin-table">
              <thead><tr><th>#</th><th>Nhãn</th><th>Trạng thái</th><th /></tr></thead>
              <tbody>
                {versions.map((v) => (
                  <tr key={v.id}>
                    <td>{v.id}</td><td>{v.label}</td><td>{v.state}</td>
                    <td className="row gap">
                      <button className="btn small" onClick={() => void open(v, false)}>{v.state === 'draft' ? 'Sửa' : 'Xem'}</button>
                      <button className="btn small" onClick={() => void open(v, true)}>Sao thành nháp</button>
                      {v.state === 'draft' && <button className="btn small primary" disabled={!m?.active} title={m?.active ? '' : 'Cần bật bảo trì'}
                        onClick={() => confirm(`Phát hành phiên bản ${v.label}? Giá/NPC mới áp dụng ngay khi kết thúc bảo trì.`) && void act(() => post(`${API}/config/versions/${v.id}/publish`))}>Phát hành</button>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </section>
      <section>
        {editing ? (
          <>
            <h3>{editing.readOnly ? 'Xem' : editing.id ? 'Sửa bản nháp' : 'Bản nháp mới'}: {editing.label}</h3>
            {!editing.readOnly && editing.id === 0 && <input value={editing.label} onChange={(e) => setEditing({ ...editing, label: e.target.value })} placeholder="Nhãn phiên bản" />}
            <textarea className="json" value={editing.text} readOnly={editing.readOnly} onChange={(e) => setEditing({ ...editing, text: e.target.value })} spellCheck={false} />
            {!editing.readOnly && (
              <button className="btn primary" onClick={() => void act(async () => {
                let content: unknown;
                try { content = JSON.parse(editing.text); } catch { throw new ApiError(422, { code: 'VALIDATION_FAILED', message: 'JSON không hợp lệ.' }); }
                const r = await post<{ id: number }>(`${API}/config/drafts`, { id: editing.id, label: editing.label, content });
                setEditing({ ...editing, id: r.result.id });
              })}>Lưu bản nháp</button>
            )}
          </>
        ) : <p className="muted">Chọn một phiên bản để xem hoặc sao thành bản nháp. Phiên bản đã phát hành không thể sửa.</p>}
      </section>
    </div>
  );
}

interface AuditEntry { id: number; admin: string | null; action: string; object: string; object_id: string | null; details: unknown; created_at: string }

function Audit() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  useEffect(() => { get<{ entries: AuditEntry[] }>(`${API}/audit`).then((e) => setEntries(e.result.entries)).catch(() => {}); }, []);
  return (
    <table className="admin-table">
      <thead><tr><th>#</th><th>Lúc</th><th>Admin</th><th>Hành động</th><th>Đối tượng</th><th>Chi tiết</th></tr></thead>
      <tbody>
        {entries.map((e) => (
          <tr key={e.id}><td>{e.id}</td><td>{new Date(e.created_at).toLocaleString('vi-VN')}</td><td>{e.admin ?? 'hệ thống'}</td>
            <td>{e.action}</td><td>{e.object} {e.object_id && <code>{e.object_id.slice(0, 12)}</code>}</td><td><code>{JSON.stringify(e.details)}</code></td></tr>
        ))}
      </tbody>
    </table>
  );
}
