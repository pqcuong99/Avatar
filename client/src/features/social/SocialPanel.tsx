import { useCallback, useEffect, useRef, useState } from 'react';
import { del, get, post } from '../../api/http.ts';
import type { Friend, FriendRequest, Player } from '../../api/types.ts';
import { store, toast, useStore } from '../../state/store.ts';
import { Window } from '../../ui/Window.tsx';
import { explain } from '../../ui/format.ts';
import { refreshSocialCounters } from './socialApi.ts';

type Tab = 'friends' | 'requests' | 'search' | 'blocks';

// Friends (TH-07), requests, player search and the private-chat block list
// (TH-06) in one window.
export function SocialPanel({ onClose }: { onClose: () => void }) {
  const [tab, setTab] = useState<Tab>('friends');
  const incoming = useStore((s) => s.incomingRequests);
  return (
    <Window title="Bạn bè" onClose={onClose} wide>
      <div className="tabs four" role="tablist">
        <button role="tab" className={tab === 'friends' ? 'active' : ''} onClick={() => setTab('friends')}>Bạn bè</button>
        <button role="tab" className={tab === 'requests' ? 'active' : ''} onClick={() => setTab('requests')}>Lời mời{incoming ? ` (${incoming})` : ''}</button>
        <button role="tab" className={tab === 'search' ? 'active' : ''} onClick={() => setTab('search')}>Tìm người</button>
        <button role="tab" className={tab === 'blocks' ? 'active' : ''} onClick={() => setTab('blocks')}>Đã chặn</button>
      </div>
      {tab === 'friends' && <FriendsTab />}
      {tab === 'requests' && <RequestsTab />}
      {tab === 'search' && <SearchTab />}
      {tab === 'blocks' && <BlocksTab />}
    </Window>
  );
}

const openProfile = (id: string) => store.set({ panel: { profile: id } });

function FriendsTab() {
  const [data, setData] = useState<{ friends: Friend[]; max: number; my_room_id: string } | null>(null);
  const version = useStore((s) => s.friendsVersion);
  const load = useCallback(() => get<{ friends: Friend[]; max: number; my_room_id: string }>('/api/v1/friends').then((e) => setData(e.result)).catch((e) => toast(explain(e), 'error')), []);
  useEffect(() => { void load(); }, [load, version]);
  if (!data) return <p className="muted">Đang tải…</p>;
  if (data.friends.length === 0) return <p className="muted">Chưa có bạn bè. Tìm người chơi ở tab “Tìm người” hoặc nhấp vào nhân vật trên bản đồ.</p>;
  return (
    <>
      <p className="muted small">{data.friends.length}/{data.max} bạn</p>
      <ul className="people">
        {data.friends.map((f) => (
          <li key={f.id}>
            <span className={`dot ${f.presence.online ? 'on' : ''}`} aria-label={f.presence.online ? 'Trực tuyến' : 'Ngoại tuyến'} />
            <button className="link name" onClick={() => openProfile(f.id)}>{f.display_name}</button>
            <span className="muted small grow">{f.presence.online ? f.presence.room_id : 'ngoại tuyến'}</span>
            <button className="btn small" onClick={() => store.set({ panel: { pm: f.id } })}>💬</button>
            {f.presence.online && f.presence.room_id !== data.my_room_id && (
              <button className="btn small" onClick={async () => {
                try {
                  await post(`/api/v1/friends/${f.id}/join-room`);
                  toast(`Đã đến phòng ${f.presence.room_id}.`, 'success');
                  store.set({ panel: null });
                } catch (e) { toast(explain(e), 'error'); }
              }}>Đến cùng phòng</button>
            )}
          </li>
        ))}
      </ul>
    </>
  );
}

