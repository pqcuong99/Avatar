import { useCallback, useEffect, useLayoutEffect, useRef, useState, type FormEvent } from 'react';
import { ApiError, command, get, newRequestId, post } from '../../api/http.ts';
import type { Conversation, PrivateMessage } from '../../api/types.ts';
import { store, useStore } from '../../state/store.ts';
import { Window } from '../../ui/Window.tsx';
import { explain } from '../../ui/format.ts';
import type { WorldSocket } from '../../game/net/socket.ts';
import { refreshSocialCounters } from './socialApi.ts';

// Drafts live for the session only (TH-05-005).
const drafts = new Map<string, string>();

interface Line extends PrivateMessage {
  status?: 'pending' | 'failed';
  requestId?: string;
  error?: string;
}

/** Private chat (UI-07): one conversation at a time (TH-05-006). */
export function PrivateChatPanel({ partnerId, socket, onClose }: { partnerId: string | null; socket: WorldSocket; onClose: () => void }) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [active, setActive] = useState<string | null>(partnerId);
  const pmVersion = useStore((s) => s.pmVersion);
  const loadConversations = useCallback(() => {
    get<{ conversations: Conversation[] }>('/api/v1/private-chat/conversations').then((e) => setConversations(e.result.conversations)).catch(() => {});
  }, []);
  useEffect(() => { loadConversations(); }, [loadConversations, pmVersion]);
  useEffect(() => { if (partnerId) setActive(partnerId); }, [partnerId]);
  const activeName = conversations.find((c) => c.partner.id === active)?.partner.display_name;

  return (
    <Window title="Tin nhắn riêng" onClose={onClose} wide className="pm-window">
      <div className="pm-layout">
        <ul className="pm-list" aria-label="Cuộc trò chuyện">
          {conversations.length === 0 && <li className="muted small">Chưa có cuộc trò chuyện. Mở hồ sơ một người chơi và chọn “Chat riêng”.</li>}
          {conversations.map((c) => (
            <li key={c.partner.id}>
              <button className={c.partner.id === active ? 'active' : ''} onClick={() => setActive(c.partner.id)}>
                <b>{c.partner.display_name}</b>
                <span className="snippet">{c.last.body}</span>
                {c.unread > 0 && <i className="badge static">{c.unread}</i>}
              </button>
            </li>
          ))}
        </ul>
        {active ? (
          <Thread key={active} partnerId={active} partnerName={activeName} socket={socket} onChanged={loadConversations} />
        ) : (
          <div className="pm-empty muted">Chọn một cuộc trò chuyện.</div>
        )}
      </div>
    </Window>
  );
}

