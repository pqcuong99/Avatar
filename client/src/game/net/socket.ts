// WebSocket client (docs/08). JSON text frames for events, binary frames for
// movement. Reconnects on its own: inside the 30 s grace the server puts the
// character back where it was (P0-007/008), later from the saved position.
import { decodeSnapshot, encodeMove, type MoveInput, type Snapshot } from './proto.ts';
import type { ConnStatus } from '../../state/store.ts';
import type { Appearance } from '../art/characterArt.ts';

export interface Envelope<T = unknown> {
  v: number;
  type: string;
  request_id?: string;
  payload: T;
}

export interface Hello {
  protocol_version: number;
  server_time: string;
  map_id: string;
  map_version: string;
  room_id: string;
  entity_id: number;
  config_version: number;
  grace_seconds: number;
  tick_ms: number;
  snapshot_every: number;
  speed_per_step: number;
  resumed: boolean;
  chat_max_chars: number;
  maintenance?: { active: boolean; message: string };
}

export interface EntityInfo {
  entity_id: number;
  character_id: string;
  display_name: string;
  x: number;
  y: number;
  dir: number;
  appearance?: Appearance | null;
}

type Handler = (payload: never, env: Envelope) => void;

// Close codes from game-server/internal/world.
const CLOSE_LOGOUT = 4000;
const CLOSE_SESSION_REPLACED = 4001;
const CLOSE_NOT_CONFIGURED = 4002;

export class WorldSocket {
  private ws: WebSocket | null = null;
  private handlers = new Map<string, Set<Handler>>();
  private retry = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private stopped = false;
  status: ConnStatus = 'connecting';
  onStatus: (s: ConnStatus) => void = () => {};
  onSnapshot: (s: Snapshot, at: number) => void = () => {};
  /** Called after several failed reconnects, to check whether we are still logged in. */
  onRepeatedFailure: () => void = () => {};

  connect() {
    this.stopped = false;
    this.open();
  }

  private setStatus(s: ConnStatus) {
    this.status = s;
    this.onStatus(s);
  }

  private open() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/ws`);
    ws.binaryType = 'arraybuffer';
    this.ws = ws;
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        let env: Envelope;
        try {
          env = JSON.parse(ev.data);
        } catch {
          return;
        }
        if (env.type === 'hello') {
          this.retry = 0;
          this.setStatus('online');
        }
        this.handlers.get(env.type)?.forEach((h) => h(env.payload as never, env));
        return;
      }
      try {
        const snap = decodeSnapshot(new Uint8Array(ev.data as ArrayBuffer));
        if (snap) this.onSnapshot(snap, performance.now());
      } catch {
        // malformed frame: ignore
      }
    };
    ws.onclose = (ev) => {
      if (this.ws !== ws) return;
      this.ws = null;
      if (this.stopped || ev.code === CLOSE_LOGOUT) {
        this.setStatus('offline');
        return;
      }
      if (ev.code === CLOSE_SESSION_REPLACED) {
        this.setStatus('replaced');
        return;
      }
      if (ev.code === CLOSE_NOT_CONFIGURED) {
        this.setStatus('not_configured');
        this.schedule(10_000);
        return;
      }
      this.setStatus('reconnecting');
      this.retry++;
      if (this.retry === 4) this.onRepeatedFailure();
      this.schedule(Math.min(5000, 400 * 2 ** Math.min(this.retry, 4)));
    };
  }

  private schedule(ms: number) {
    if (this.timer) clearTimeout(this.timer);
    this.timer = setTimeout(() => { this.timer = null; if (!this.stopped) this.open(); }, ms);
  }

  on<T>(type: string, h: (payload: T, env: Envelope) => void): () => void {
    let set = this.handlers.get(type);
    if (!set) this.handlers.set(type, (set = new Set()));
    set.add(h as Handler);
    return () => set!.delete(h as Handler);
  }

  send(type: string, payload: unknown, requestId?: string): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false;
    this.ws.send(JSON.stringify({ v: 1, type, request_id: requestId, payload }));
    return true;
  }

  sendMove(m: MoveInput): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false;
    this.ws.send(encodeMove(m));
    return true;
  }

  close() {
    this.stopped = true;
    if (this.timer) clearTimeout(this.timer);
    this.ws?.close(1000, 'bye');
    this.ws = null;
  }
}
