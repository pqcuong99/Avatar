// Tiny observable store shared by React (useStore) and the Phaser scene.
import { useSyncExternalStore } from 'react';
import type { Catalog, GameState } from '../api/types.ts';

export interface Toast {
  id: number;
  kind: 'info' | 'success' | 'error';
  text: string;
}

export interface ChatLine {
  key: string;
  requestId?: string;
  author: string;
  body: string;
  at: number;
  mine: boolean;
  status: 'pending' | 'sent' | 'failed';
  error?: string;
}

export type ConnStatus = 'connecting' | 'online' | 'reconnecting' | 'replaced' | 'not_configured' | 'offline';

export type Panel = null | 'inventory' | 'farm' | 'pending' | 'social' | 'wardrobe' | 'quests' | 'settings' | { npc: string } | { profile: string } | { pm: string | null };

export interface AppState {
  catalog: Catalog | null;
  game: GameState | null;
  /** My character id, for social views. */
  me: { id: string; name: string } | null;
  incomingRequests: number;
  unreadPM: number;
  /** Partner id -> time until which "đang nhập…" shows. */
  typing: Record<string, number>;
  /** Bumped when a private message or seen receipt arrives, to refresh views. */
  pmVersion: number;
  friendsVersion: number;
  claimableQuests: number;
  maintenance: { active: boolean; message: string };
  /** serverTime - Date.now(), from the latest server timestamp. */
  serverOffsetMs: number;
  toasts: Toast[];
  chat: ChatLine[];
  conn: ConnStatus;
  panel: Panel;
  mapOutdated: boolean;
}

type Listener = () => void;

function createStore<T>(initial: T) {
  let state = initial;
  const listeners = new Set<Listener>();
  return {
    get: () => state,
    set(patch: Partial<T> | ((s: T) => Partial<T>)) {
      const p = typeof patch === 'function' ? patch(state) : patch;
      state = { ...state, ...p };
      listeners.forEach((l) => l());
    },
    subscribe(l: Listener) {
      listeners.add(l);
      return () => listeners.delete(l);
    },
  };
}

export const store = createStore<AppState>({
  catalog: null,
  game: null,
  me: null,
  incomingRequests: 0,
  unreadPM: 0,
  typing: {},
  pmVersion: 0,
  friendsVersion: 0,
  claimableQuests: 0,
  maintenance: { active: false, message: '' },
  serverOffsetMs: 0,
  toasts: [],
  chat: [],
  conn: 'connecting',
  panel: null,
  mapOutdated: false,
});

export function useStore<S>(selector: (s: AppState) => S): S {
  return useSyncExternalStore(store.subscribe, () => selector(store.get()));
}

export const serverNow = () => Date.now() + store.get().serverOffsetMs;

/** Applies a state snapshot that came with a command or a refresh. */
export function applyGameState(state: unknown) {
  if (!state) return;
  const gs = state as GameState;
  store.set({ game: gs, serverOffsetMs: new Date(gs.server_time).getTime() - Date.now() });
}

let toastSeq = 0;
export function toast(text: string, kind: Toast['kind'] = 'info', ms = 3500) {
  const id = ++toastSeq;
  store.set((s) => ({ toasts: [...s.toasts.slice(-3), { id, kind, text }] }));
  setTimeout(() => store.set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })), ms);
}

export function itemName(code: string): string {
  return store.get().catalog?.items?.find((i) => i.item_code === code)?.display_name ?? code;
}
