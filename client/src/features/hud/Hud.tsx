import { store, useStore, type Panel } from '../../state/store.ts';
import { CurrencyIcon } from '../../ui/ItemIcon.tsx';
import { num } from '../../ui/format.ts';

const CONN_TEXT: Record<string, string | null> = {
  connecting: 'Đang kết nối máy chủ…',
  online: null,
  reconnecting: 'Mất kết nối — đang thử nối lại…',
  replaced: 'Tài khoản vừa đăng nhập ở nơi khác. Phiên này đã dừng.',
  not_configured: 'Bản đồ chưa sẵn sàng (Admin chưa cấu hình sức chứa phòng). Đang chờ…',
  offline: 'Đã ngắt kết nối.',
};

export function Hud({ name, onReload }: { name: string; onReload: () => void }) {
  const game = useStore((s) => s.game);
  const myName = useStore((s) => s.me?.name) ?? name;
  const conn = useStore((s) => s.conn);
  const mapOutdated = useStore((s) => s.mapOutdated);
  const maintenance = useStore((s) => s.maintenance);
  const pendingCount = useStore((s) => (s.game?.pending_crops?.length ?? 0) + (s.game?.pending_diamonds?.length ?? 0));
  const requests = useStore((s) => s.incomingRequests);
  const unread = useStore((s) => s.unreadPM);
  const quests = useStore((s) => s.claimableQuests);
  const readyPlots = useStore((s) => s.game?.farm.plots?.filter((p) => p.planting?.mature || p.planting?.pest?.active).length ?? 0);
  const open = (p: Panel) => store.set({ panel: p });
  const banner = mapOutdated ? 'Bản đồ đã được cập nhật.'
    : maintenance.active && conn === 'online' ? `🔧 Bảo trì${maintenance.message ? `: ${maintenance.message}` : ''} — giao dịch tạm dừng.`
      : CONN_TEXT[conn];

  return (
    <>
      <div className="hud-top-left">
        <div className="player-card">
          <div className="player-name">{myName}</div>
          <div className="wallets">
            <span title="Xu"><CurrencyIcon currency="XU" /> {num(game?.wallets.XU.balance ?? 0)}</span>
            <span title="Kim Cương"><CurrencyIcon currency="DIAMOND" /> {num(game?.wallets.DIAMOND.balance ?? 0)}</span>
          </div>
        </div>
      </div>
      <nav className="hud-top-right" aria-label="Chức năng">
        <button className="hud-btn" onClick={() => open('inventory')} title="Túi đồ (I)"><span>🎒</span><small>Túi</small></button>
        <button className="hud-btn" onClick={() => open('farm')} title="Nông trại (F)">
          <span>🌱</span><small>Nông trại</small>{readyPlots > 0 && <b className="badge">{readyPlots}</b>}
        </button>
        <button className="hud-btn" onClick={() => open('pending')} title="Hàng chờ nhận (P)">
          <span>📦</span><small>Hàng chờ</small>{pendingCount > 0 && <b className="badge">{pendingCount}</b>}
        </button>
        <button className="hud-btn" onClick={() => open('wardrobe')} title="Tủ đồ (T)"><span>👕</span><small>Tủ đồ</small></button>
        <button className="hud-btn" onClick={() => open('quests')} title="Nhiệm vụ ngày (Q)">
          <span>📜</span><small>Nhiệm vụ</small>{quests > 0 && <b className="badge">{quests}</b>}
        </button>
        <button className="hud-btn" onClick={() => open('social')} title="Bạn bè (B)">
          <span>👥</span><small>Bạn bè</small>{requests > 0 && <b className="badge">{requests}</b>}
        </button>
        <button className="hud-btn" onClick={() => open({ pm: null })} title="Tin nhắn riêng (M)">
          <span>💌</span><small>Tin nhắn</small>{unread > 0 && <b className="badge">{unread}</b>}
        </button>
        <button className="hud-btn" onClick={() => open('settings')} title="Cài đặt"><span>⚙️</span><small>Cài đặt</small></button>
      </nav>
      {banner && (
        <div className={`conn-banner ${conn === 'replaced' ? 'bad' : ''}`} role="status">
          {banner}
          {(mapOutdated || conn === 'replaced') && <button className="btn small" onClick={onReload}>Tải lại</button>}
        </div>
      )}
    </>
  );
}
