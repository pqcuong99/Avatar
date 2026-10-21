import Phaser from 'phaser';
import {
  ART_H, ART_W, FEET_X, FEET_Y, NPC_PALETTES, appearanceKey, drawCharacter, facingOf, paletteFor, paletteWith,
  type Appearance, type Palette,
} from './art/characterArt.ts';
import { clampDir, collides, dir8, step, UNIT, type LoadedMap, type TiledMap } from './sim/movement.ts';
import { findPath, type Point } from './sim/pathfinding.ts';
import { PathFollower } from './sim/follow.ts';
import type { EntityInfo, Hello, WorldSocket } from './net/socket.ts';
import type { MoveInput, Snapshot } from './net/proto.ts';

export interface SceneHooks {
  /** The player reached (or clicked while near) an NPC. */
  npcReached(code: string): void;
  /** Another player was clicked or tapped: show its profile. */
  playerClicked(characterId: string): void;
  mapOutdated(): void;
  /** create() finished and the scene listens to the socket: connect now. */
  ready(): void;
}

export interface SceneDeps {
  socket: WorldSocket;
  map: LoadedMap;
  tiled: TiledMap;
  tilesUrl: string;
  hooks: SceneHooks;
  npcNames: Record<string, string>;
}

// Viet text needs a taller test string so tone marks on capitals are not cut
// (see the earlier demo fix): every label goes through viText.
const VI_TEXT: Phaser.Types.GameObjects.Text.TextStyle = { testString: '|ÉqgỵẢỒẾỐỸẨẪẲẴ', padding: { top: 2 } };
function viText(scene: Phaser.Scene, x: number, y: number, text: string, style: Phaser.Types.GameObjects.Text.TextStyle) {
  return scene.add.text(x, y, text, { ...VI_TEXT, fontFamily: 'Segoe UI, Roboto, Arial, sans-serif', ...style, padding: { ...VI_TEXT.padding, ...style.padding } });
}

const ART_SCALE = 2; // textures are drawn at 2x and shown at 0.5
const INTERP_DELAY_MS = 200;

/** A character on screen: shadow, body, name label and chat bubble. */
class Avatar {
  readonly root: Phaser.GameObjects.Container;
  private body: Phaser.GameObjects.Image;
  private label: Phaser.GameObjects.Text;
  private bubble: Phaser.GameObjects.Container | null = null;
  private bubbleTimer: Phaser.Time.TimerEvent | null = null;
  private seed: string;
  private base: Palette;
  private palette: Palette;
  private lookKey = 'base';
  dir = 0;
  moving = false;

  constructor(private scene: Phaser.Scene, seed: string, palette: Palette, name: string, kind: 'self' | 'other' | 'npc', look?: Appearance | null) {
    this.seed = seed;
    this.base = palette;
    this.palette = paletteWith(palette, look);
    this.lookKey = appearanceKey(look);
    const shadow = scene.add.ellipse(0, 0, 20, 6, 0x000000, 0.22);
    this.body = scene.add.image(0, 0, this.texture('front', -1)).setOrigin(FEET_X / ART_W, FEET_Y / ART_H).setScale(1 / ART_SCALE);
    const color = kind === 'self' ? '#ffe066' : kind === 'npc' ? '#ffd8a8' : '#ffffff';
    this.label = viText(scene, 0, -ART_H + 2, name, {
      fontSize: '12px', color, stroke: '#1b2433', strokeThickness: 3, fontStyle: kind === 'npc' ? 'bold' : 'normal',
    }).setOrigin(0.5, 1);
    this.root = scene.add.container(0, 0, [shadow, this.body, this.label]);
  }

  /** New outfit (character.appearance.changed); textures are cached per outfit. */
  setAppearance(look: Appearance | null) {
    this.palette = paletteWith(this.base, look);
    this.lookKey = appearanceKey(look);
  }

  private texture(facing: 'front' | 'back' | 'side', frame: number): string {
    const key = `av:${this.seed}:${this.lookKey}:${facing}:${frame}`;
    if (!this.scene.textures.exists(key)) {
      const tex = this.scene.textures.createCanvas(key, ART_W * ART_SCALE, ART_H * ART_SCALE)!;
      const ctx = tex.getContext();
      ctx.scale(ART_SCALE, ART_SCALE);
      drawCharacter(ctx, this.palette, facing, frame);
      tex.refresh();
    }
    return key;
  }

  setName(name: string) { this.label.setText(name); }

  /** Makes the figure clickable; the scene reads `character` from it. */
  clickable(characterId: string) {
    this.body.setInteractive({ useHandCursor: true });
    this.body.setData('character', characterId);
  }