function Thread({ partnerId, partnerName, socket, onChanged }: { partnerId: string; partnerName?: string; socket: WorldSocket; onChanged: () => void }) {
  const me = useStore((s) => s.me);
  const typingUntil = useStore((s) => s.typing[partnerId] ?? 0);
  const pmVersion = useStore((s) => s.pmVersion);
  const [lines, setLines] = useState<Line[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState(partnerName ?? '');
  const [text, setText] = useState(drafts.get(partnerId) ?? '');
  const [, tick] = useState(0);
  const list = useRef<HTMLDivElement>(null);
  const keepScroll = useRef<{ height: number; top: number } | null>(null);
  const stickBottom = useRef(true);
  const seenQueue = useRef(new Set<string>());
  const seenTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // First page: the newest 50 messages.
  useEffect(() => {
    let alive = true;
    setLoading(true);
    get<{ messages: PrivateMessage[]; next_cursor: string | null }>(`/api/v1/private-chat/with/${partnerId}/messages`).then((e) => {
      if (!alive) return;
      setLines(e.result.messages);
      setCursor(e.result.next_cursor);
      stickBottom.current = true;
    }).catch(() => {}).finally(() => alive && setLoading(false));
    if (!partnerName) get<{ profile: { display_name: string } }>(`/api/v1/players/${partnerId}/profile`).then((e) => alive && setName(e.result.profile.display_name)).catch(() => {});
    return () => { alive = false; };
  }, [partnerId]); // eslint-disable-line react-hooks/exhaustive-deps

  // New messages and seen receipts arrive over the socket (pmVersion bump):
  // re-read the newest page and merge it.
  useEffect(() => {
    if (pmVersion === 0) return;
    get<{ messages: PrivateMessage[] }>(`/api/v1/private-chat/with/${partnerId}/messages`).then((e) => {
      setLines((prev) => {
        const byId = new Map(prev.map((l) => [l.id, l]));
        for (const m of e.result.messages) byId.set(m.id, { ...byId.get(m.id), ...m, status: undefined });
        return [...byId.values()].sort((a, b) => a.sent_at.localeCompare(b.sent_at));
      });
    }).catch(() => {});
  }, [pmVersion, partnerId]);

  // Keep the reading position when older messages are prepended (TH-05-002),
  // otherwise follow the newest message when already at the bottom.
  useLayoutEffect(() => {
    const el = list.current;
    if (!el) return;
    if (keepScroll.current) {
      el.scrollTop = el.scrollHeight - keepScroll.current.height + keepScroll.current.top;
      keepScroll.current = null;
    } else if (stickBottom.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [lines]);

  const loadOlder = useCallback(async () => {
    if (!cursor || loading) return;
    const el = list.current!;
    setLoading(true);
    try {
      const e = await get<{ messages: PrivateMessage[]; next_cursor: string | null }>(`/api/v1/private-chat/with/${partnerId}/messages?before=${encodeURIComponent(cursor)}`);
      keepScroll.current = { height: el.scrollHeight, top: el.scrollTop };
      setLines((prev) => [...e.result.messages, ...prev]);
      setCursor(e.result.next_cursor);
    } finally {
      setLoading(false);
    }
  }, [cursor, loading, partnerId]);

  function onScroll() {
    const el = list.current!;
    stickBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    if (el.scrollTop < 60) void loadOlder();
  }

  // Seen = the message really was on screen (TH-05-003): observe unseen
  // incoming messages and report the visible ones in small batches.
  useEffect(() => {
    const root = list.current;
    if (!root || !me) return;
    const flush = () => {
      const ids = [...seenQueue.current];
      seenQueue.current.clear();
      if (ids.length === 0) return;
      post('/api/v1/private-chat/seen', { message_ids: ids }).then(() => {
        setLines((prev) => prev.map((l) => (ids.includes(l.id) ? { ...l, seen_at: new Date().toISOString() } : l)));
        void refreshSocialCounters();
        onChanged();
      }).catch(() => {});
    };
    const io = new IntersectionObserver((entries) => {
      for (const en of entries) {
        if (en.isIntersecting && document.visibilityState === 'visible') {
          seenQueue.current.add((en.target as HTMLElement).dataset.id!);
          io.unobserve(en.target);
        }
      }
      if (seenQueue.current.size && !seenTimer.current) {
        seenTimer.current = setTimeout(() => { seenTimer.current = null; flush(); }, 400);
      }
    }, { root, threshold: 0.6 });
    root.querySelectorAll<HTMLElement>('[data-unseen="1"]').forEach((el) => io.observe(el));
    return () => io.disconnect();
  }, [lines, me, onChanged]);

  // "đang nhập…" disappears on its own.
  useEffect(() => {
    if (typingUntil <= Date.now()) return;
    const t = setTimeout(() => tick((n) => n + 1), typingUntil - Date.now() + 50);
    return () => clearTimeout(t);
  }, [typingUntil]);

  async function send(body: string, requestId = newRequestId()) {
    const pending: Line = {
      id: 'local-' + requestId, requestId, sender_id: me!.id, recipient_id: partnerId, body,
      sent_at: new Date(Date.now() + store.get().serverOffsetMs).toISOString(), expires_at: '', seen_at: null, status: 'pending',
    };
    stickBottom.current = true;
    setLines((prev) => [...prev.filter((l) => l.requestId !== requestId), pending]);
    try {
      const e = await command<{ message: PrivateMessage }>(`/api/v1/private-chat/with/${partnerId}/messages`, { body }, requestId);
      setLines((prev) => {
        const rest = prev.filter((l) => l.requestId !== requestId && l.id !== e.result.message.id);
        return [...rest, e.result.message].sort((a, b) => a.sent_at.localeCompare(b.sent_at));
      });
      onChanged();
    } catch (err) {
      const msg = err instanceof ApiError && err.code === 'FORBIDDEN' ? 'Không gửi được tin nhắn.' : explain(err);
      setLines((prev) => prev.map((l) => (l.requestId === requestId ? { ...l, status: 'failed', error: msg } : l)));
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    const body = text.trim();
    if (!body) return;
    setText('');
    drafts.delete(partnerId);
    void send(body);
  }

  const lastSeenMine = [...lines].reverse().find((l) => l.sender_id === me?.id && l.seen_at);
  return (
    <div className="pm-thread">
      <div className="pm-head">
        <button className="link" onClick={() => store.set({ panel: { profile: partnerId } })}><b>{name || '…'}</b></button>
        <span className="muted small">Tin nhắn tự xóa sau 30 ngày</span>
      </div>
      <div className="pm-messages" ref={list} onScroll={onScroll}>
        {cursor && <div className="muted small center">{loading ? 'Đang tải tin cũ…' : 'Cuộn lên để xem tin cũ hơn'}</div>}
        {!cursor && lines.length > 0 && <div className="muted small center">Đầu cuộc trò chuyện</div>}
        {lines.map((l) => {
          const mine = l.sender_id === me?.id;
          return (
            <div key={l.id} className={`pm-msg ${mine ? 'mine' : ''} ${l.status ?? ''}`} data-id={l.id} data-unseen={!mine && !l.seen_at && !l.status ? '1' : '0'}>
              <div className="bubble">{l.body}</div>
              <div className="meta">
                {new Date(l.sent_at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}
                {l.status === 'pending' && ' · đang gửi'}
                {l.status === 'failed' && <> · {l.error} <button className="link" onClick={() => void send(l.body, l.requestId)}>Gửi lại</button></>}
                {lastSeenMine && l.id === lastSeenMine.id && ' · Đã xem'}
              </div>
            </div>
          );
        })}
      </div>
      {typingUntil > Date.now() && <div className="pm-typing muted small">{name} đang nhập…</div>}
      <form className="chat-form light" onSubmit={submit}>
        <input
          value={text}
          maxLength={500}
          placeholder="Nhập tin nhắn riêng…"
          onChange={(e) => {
            setText(e.target.value);
            drafts.set(partnerId, e.target.value);
            if (e.target.value) socket.send('chat.private.typing', { to: partnerId });
          }}
          aria-label="Tin nhắn riêng"
        />
        <button className="btn primary small" disabled={!text.trim()}>Gửi</button>
      </form>
    </div>
  );
}
