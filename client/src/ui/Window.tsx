import { useEffect, type ReactNode } from 'react';
import { store, useStore } from '../state/store.ts';

export function Window({ title, onClose, children, wide, className }: {
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
  wide?: boolean;
  className?: string;
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  return (
    <div className="window-backdrop" onPointerDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <section className={`window ${wide ? 'wide' : ''} ${className ?? ''}`} role="dialog" aria-modal="true" aria-label={typeof title === 'string' ? title : undefined}>
        <header className="window-head">
          <h2>{title}</h2>
          <button className="icon-btn" onClick={onClose} aria-label="Đóng">✕</button>
        </header>
        <div className="window-body">{children}</div>
      </section>
    </div>
  );
}

export interface ConfirmRequest {
  title: string;
  body: ReactNode;
  confirmLabel: string;
  danger?: boolean;
  onConfirm: () => void;
}

export function ConfirmDialog({ req, onClose }: { req: ConfirmRequest; onClose: () => void }) {
  return (
    <div className="window-backdrop confirm" onPointerDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <section className="window confirm-window" role="alertdialog" aria-modal="true" aria-label={req.title}>
        <header className="window-head"><h2>{req.title}</h2></header>
        <div className="window-body">
          <div className="confirm-text">{req.body}</div>
          <div className="row end gap">
            <button className="btn ghost" onClick={onClose}>Hủy</button>
            <button className={`btn ${req.danger ? 'danger' : 'primary'}`} autoFocus onClick={() => { onClose(); req.onConfirm(); }}>
              {req.confirmLabel}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}

export function Toasts() {
  const toasts = useStore((s) => s.toasts);
  return (
    <div className="toasts" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={`toast ${t.kind}`} onClick={() => store.set((s) => ({ toasts: s.toasts.filter((x) => x.id !== t.id) }))}>
          {t.kind === 'success' ? '✔ ' : t.kind === 'error' ? '⚠ ' : ''}{t.text}
        </div>
      ))}
    </div>
  );
}
