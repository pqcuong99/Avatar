import { useEffect, useMemo, useRef, useState } from 'react';
import Phaser from 'phaser';
import plazaMap from '@shared/maps/plaza.json';
import tilesUrl from '@shared/maps/tiles_placeholder.png';
import { ApiError, get, post } from '../api/http.ts';
import type { Me, OffersView, Player } from '../api/types.ts';
import { store, toast, useStore, type ChatLine } from '../state/store.ts';
import { loadCatalog, refreshState } from '../features/actions.ts';
import { Hud } from '../features/hud/Hud.tsx';
import { Joystick } from '../features/hud/Joystick.tsx';
import { ChatBox } from '../features/chat/ChatBox.tsx';
import { InventoryPanel } from '../features/inventory/InventoryPanel.tsx';
import { FarmPanel } from '../features/farm/FarmPanel.tsx';
import { PendingPanel } from '../features/pending/PendingPanel.tsx';
import { NpcWindow } from '../features/npc/NpcWindow.tsx';
import { SocialPanel } from '../features/social/SocialPanel.tsx';
import { ProfileWindow } from '../features/social/ProfileWindow.tsx';
import { PrivateChatPanel } from '../features/social/PrivateChatPanel.tsx';
import { refreshSocialCounters } from '../features/social/socialApi.ts';
import { WardrobePanel } from '../features/fashion/WardrobePanel.tsx';
import { QuestsPanel } from '../features/quests/QuestsPanel.tsx';
import { refreshQuestBadge } from '../features/quests/questApi.ts';
import { SettingsPanel } from '../features/settings/SettingsPanel.tsx';
import { Toasts } from '../ui/Window.tsx';
import { loadMap, type TiledMap } from './sim/movement.ts';
import { WorldSocket, type Hello } from './net/socket.ts';
import { WorldScene } from './WorldScene.ts';

const tiled = plazaMap as unknown as TiledMap;
const map = loadMap(tiled);

