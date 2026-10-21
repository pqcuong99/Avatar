import { useCallback, useEffect, useState } from 'react';
import { ApiError, del, get, post } from '../../api/http.ts';
import type { Presence, Profile } from '../../api/types.ts';
import { store, toast, useStore } from '../../state/store.ts';
import { ConfirmDialog, Window, type ConfirmRequest } from '../../ui/Window.tsx';
import { explain } from '../../ui/format.ts';
import { CharacterPreview } from '../character/CharacterPreview.tsx';
import { refreshSocialCounters } from './socialApi.ts';

// UI-05: profile of another player with the social actions of TH-07-003.
export function ProfileWindow({ id, onClose }: { id: string; onClose: () => void }) {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [presence, setPresence] = useState<Presence | null>(null);
  const [myRoom, setMyRoom] = useState<string>('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null);
  const friendsVersion = useStore((s) => s.friendsVersion);

  const load = useCallback(async () => {
    try {
      const env = await get<{ profile: Profile; presence?: Presence; my_room_id?: string }>(`/api/v1/players/${id}/profile`);
      setProfile(env.result.profile);
      setPresence(env.result.presence ?? null);
      setMyRoom(env.result.my_room_id ?? '');
    } catch (e) {
      setError(explain(e));
    }
  }, [id]);
  useEffect(() => { void load(); }, [load, friendsVersion]);

  async function run(fn: () => Promise<unknown>, ok?: string) {
    setBusy(true);
    try {
      await fn();
      if (ok) toast(ok, 'success');
      store.set((s) => ({ friendsVersion: s.friendsVersion + 1 }));
      void refreshSocialCounters();
    } catch (e) {
      if (e instanceof ApiError && e.code === 'FRIEND_LIMIT_REACHED') {
        toast(e.details.mine ? 'Danh sách bạn của bạn đã đủ 100 người.' : 'Danh sách bạn của người này đã đầy.', 'error');
      } else toast(explain(e), 'error');
    } finally {
      setBusy(false);
      void load();
    }
  }

  if (error) return <Window title="Hồ sơ" onClose={onClose}><p className="form-error">{error}</p></Window>;
  if (!profile) return <Window title="Hồ sơ" onClose={onClose}><p className="muted">Đang tải…</p></Window>;
  const p = profile;

  return (
    <Window title={`Hồ sơ · ${p.display_name}`} onClose={onClose}>
      <div className="profile">
        <CharacterPreview seed={p.id} name={p.display_name} />
        <div className="profile-info">
          <div className="muted small">ID: <code>{p.id}</code></div>
          {p.is_friend && <div className="tag green">Bạn bè</div>}
          {p.is_friend && presence && <div className="small">{presence.online ? `🟢 Đang ở ${presence.room_id}` : '⚪ Không trực tuyến'}</div>}
          {p.blocked_by_me && <div className="tag red">Bạn đã chặn chat riêng với người này</div>}
          <div className="profile-actions">
            {!p.is_friend && p.request === '' && (
              <button className="btn primary" disabled={busy} onClick={() => void run(async () => {
                const env = await post<{ state: string }>('/api/v1/friends/requests', { character_id: p.id });
                toast(env.result.state === 'accepted' ? `Đã là bạn với ${p.display_name}!` : 'Đã gửi lời mời kết bạn.', 'success');
              })}>➕ Kết bạn</button>
            )}
            {p.request === 'outgoing' && (
              <button className="btn" disabled={busy} onClick={() => void run(() => post(`/api/v1/friends/requests/${p.request_id}/cancel`), 'Đã hủy lời mời.')}>Hủy lời mời</button>
            )}
            {p.request === 'incoming' && (
              <>
                <button className="btn primary" disabled={busy} onClick={() => void run(() => post(`/api/v1/friends/requests/${p.request_id}/accept`), `Đã kết bạn với ${p.display_name}.`)}>Chấp nhận kết bạn</button>
                <button className="btn" disabled={busy} onClick={() => void run(() => post(`/api/v1/friends/requests/${p.request_id}/reject`), 'Đã từ chối.')}>Từ chối</button>
              </>
            )}
            <button className="btn" onClick={() => store.set({ panel: { pm: p.id } })}>💬 Chat riêng</button>
            {p.is_friend && presence?.online && presence.room_id !== myRoom && (
              <button className="btn" disabled={busy} onClick={() => void run(async () => {
                await post(`/api/v1/friends/${p.id}/join-room`);
                toast(`Đã đến phòng ${presence.room_id}.`, 'success');
                onClose();
              })}>🚪 Đến cùng phòng</button>
            )}
            {p.is_friend && (
              <button className="btn ghost" disabled={busy} onClick={() => setConfirm({
                title: 'Hủy kết bạn?', body: <>Bỏ <b>{p.display_name}</b> khỏi danh sách bạn bè?</>, confirmLabel: 'Hủy kết bạn', danger: true,
                onConfirm: () => void run(() => del(`/api/v1/friends/${p.id}`), 'Đã hủy kết bạn.'),
              })}>Hủy kết bạn</button>
            )}
            {p.blocked_by_me ? (
              <button className="btn ghost" disabled={busy} onClick={() => void run(() => del(`/api/v1/private-chat/blocks/${p.id}`), 'Đã bỏ chặn.')}>Bỏ chặn chat riêng</button>
            ) : (
              <button className="btn danger ghost" disabled={busy} onClick={() => setConfirm({
                title: 'Chặn chat riêng?',
                body: <>Bạn sẽ không nhận tin nhắn riêng từ <b>{p.display_name}</b>. Người này không được thông báo.</>,
                confirmLabel: 'Chặn', danger: true,
                onConfirm: () => void run(() => post(`/api/v1/private-chat/blocks/${p.id}`), 'Đã chặn chat riêng.'),
              })}>Chặn chat riêng</button>
            )}
          </div>
        </div>
      </div>
      {confirm && <ConfirmDialog req={confirm} onClose={() => setConfirm(null)} />}
    </Window>
  );
}