  place(xPx: number, yPx: number) {
    this.root.setPosition(xPx, yPx);
    this.root.setDepth(yPx);
  }

  animate(time: number) {
    const { facing, flip } = facingOf(this.dir);
    const frame = this.moving ? Math.floor(time / 130) % 4 : -1;
    this.body.setTexture(this.texture(facing, frame));
    this.body.setFlipX(flip);
  }

  say(text: string) {
    this.bubble?.destroy();
    this.bubbleTimer?.remove();
    const t = viText(this.scene, 0, 0, text, { fontSize: '12px', color: '#1b2433', wordWrap: { width: 160, useAdvancedWrap: true }, align: 'center' }).setOrigin(0.5, 1);
    const w = Math.max(28, t.width + 14), h = t.height + 8;
    const g = this.scene.add.graphics();
    g.fillStyle(0xffffff, 0.96).lineStyle(1, 0x1b2433, 0.25);
    g.fillRoundedRect(-w / 2, -h, w, h, 8).strokeRoundedRect(-w / 2, -h, w, h, 8);
    g.fillTriangle(-5, 0, 5, 0, 0, 6);
    t.setPosition(0, -4);
    this.bubble = this.scene.add.container(0, -ART_H - 14, [g, t]);
    this.root.add(this.bubble);
    this.bubbleTimer = this.scene.time.delayedCall(6000, () => { this.bubble?.destroy(); this.bubble = null; });
  }

  destroy() {
    this.bubbleTimer?.remove();
    this.root.destroy();
  }
}

interface Remote {
  info: EntityInfo;
  avatar: Avatar;
  buffer: { t: number; x: number; y: number; dir: number; moving: boolean }[];
}

export class WorldScene extends Phaser.Scene {
  private deps!: SceneDeps;
  private hello: Hello | null = null;
  private self: Avatar | null = null;
  private selfName = '';
  private selfCharacterId = '';
  private pred: Point = [0, 0];
  private prevPred: Point = [0, 0];
  private correction: [number, number] = [0, 0];
  private pending: MoveInput[] = [];
  private seq = 0;
  private acc = 0;
  private remotes = new Map<number, Remote>();
  private follower: PathFollower | null = null;
  private targetNpc: string | null = null;
  private marker!: Phaser.GameObjects.Arc;
  private keys!: Record<string, Phaser.Input.Keyboard.Key>;
  private joy: [number, number] | null = null;
  private offsets: { at: number; o: number }[] = [];
  private unsub: (() => void)[] = [];
  private npcAvatars = new Map<string, Avatar>();
  private names: Record<string, string> = {};

  constructor() { super('world'); }

  init(deps: SceneDeps) { this.deps = deps; }

  preload() {
    this.load.tilemapTiledJSON('map', this.deps.tiled);
    this.load.image('tiles', this.deps.tilesUrl);
  }