export function GameView({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const host = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<WorldScene | null>(null);
  const socket = useMemo(() => new WorldSocket(), []);
  const panel = useStore((s) => s.panel);
  const [chatMax, setChatMax] = useState(200);
  const touch = useMemo(() => window.matchMedia('(pointer: coarse)').matches, []);
  const character = me.character!;

  useEffect(() => {
    store.set({ chat: [], panel: null, mapOutdated: false, conn: 'connecting', me: { id: character.id, name: character.display_name } });
    void loadCatalog().catch(() => {});
    void refreshState().catch(() => {});
    void refreshSocialCounters();
    void refreshQuestBadge();
    socket.onStatus = (s) => store.set({ conn: s });
    socket.onRepeatedFailure = () => {
      get<Me>('/api/v1/auth/me').catch((e) => { if (e instanceof ApiError && e.status === 401) onLogout(); });
    };
    const unsub = [
      socket.on<Hello>('hello', (h) => {
        setChatMax(h.chat_max_chars);
        if (h.maintenance) store.set({ maintenance: h.maintenance });
        void refreshState().catch(() => {});
      }),
      socket.on<{ active: boolean; message: string }>('maintenance.notice', (m) => {
        store.set({ maintenance: m });
        toast(m.active ? `🔧 Bắt đầu bảo trì${m.message ? `: ${m.message}` : ''}` : 'Đã kết thúc bảo trì.', 'info', 8000);
        if (!m.active) { void loadCatalog().catch(() => {}); void refreshState().catch(() => {}); }
      }),
      socket.on<{ message_id: string; body: string; sender: { display_name: string; character_id: string } }>('chat.public.delivered', (p, env) => {
        store.set((s) => {
          const mine = s.chat.find((l) => l.requestId === env.request_id);
          if (mine) return { chat: s.chat.map((l) => (l === mine ? { ...l, status: 'sent' as const } : l)) };
          const line: ChatLine = { key: p.message_id, author: p.sender.display_name, body: p.body, at: Date.now(), mine: p.sender.character_id === character.id, status: 'sent' };
          return { chat: [...s.chat.slice(-99), line] };
        });
      }),
      socket.on<{ code: string; message: string }>('chat.public.failed', (p, env) => {
        store.set((s) => ({ chat: s.chat.map((l) => (l.requestId === env.request_id ? { ...l, status: 'failed' as const, error: p.message } : l)) }));
      }),
      socket.on<{ from: Player }>('friend.request', (p) => {
        toast(`${p.from.display_name} muốn kết bạn với bạn.`, 'info', 6000);
        void refreshSocialCounters();
        store.set((s) => ({ friendsVersion: s.friendsVersion + 1 }));
      }),
      socket.on<{ friend: Player }>('friend.accepted', (p) => {
        toast(`Bạn và ${p.friend.display_name} đã là bạn bè!`, 'success', 6000);
        store.set((s) => ({ friendsVersion: s.friendsVersion + 1 }));
      }),
      socket.on<{ sender: Player }>('chat.private.message', (p) => {
        const panel = store.get().panel;
        const open = panel && typeof panel === 'object' && 'pm' in panel && panel.pm === p.sender.id;
        if (!open) toast(`💌 Tin nhắn mới từ ${p.sender.display_name}`, 'info', 5000);
        store.set((s) => ({ pmVersion: s.pmVersion + 1, typing: { ...s.typing, [p.sender.id]: 0 } }));
        void refreshSocialCounters();
      }),
      socket.on('chat.private.seen', () => store.set((s) => ({ pmVersion: s.pmVersion + 1 }))),
      socket.on<{ from: string }>('chat.private.typing', (p) => {
        store.set((s) => ({ typing: { ...s.typing, [p.from]: Date.now() + 3000 } }));
      }),
    ];

    const scene = new WorldScene();
    sceneRef.current = scene;
    if (import.meta.env.DEV) (window as unknown as { __avatarScene: WorldScene }).__avatarScene = scene;
    scene.setSelf(character.id, character.display_name);
    let game: Phaser.Game | null = null;
    // WebGL cannot start on a 0x0 canvas (a hidden tab or pane at load time):
    // wait until the container has a real size.
    const start = () => {
      const el = host.current;
      if (game || !el || el.clientWidth === 0 || el.clientHeight === 0) return;
      game = new Phaser.Game({
        type: Phaser.AUTO,
        parent: el,
        backgroundColor: '#2f6e2c',
        scale: { mode: Phaser.Scale.RESIZE, width: el.clientWidth, height: el.clientHeight },
        render: { antialias: true, roundPixels: true },
        input: { activePointers: 3 },
        banner: false,
      });
      game.scene.add('world', scene, true, {
        socket, map, tiled, tilesUrl,
        npcNames: {},
        hooks: {
          npcReached: (code: string) => store.set({ panel: { npc: code } }),
          playerClicked: (id: string) => store.set({ panel: { profile: id } }),
          mapOutdated: () => store.set({ mapOutdated: true }),
          // Connect only once the scene listens, or hello/room.snapshot are missed.
          ready: () => socket.connect(),
        },
      });
    };
    const sizeWatch = new ResizeObserver(start);
    sizeWatch.observe(host.current!);
    start();

    // NPC names live in the database (Admin-configured); the map only has codes.
    for (const npc of map.npcs) {
      get<OffersView>(`/api/v1/npc/${npc.code}/offers`).then((env) => {
        sceneRef.current?.setNpcNames({ [npc.code]: env.result.display_name });
      }).catch(() => {});
    }

    const refresh = setInterval(() => void refreshState().catch(() => {}), 60000);
    const onKey = (e: KeyboardEvent) => {
      const el = document.activeElement;
      if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT')) return;
      const k = e.key.toLowerCase();
      if (k === 'i') store.set({ panel: store.get().panel === 'inventory' ? null : 'inventory' });
      if (k === 'f') store.set({ panel: store.get().panel === 'farm' ? null : 'farm' });
      if (k === 'p') store.set({ panel: store.get().panel === 'pending' ? null : 'pending' });
      if (k === 'b') store.set({ panel: store.get().panel === 'social' ? null : 'social' });
      if (k === 't') store.set({ panel: store.get().panel === 'wardrobe' ? null : 'wardrobe' });
      if (k === 'q') store.set({ panel: store.get().panel === 'quests' ? null : 'quests' });
      if (k === 'm') store.set({ panel: { pm: null } });
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
      clearInterval(refresh);
      unsub.forEach((u) => u());
      sizeWatch.disconnect();
      socket.close();
      game?.destroy(true);
      sceneRef.current = null;
    };
  }, [socket]); // eslint-disable-line react-hooks/exhaustive-deps

  async function logout() {
    try {
      await post('/api/v1/auth/logout');
    } catch {
      // the session ends on the server anyway when it expires
    }
    socket.close();
    onLogout();
  }

  const close = () => store.set({ panel: null });
  return (
    <div className="game">
      <div className="game-canvas" ref={host} aria-label={`Bản đồ ${map.displayName}`} />
      <div className="hud">
        <Hud name={character.display_name} onReload={() => location.reload()} />
        <ChatBox socket={socket} maxChars={chatMax} compact={touch} />
        {touch && <Joystick onChange={(v) => sceneRef.current?.setJoystick(v)} />}
        {!touch && <div className="help-hint">WASD/mũi tên để đi · nhấp chuột để tự tìm đường · nhấp NPC để giao dịch · I/F/P: Túi/Nông trại/Hàng chờ</div>}
      </div>
      {panel === 'inventory' && <InventoryPanel onClose={close} />}
      {panel === 'farm' && <FarmPanel onClose={close} />}
      {panel === 'pending' && <PendingPanel onClose={close} />}
      {panel === 'social' && <SocialPanel onClose={close} />}
      {panel === 'wardrobe' && <WardrobePanel onClose={close} />}
      {panel === 'quests' && <QuestsPanel onClose={close} />}
      {panel === 'settings' && <SettingsPanel onClose={close} onLogout={() => void logout()} />}
      {panel && typeof panel === 'object' && 'npc' in panel && <NpcWindow code={panel.npc} onClose={close} />}
      {panel && typeof panel === 'object' && 'profile' in panel && <ProfileWindow id={panel.profile} onClose={close} />}
      {panel && typeof panel === 'object' && 'pm' in panel && <PrivateChatPanel partnerId={panel.pm} socket={socket} onClose={close} />}
      <Toasts />
    </div>
  );
}