function RequestsTab() {
  const [data, setData] = useState<{ incoming: FriendRequest[]; outgoing: FriendRequest[] } | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const load = useCallback(() => get<{ incoming: FriendRequest[]; outgoing: FriendRequest[] }>('/api/v1/friends/requests').then((e) => setData(e.result)).catch(() => {}), []);
  const version = useStore((s) => s.friendsVersion);
  useEffect(() => { void load(); }, [load, version]);
  async function decide(id: string, action: 'accept' | 'reject' | 'cancel', ok: string) {
    setBusy(id);
    try {
      await post(`/api/v1/friends/requests/${id}/${action}`);
      toast(ok, 'success');
      store.set((s) => ({ friendsVersion: s.friendsVersion + 1 }));
    } catch (e) {
      toast(explain(e), 'error');
    } finally {
      setBusy(null);
      void refreshSocialCounters();
      void load();
    }
  }
  if (!data) return <p className="muted">Đang tải…</p>;
  return (
    <>
      <h3 className="section-title">Lời mời nhận được</h3>
      {data.incoming.length === 0 ? <p className="muted">Không có.</p> : (
        <ul className="people">
          {data.incoming.map((r) => (
            <li key={r.id}>
              <button className="link name grow" onClick={() => openProfile(r.player.id)}>{r.player.display_name}</button>
              <button className="btn small primary" disabled={busy === r.id} onClick={() => void decide(r.id, 'accept', `Đã kết bạn với ${r.player.display_name}.`)}>Chấp nhận</button>
              <button className="btn small" disabled={busy === r.id} onClick={() => void decide(r.id, 'reject', 'Đã từ chối.')}>Từ chối</button>
            </li>
          ))}
        </ul>
      )}
      <h3 className="section-title">Lời mời đã gửi</h3>
      {data.outgoing.length === 0 ? <p className="muted">Không có.</p> : (
        <ul className="people">
          {data.outgoing.map((r) => (
            <li key={r.id}>
              <button className="link name grow" onClick={() => openProfile(r.player.id)}>{r.player.display_name}</button>
              <button className="btn small" disabled={busy === r.id} onClick={() => void decide(r.id, 'cancel', 'Đã hủy lời mời.')}>Hủy</button>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

function SearchTab() {
  const [q, setQ] = useState('');
  const [results, setResults] = useState<Player[] | null>(null);
  useEffect(() => {
    const term = q.trim();
    if (!term) { setResults(null); return; }
    const t = setTimeout(() => {
      get<{ players: Player[] }>(`/api/v1/players/search?q=${encodeURIComponent(term)}`).then((e) => setResults(e.result.players)).catch(() => setResults([]));
    }, 300);
    return () => clearTimeout(t);
  }, [q]);
  return (
    <>
      <input className="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Tên nhân vật (có dấu hay không đều được) hoặc ID đầy đủ" autoFocus maxLength={64} />
      {results && results.length === 0 && <p className="muted">Không tìm thấy.</p>}
      <ul className="people">
        {results?.map((p) => (
          <li key={p.id}>
            <button className="link name grow" onClick={() => openProfile(p.id)}>{p.display_name}</button>
            <button className="btn small" onClick={() => openProfile(p.id)}>Xem hồ sơ</button>
          </li>
        ))}
      </ul>
    </>
  );
}

interface Blocked extends Player { blocked_at: string }

// TH-06: newest first, 50 then 20 more; search by name ignoring case and
// accents, or by full id. Closing the window stops loading but never cancels
// an unblock already sent (TH-06-008).
function BlocksTab() {
  const [q, setQ] = useState('');
  const [items, setItems] = useState<Blocked[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const alive = useRef(true);
  useEffect(() => () => { alive.current = false; }, []);

  const load = useCallback(async (reset: boolean, from: string | null) => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (q.trim()) params.set('q', q.trim());
      if (!reset && from) params.set('cursor', from);
      const env = await get<{ items: Blocked[]; next_cursor: string | null }>(`/api/v1/private-chat/blocks?${params}`);
      if (!alive.current) return;
      setItems((prev) => (reset ? env.result.items : [...prev, ...env.result.items]));
      setCursor(env.result.next_cursor);
    } catch (e) {
      if (alive.current) toast(explain(e), 'error');
    } finally {
      if (alive.current) setLoading(false);
    }
  }, [q]);
  useEffect(() => {
    const t = setTimeout(() => void load(true, null), 250);
    return () => clearTimeout(t);
  }, [load]);

  async function unblock(p: Blocked) {
    try {
      await del(`/api/v1/private-chat/blocks/${p.id}`);
      toast(`Đã bỏ chặn ${p.display_name}.`, 'success');
      if (alive.current) setItems((prev) => prev.filter((x) => x.id !== p.id));
    } catch (e) {
      toast(explain(e), 'error');
    }
  }

  return (
    <>
      <input className="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Tìm theo tên hoặc ID đầy đủ" maxLength={64} />
      {items.length === 0 && !loading && <p className="muted">Danh sách chặn trống.</p>}
      <ul className="people">
        {items.map((p) => (
          <li key={p.id}>
            <button className="link name grow" onClick={() => openProfile(p.id)}>{p.display_name}</button>
            <span className="muted small">{new Date(p.blocked_at).toLocaleDateString('vi-VN')}</span>
            <button className="btn small" onClick={() => void unblock(p)}>Bỏ chặn</button>
          </li>
        ))}
      </ul>
      {cursor && <button className="btn ghost" disabled={loading} onClick={() => void load(false, cursor)}>{loading ? 'Đang tải…' : 'Tải thêm 20'}</button>}
    </>
  );
}