  create() {
    const tm = this.make.tilemap({ key: 'map' });
    const tileset = tm.addTilesetImage('placeholder', 'tiles')!;
    tm.createLayer('ground', tileset)?.setDepth(-30);
    tm.createLayer('decor', tileset)?.setDepth(-20);
    tm.createLayer('obstacles', tileset)?.setDepth(-10);
    tm.createLayer('above', tileset)?.setDepth(100000);
    const cam = this.cameras.main;
    cam.setBounds(0, 0, tm.widthInPixels, tm.heightInPixels);
    cam.setBackgroundColor('#2f6e2c');
    cam.setRoundPixels(true);
    this.fitZoom();
    this.scale.on('resize', () => this.fitZoom());

    for (const npc of this.deps.map.npcs) {
      const name = this.names[npc.code] ?? this.deps.npcNames[npc.code] ?? '…';
      const a = new Avatar(this, 'npc:' + npc.code, NPC_PALETTES[npc.code] ?? paletteFor(npc.code), name, 'npc');
      a.place(npc.x / UNIT, npc.y / UNIT);
      // Hit area covers the whole figure and its name, generous for touch.
      const hit = this.add.zone(npc.x / UNIT, npc.y / UNIT - 30, 48, 72).setInteractive({ useHandCursor: true });
      hit.setData('npc', npc.code);
      const mark = viText(this, 0, -ART_H - 16, '▼', { fontSize: '13px', color: '#ffd43b', stroke: '#1b2433', strokeThickness: 3 }).setOrigin(0.5, 1);
      a.root.add(mark);
      this.tweens.add({ targets: mark, y: mark.y - 5, duration: 600, yoyo: true, repeat: -1, ease: 'Sine.inOut' });
      this.npcAvatars.set(npc.code, a);
    }

    this.marker = this.add.circle(0, 0, 9).setStrokeStyle(2, 0xffe066, 0.9).setVisible(false).setDepth(5);

    const kb = this.input.keyboard!;
    // enableCapture=false: keys typed into the chat box must reach the input.
    this.keys = kb.addKeys('W,A,S,D,UP,DOWN,LEFT,RIGHT', false) as Record<string, Phaser.Input.Keyboard.Key>;
    for (const k of Object.values(this.keys)) {
      // P0-005: any manual control cancels auto-path, even a tap shorter than a frame.
      k.on(Phaser.Input.Keyboard.Events.DOWN, () => { if (!typing()) this.cancelPath(); });
    }
    this.input.on('pointerdown', this.onPointerDown, this);

    const s = this.deps.socket;
    this.unsub.push(
      s.on<Hello>('hello', (h) => this.onHello(h)),
      s.on<{ room_id: string; entities: EntityInfo[] }>('room.snapshot', (p) => this.onRoomSnapshot(p.entities)),
      s.on<EntityInfo>('room.player_joined', (e) => this.addRemote(e)),
      s.on<{ entity_id: number }>('room.player_left', (p) => this.removeRemote(p.entity_id)),
      s.on<{ body: string; sender: { entity_id: number } }>('chat.public.delivered', (p) => this.bubble(p.sender.entity_id, p.body)),
      s.on<{ entity_id: number; appearance: Appearance }>('character.appearance.changed', (p) => {
        if (this.hello && p.entity_id === this.hello.entity_id) this.self?.setAppearance(p.appearance);
        else this.remotes.get(p.entity_id)?.avatar.setAppearance(p.appearance);
      }),
      s.on<{ entity_id: number; display_name: string }>('character.renamed', (p) => {
        if (this.hello && p.entity_id === this.hello.entity_id) {
          this.selfName = p.display_name;
          this.self?.setName(p.display_name);
        } else this.remotes.get(p.entity_id)?.avatar.setName(p.display_name);
      }),
      s.on<{ code: string }>('error', (p) => { if (p.code === 'MAP_VERSION_MISMATCH') this.deps.hooks.mapOutdated(); }),
    );
    s.onSnapshot = (snap, at) => this.onSnapshot(snap, at);
    this.events.once(Phaser.Scenes.Events.SHUTDOWN, () => {
      this.unsub.forEach((u) => u());
      s.onSnapshot = () => {};
    });
    this.deps.hooks.ready();
  }

  /** May be called before the scene has started; names are kept until create(). */
  setNpcNames(names: Record<string, string>) {
    Object.assign(this.names, names);
    for (const [code, name] of Object.entries(names)) this.npcAvatars.get(code)?.setName(name);
  }

  setSelf(characterId: string, name: string) {
    this.selfCharacterId = characterId;
    this.selfName = name;
  }

  private fitZoom() {
    const w = this.scale.width;
    this.cameras.main.setZoom(w >= 1700 ? 1.5 : w >= 1250 ? 1.25 : 1);
  }

  // ---------------------------------------------------------------------
  // Network
  // ---------------------------------------------------------------------

  private onHello(h: Hello) {
    this.hello = h;
    this.seq = 0;
    this.pending = [];
    this.cancelPath();
    if (h.map_version !== this.deps.map.version) this.deps.hooks.mapOutdated();
    this.deps.socket.send('client.ready', { map_version: this.deps.map.version });
  }

  private onRoomSnapshot(entities: EntityInfo[]) {
    for (const r of this.remotes.values()) r.avatar.destroy();
    this.remotes.clear();
    for (const e of entities) {
      if (this.hello && e.entity_id === this.hello.entity_id) {
        this.pred = [e.x, e.y];
        this.prevPred = [e.x, e.y];
        this.correction = [0, 0];
        if (!this.self) {
          this.self = new Avatar(this, this.selfCharacterId || e.character_id, paletteFor(e.character_id), this.selfName || e.display_name, 'self', e.appearance);
          this.cameras.main.startFollow(this.self.root, true, 0.15, 0.15);
        } else {
          this.self.setAppearance(e.appearance ?? null);
        }
        this.self.dir = e.dir;
        this.self.place(e.x / UNIT, e.y / UNIT);
        this.cameras.main.centerOn(e.x / UNIT, e.y / UNIT);
      } else this.addRemote(e);
    }
  }

