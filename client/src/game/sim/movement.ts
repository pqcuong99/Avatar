// Client copy of the server movement rules (game-server/internal/sim/sim.go),
// used for prediction. It must stay identical rule for rule: integer math in
// 1/100 px, direction scaled by 1000, x then y in sub-steps of one pixel.
// shared/fixtures/movement_cases.json is replayed by movement.test.ts.

export const UNIT = 100;
export const DIR_SCALE = 1000;
const SUB_STEP = 1 * UNIT;
export const HALF_WIDTH = 9 * UNIT;
export const HALF_HEIGHT = 5 * UNIT;
const MAX_COMPONENT = 1 << 20;

export interface Rect { x0: number; y0: number; x1: number; y1: number }

export interface SimWorld {
  width: number; // units
  height: number;
  tileSize: number; // units
  cols: number;
  rows: number;
  blocked: Uint8Array; // cols*rows
  solids: Rect[]; // NPC colliders
}

// Integer division truncating towards zero, like Go. Exact for the ranges used
// here (|a| < 2^31, |b| < 2^21), see sim.go.
const idiv = (a: number, b: number) => Math.trunc(a / b);

export function clampDir(dx: number, dy: number): [number, number] {
  if (Math.abs(dx) > MAX_COMPONENT || Math.abs(dy) > MAX_COMPONENT) return [0, 0];
  const l2 = dx * dx + dy * dy;
  if (l2 <= 1001 * 1001) return [dx, dy];
  const l = Math.floor(Math.sqrt(l2));
  return [idiv(dx * DIR_SCALE, l), idiv(dy * DIR_SCALE, l)];
}

export function collides(w: SimWorld, x: number, y: number): boolean {
  const x0 = x - HALF_WIDTH, x1 = x + HALF_WIDTH;
  const y0 = y - HALF_HEIGHT, y1 = y + HALF_HEIGHT;
  if (x0 < 0 || y0 < 0 || x1 > w.width || y1 > w.height) return true;
  for (let ty = idiv(y0, w.tileSize); ty <= idiv(y1 - 1, w.tileSize); ty++) {
    for (let tx = idiv(x0, w.tileSize); tx <= idiv(x1 - 1, w.tileSize); tx++) {
      if (w.blocked[ty * w.cols + tx]) return true;
    }
  }
  for (const s of w.solids) {
    if (x0 < s.x1 && s.x0 < x1 && y0 < s.y1 && s.y0 < y1) return true;
  }
  return false;
}

const clampAbs = (v: number, limit: number) => (v > limit ? limit : v < -limit ? -limit : v);

export function step(w: SimWorld, x: number, y: number, dx: number, dy: number, speed: number): [number, number] {
  [dx, dy] = clampDir(dx, dy);
  let mx = idiv(dx * speed, DIR_SCALE);
  let my = idiv(dy * speed, DIR_SCALE);
  while (mx !== 0) {
    const s = clampAbs(mx, SUB_STEP);
    if (collides(w, x + s, y)) break;
    x += s;
    mx -= s;
  }
  while (my !== 0) {
    const s = clampAbs(my, SUB_STEP);
    if (collides(w, x, y + s)) break;
    y += s;
    my -= s;
  }
  return [x, y];
}

// 0=S 1=SW 2=W 3=NW 4=N 5=NE 6=E 7=SE (display only, not part of the rules).
export function dir8(dx: number, dy: number): number | null {
  if (dx === 0 && dy === 0) return null;
  let a = Math.atan2(-dx, dy);
  if (a < 0) a += 2 * Math.PI;
  return Math.round(a / (Math.PI / 4)) % 8;
}

// ---------------------------------------------------------------------------
// Tiled map -> SimWorld (same reading as game-server/internal/mapdata)
// ---------------------------------------------------------------------------

interface TiledProperty { name: string; type: string; value: unknown }
interface TiledObject { name: string; x: number; y: number; width: number; height: number; point?: boolean; properties?: TiledProperty[] }
interface TiledLayer { name: string; type: string; data?: number[]; objects?: TiledObject[] }
export interface TiledMap {
  width: number; height: number; tilewidth: number; tileheight: number;
  properties?: TiledProperty[];
  layers: TiledLayer[];
}

export interface MapNPC { code: string; x: number; y: number; collider: Rect; interactRadius: number }

export interface LoadedMap {
  id: string;
  version: string;
  displayName: string;
  world: SimWorld;
  npcs: MapNPC[];
  spawn: [number, number];
}

export function prop<T>(props: TiledProperty[] | undefined, name: string): T | undefined {
  return props?.find((p) => p.name === name)?.value as T | undefined;
}

const px = (v: number) => Math.trunc(v * UNIT);

export function loadMap(tm: TiledMap): LoadedMap {
  const tile = tm.tilewidth * UNIT;
  const world: SimWorld = {
    width: tm.width * tile, height: tm.height * tile, tileSize: tile,
    cols: tm.width, rows: tm.height, blocked: new Uint8Array(tm.width * tm.height), solids: [],
  };
  const npcs: MapNPC[] = [];
  let spawn: [number, number] = [0, 0];
  for (const l of tm.layers) {
    if (l.type === 'tilelayer' && l.name === 'collision' && l.data) {
      l.data.forEach((gid, i) => { world.blocked[i] = gid !== 0 ? 1 : 0; });
    } else if (l.type === 'objectgroup' && l.name === 'npcs') {
      for (const o of l.objects ?? []) {
        const code = prop<string>(o.properties, 'npc_code') ?? o.name;
        const radius = prop<number>(o.properties, 'interact_radius') ?? 80;
        const r = { x0: px(o.x), y0: px(o.y), x1: px(o.x + o.width), y1: px(o.y + o.height) };
        npcs.push({ code, x: idiv(r.x0 + r.x1, 2), y: r.y1, collider: r, interactRadius: px(radius) });
        world.solids.push(r);
      }
    } else if (l.type === 'objectgroup' && l.name === 'spawns') {
      const d = l.objects?.find((o) => o.name === 'default');
      if (d) spawn = [px(d.x), px(d.y)];
    }
  }
  return {
    id: prop<string>(tm.properties, 'map_id') ?? 'unknown',
    version: prop<string>(tm.properties, 'map_version') ?? 'unknown',
    displayName: prop<string>(tm.properties, 'display_name') ?? '',
    world, npcs, spawn,
  };
}
