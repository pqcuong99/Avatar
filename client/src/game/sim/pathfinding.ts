// Click/tap-to-move pathfinding (P0-003/004/168). The client plans the route;
// the server still validates every step it takes.
//
// A* runs on the tile grid with 8 neighbours and no corner cutting; tiles
// touched by an NPC collider count as blocked. The tile path is then
// shortened by line-of-sight checks with the real collider so the character
// walks straight lines instead of tile zig-zags.
import { collides, type SimWorld } from './movement.ts';

export type Point = [number, number];

function tileBlocked(w: SimWorld, tx: number, ty: number, solidTiles: Set<number>): boolean {
  if (tx < 0 || ty < 0 || tx >= w.cols || ty >= w.rows) return true;
  const i = ty * w.cols + tx;
  return w.blocked[i] === 1 || solidTiles.has(i);
}

function solidTileSet(w: SimWorld): Set<number> {
  const s = new Set<number>();
  for (const r of w.solids) {
    for (let ty = Math.floor(r.y0 / w.tileSize); ty <= Math.floor((r.y1 - 1) / w.tileSize); ty++)
      for (let tx = Math.floor(r.x0 / w.tileSize); tx <= Math.floor((r.x1 - 1) / w.tileSize); tx++) s.add(ty * w.cols + tx);
  }
  return s;
}

const center = (w: SimWorld, tx: number, ty: number): Point => [tx * w.tileSize + w.tileSize / 2, ty * w.tileSize + w.tileSize / 2];

/** Nearest free tile to (tx,ty) by breadth-first search, limited radius. */
function nearestFree(w: SimWorld, tx: number, ty: number, solids: Set<number>, limit = 6): Point | null {
  if (!tileBlocked(w, tx, ty, solids)) return [tx, ty];
  for (let r = 1; r <= limit; r++) {
    let best: Point | null = null, bestD = Infinity;
    for (let dy = -r; dy <= r; dy++) for (let dx = -r; dx <= r; dx++) {
      if (Math.max(Math.abs(dx), Math.abs(dy)) !== r) continue;
      if (!tileBlocked(w, tx + dx, ty + dy, solids)) {
        const d = dx * dx + dy * dy;
        if (d < bestD) { bestD = d; best = [tx + dx, ty + dy]; }
      }
    }
    if (best) return best;
  }
  return null;
}

/** Is the straight segment a->b walkable for the player's collider? */
export function clearLine(w: SimWorld, a: Point, b: Point): boolean {
  const dist = Math.hypot(b[0] - a[0], b[1] - a[1]);
  const steps = Math.max(1, Math.ceil(dist / 300)); // probe every 3 px
  for (let i = 1; i <= steps; i++) {
    const t = i / steps;
    if (collides(w, Math.round(a[0] + (b[0] - a[0]) * t), Math.round(a[1] + (b[1] - a[1]) * t))) return false;
  }
  return true;
}

/**
 * Returns waypoints (units) from `from` to `to`, excluding the start, or null
 * when the target cannot be reached. If `to` itself is blocked the route ends
 * at the nearest reachable free spot.
 */
export function findPath(w: SimWorld, from: Point, to: Point, maxNodes = 6000): Point[] | null {
  const solids = solidTileSet(w);
  const sx = Math.floor(from[0] / w.tileSize), sy = Math.floor(from[1] / w.tileSize);
  const goal = nearestFree(w, Math.floor(to[0] / w.tileSize), Math.floor(to[1] / w.tileSize), solids);
  if (!goal) return null;
  const [gx, gy] = goal;
  const target: Point = collides(w, to[0], to[1]) || gx !== Math.floor(to[0] / w.tileSize) || gy !== Math.floor(to[1] / w.tileSize)
    ? center(w, gx, gy) : to;
  if (clearLine(w, from, target)) return [target];

  const key = (x: number, y: number) => y * w.cols + x;
  const open: { x: number; y: number; f: number }[] = [{ x: sx, y: sy, f: 0 }];
  const g = new Map<number, number>([[key(sx, sy), 0]]);
  const came = new Map<number, number>();
  const h = (x: number, y: number) => {
    const dx = Math.abs(x - gx), dy = Math.abs(y - gy);
    return Math.max(dx, dy) + (Math.SQRT2 - 1) * Math.min(dx, dy);
  };
  let expanded = 0;
  while (open.length > 0 && expanded++ < maxNodes) {
    let bi = 0;
    for (let i = 1; i < open.length; i++) if (open[i].f < open[bi].f) bi = i;
    const cur = open.splice(bi, 1)[0];
    if (cur.x === gx && cur.y === gy) {
      const tiles: Point[] = [];
      let k = key(gx, gy);
      while (k !== key(sx, sy)) {
        tiles.push(center(w, k % w.cols, Math.floor(k / w.cols)));
        k = came.get(k)!;
      }
      tiles.reverse();
      tiles[tiles.length - 1] = target;
      return smooth(w, from, tiles);
    }
    const cg = g.get(key(cur.x, cur.y))!;
    for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) {
      if (!dx && !dy) continue;
      const nx = cur.x + dx, ny = cur.y + dy;
      if (tileBlocked(w, nx, ny, solids)) continue;
      // no corner cutting
      if (dx && dy && (tileBlocked(w, cur.x + dx, cur.y, solids) || tileBlocked(w, cur.x, cur.y + dy, solids))) continue;
      const ng = cg + (dx && dy ? Math.SQRT2 : 1);
      const nk = key(nx, ny);
      if (ng < (g.get(nk) ?? Infinity)) {
        g.set(nk, ng);
        came.set(nk, key(cur.x, cur.y));
        open.push({ x: nx, y: ny, f: ng + h(nx, ny) });
      }
    }
  }
  return null;
}

/** Drop waypoints that can be skipped with a straight, collision-free line. */
function smooth(w: SimWorld, from: Point, pts: Point[]): Point[] {
  const out: Point[] = [];
  let anchor = from;
  let i = 0;
  while (i < pts.length) {
    let j = pts.length - 1;
    while (j > i && !clearLine(w, anchor, pts[j])) j--;
    out.push(pts[j]);
    anchor = pts[j];
    i = j + 1;
  }
  return out;
}