  private addRemote(e: EntityInfo) {
    if (this.hello && e.entity_id === this.hello.entity_id) return;
    this.removeRemote(e.entity_id);
    const avatar = new Avatar(this, e.character_id, paletteFor(e.character_id), e.display_name, 'other', e.appearance);
    avatar.clickable(e.character_id);
    avatar.dir = e.dir;
    avatar.place(e.x / UNIT, e.y / UNIT);
    this.remotes.set(e.entity_id, { info: e, avatar, buffer: [] });
  }

  private removeRemote(id: number) {
    const r = this.remotes.get(id);
    if (r) {
      r.avatar.destroy();
      this.remotes.delete(id);
    }
  }

  private bubble(entityId: number, body: string) {
    if (this.hello && entityId === this.hello.entity_id) this.self?.say(body);
    else this.remotes.get(entityId)?.avatar.say(body);
  }

  private onSnapshot(snap: Snapshot, at: number) {
    if (!this.hello) return;
    const tickMs = this.hello.tick_ms;
    const t = snap.server_tick * tickMs;
    // Local-clock offset of the server tick timeline: the smallest delay seen
    // recently is the best estimate (jittery packets only arrive later).
    this.offsets.push({ at, o: at - t });
    while (this.offsets.length > 0 && at - this.offsets[0].at > 3000) this.offsets.shift();
    for (const e of snap.entities) {
      if (e.entity_id === this.hello.entity_id) {
        this.reconcile(e.x, e.y, snap.ack_seq);
        continue;
      }
      const r = this.remotes.get(e.entity_id);
      if (!r) continue;
      r.buffer.push({ t, x: e.x, y: e.y, dir: e.dir, moving: e.moving });
      if (r.buffer.length > 30) r.buffer.shift();
    }
  }

  /** Server position for our last acknowledged input; replay the rest. */
  private reconcile(sx: number, sy: number, ack: number) {
    this.pending = this.pending.filter((p) => p.seq > ack);
    let x = sx, y = sy;
    for (const p of this.pending) [x, y] = step(this.deps.map.world, x, y, p.dx, p.dy, this.hello!.speed_per_step);
    const ex = this.pred[0] - x, ey = this.pred[1] - y;
    if (ex === 0 && ey === 0) return;
    if (Math.hypot(ex, ey) > 64 * UNIT) {
      this.correction = [0, 0]; // far off (e.g. after a resume): snap
    } else {
      this.correction = [this.correction[0] + ex, this.correction[1] + ey];
    }
    this.pred = [x, y];
    this.prevPred = [this.prevPred[0] - ex, this.prevPred[1] - ey];
  }

  // ---------------------------------------------------------------------
  // Input
  // ---------------------------------------------------------------------

  private onPointerDown(p: Phaser.Input.Pointer, over: Phaser.GameObjects.GameObject[]) {
    if (!this.hello) return;
    const npcHit = over.find((o) => o.getData && o.getData('npc'));
    if (npcHit) {
      this.goToNpc(npcHit.getData('npc') as string);
      return;
    }
    const playerHit = over.find((o) => o.getData && o.getData('character'));
    if (playerHit) {
      this.deps.hooks.playerClicked(playerHit.getData('character') as string);
      return;
    }
    this.walkTo([Math.round(p.worldX * UNIT), Math.round(p.worldY * UNIT)]);
  }

  /** Virtual joystick vector in [-1, 1] (mobile, P0-003); null when released. */
  setJoystick(v: [number, number] | null) {
    this.joy = v && (v[0] || v[1]) ? v : null;
    if (this.joy) this.cancelPath();
  }

  private cancelPath() {
    this.follower = null;
    this.targetNpc = null;
    this.marker?.setVisible(false);
  }

  walkTo(target: Point, npc: string | null = null) {
    const path = findPath(this.deps.map.world, this.pred, target);
    if (!path) return;
    this.follower = new PathFollower(path, this.hello!.speed_per_step);
    this.targetNpc = npc;
    const end = path[path.length - 1];
    this.marker.setPosition(end[0] / UNIT, end[1] / UNIT).setVisible(!npc);
  }

  private goToNpc(code: string) {
    const npc = this.deps.map.npcs.find((n) => n.code === code);
    if (!npc) return;
    if (Math.hypot(this.pred[0] - npc.x, this.pred[1] - npc.y) <= npc.interactRadius - 1000) {
      this.cancelPath();
      this.deps.hooks.npcReached(code);
      return;
    }
    // Stand just in front of (below) the NPC, or beside it if that is blocked.
    const w = this.deps.map.world;
    const spots: Point[] = [[npc.x, npc.y + 4000], [npc.x - 4200, npc.y + 1000], [npc.x + 4200, npc.y + 1000], [npc.x, npc.collider.y0 - 1200]];
    const spot = spots.find(([x, y]) => !collides(w, x, y)) ?? spots[0];
    this.walkTo(spot, code);
  }

