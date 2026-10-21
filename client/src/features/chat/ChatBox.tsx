import { useEffect, useRef, useState, type FormEvent } from 'react';
import { newRequestId } from '../../api/http.ts';
import { store, useStore, type ChatLine } from '../../state/store.ts';
import type { WorldSocket } from '../../game/net/socket.ts';

// UI-06: public chat of the current room. A message shows as "sending" until
// the server confirms it; a failed one can be retried with the same
// request id, so it is never posted twice (S-10).
export function ChatBox({ socket, maxChars, compact }: { socket: WorldSocket; maxChars: number; compact: boolean }) {
  const lines = useStore((s) => s.chat);
  const conn = useStore((s) => s.conn);
  const [text, setText] = useState('');
  const [open, setOpen] = useState(!compact);
  const list = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const el = list.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [lines.length, open]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Enter' && document.activeElement?.tagName !== 'INPUT' && !store.get().panel) {
        setOpen(true);
        setTimeout(() => input.current?.focus(), 0);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  function send(body: string, requestId = newRequestId()) {
    const ok = socket.send('chat.public.send', { body }, requestId);
    store.set((s) => {
      const existing = s.chat.find((l) => l.requestId === requestId);
      const line: ChatLine = { key: requestId, requestId, author: 'Bạn', body, at: Date.now(), mine: true, status: ok ? 'pending' : 'failed', error: ok ? undefined : 'Chưa kết nối' };
      return { chat: existing ? s.chat.map((l) => (l.requestId === requestId ? line : l)) : [...s.chat.slice(-99), line] };
    });
    if (ok) {
      // No answer at all (connection dropped): let the player retry.
      setTimeout(() => store.set((s) => ({
        chat: s.chat.map((l) => (l.requestId === requestId && l.status === 'pending' ? { ...l, status: 'failed', error: 'Không có phản hồi' } : l)),
      })), 8000);
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    const body = text.trim();
    if (!body) return;
    send(body);
    setText('');
  }

  if (compact && !open) {
    return <button className="chat-toggle" onClick={() => setOpen(true)}>💬</button>;
  }
  return (
    <section className={`chat ${compact ? 'compact' : ''}`} aria-label="Trò chuyện trong phòng">
      <div className="chat-head">
        <span>Trò chuyện · phòng hiện tại</span>
        <button className="icon-btn small" onClick={() => setOpen(false)} aria-label="Thu gọn">—</button>
      </div>
      <div className="chat-list" ref={list}>
        {lines.length === 0 && <div className="chat-empty">Chào mọi người trong phòng nhé!</div>}
        {lines.map((l) => (
          <div key={l.key} className={`chat-line ${l.mine ? 'mine' : ''} ${l.status}`}>
            <b>{l.author}:</b> <span>{l.body}</span>
            {l.status === 'pending' && <em> · đang gửi</em>}
            {l.status === 'failed' && (
              <em> · {l.error ?? 'lỗi'} <button className="link" onClick={() => send(l.body, l.requestId)}>Gửi lại</button></em>
            )}
          </div>
        ))}
      </div>
      <form className="chat-form" onSubmit={submit}>
        <input
          ref={input}
          value={text}
          onChange={(e) => setText(e.target.value)}
          maxLength={maxChars}
          placeholder={conn === 'online' ? 'Nhập tin nhắn… (Enter)' : 'Đang chờ kết nối…'}
          onKeyDown={(e) => { if (e.key === 'Escape') (e.target as HTMLInputElement).blur(); }}
          aria-label="Tin nhắn"
        />
        <button className="btn primary small" disabled={!text.trim()}>Gửi</button>
      </form>
    </section>
  );
}