  private keyDir(): [number, number] | null {
    if (typing()) return null;
    const k = this.keys;
    const h = Number(k.D.isDown || k.RIGHT.isDown) - Number(k.A.isDown || k.LEFT.isDown);
    const v = Number(k.S.isDown || k.DOWN.isDown) - Number(k.W.isDown || k.UP.isDown);
    if (!h && !v) return null;
    return h && v ? [h * 707, v * 707] : [h * 1000, v * 1000];
  }

  // ---------------------------------------------------------------------
  // Simulation and rendering
  // ---------------------------------------------------------------------

  /** One fixed step: one input, applied locally and sent to the server. */
  private fixedStep() {
    if (!this.hello || !this.self || this.deps.socket.status !== 'online') return;
    this.prevPred = [this.pred[0], this.pred[1]];
    let dir = this.keyDir();
    if (!dir && this.joy) dir = [Math.round(this.joy[0] * 1000), Math.round(this.joy[1] * 1000)];
    if (dir) {
      if (this.follower) this.cancelPath();
    } else if (this.follower) {
      dir = this.follower.next(this.pred);
      if (!dir) this.arrive();
    }
    if (!dir) { this.self.moving = false; return; }
    const [dx, dy] = clampDir(dir[0], dir[1]);
    if (!dx && !dy) return;
    const m = { seq: ++this.seq, dx, dy };
    this.pred = step(this.deps.map.world, this.pred[0], this.pred[1], dx, dy, this.hello.speed_per_step);
    this.pending.push(m);
    this.deps.socket.sendMove(m);
    const d = dir8(dx, dy);
    if (d !== null) this.self.dir = d;
    this.self.moving = this.pred[0] !== this.prevPred[0] || this.pred[1] !== this.prevPred[1];
  }

  private arrive() {
    const npc = this.targetNpc;
    this.cancelPath();
    if (!npc) return;
    const n = this.deps.map.npcs.find((x) => x.code === npc);
    if (n && Math.hypot(this.pred[0] - n.x, this.pred[1] - n.y) <= n.interactRadius) this.deps.hooks.npcReached(npc);
  }

  update(time: number, delta: number) {
    if (!this.hello) return;
    const tickMs = this.hello.tick_ms;
    this.acc = Math.min(this.acc + delta, tickMs * 5);
    while (this.acc >= tickMs) {
      this.acc -= tickMs;
      this.fixedStep();
    }
    if (this.self) {
      const a = this.acc / tickMs;
      this.correction = [this.correction[0] * 0.82, this.correction[1] * 0.82];
      if (Math.abs(this.correction[0]) < 20 && Math.abs(this.correction[1]) < 20) this.correction = [0, 0];
      const x = this.prevPred[0] + (this.pred[0] - this.prevPred[0]) * a + this.correction[0];
      const y = this.prevPred[1] + (this.pred[1] - this.prevPred[1]) * a + this.correction[1];
      this.self.place(x / UNIT, y / UNIT);
      this.self.animate(time);
    }
    this.renderRemotes(time);
  }

  private renderRemotes(time: number) {
    if (this.offsets.length === 0) return;
    let offset = Infinity;
    for (const s of this.offsets) offset = Math.min(offset, s.o);
    const renderT = performance.now() - offset - INTERP_DELAY_MS;
    for (const r of this.remotes.values()) {
      const b = r.buffer;
      if (b.length === 0) { r.avatar.animate(time); continue; }
      let x = b[b.length - 1].x, y = b[b.length - 1].y, dir = b[b.length - 1].dir, moving = b[b.length - 1].moving;
      for (let i = b.length - 1; i > 0; i--) {
        if (b[i - 1].t <= renderT && renderT <= b[i].t) {
          const k = (renderT - b[i - 1].t) / (b[i].t - b[i - 1].t || 1);
          x = b[i - 1].x + (b[i].x - b[i - 1].x) * k;
          y = b[i - 1].y + (b[i].y - b[i - 1].y) * k;
          dir = b[i].dir;
          moving = b[i].moving || b[i].x !== b[i - 1].x || b[i].y !== b[i - 1].y;
          break;
        }
      }
      if (renderT < b[0].t) { x = b[0].x; y = b[0].y; }
      r.avatar.dir = dir;
      r.avatar.moving = moving;
      r.avatar.place(x / UNIT, y / UNIT);
      r.avatar.animate(time);
    }
  }
}

function typing(): boolean {
  const el = document.activeElement as HTMLElement | null;
  return !!el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable);
}
