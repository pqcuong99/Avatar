// Generates the DEV SAMPLE map of Project Thanh:
//   shared/maps/tiles_placeholder.png  original procedural 32x32 tileset (no third-party art)
//   shared/maps/plaza.json             Tiled JSON map (open/edit it in Tiled Map Editor)
//   shared/maps/maps.json              map registry read by the Game Server
//
// The map list, NPC positions and art are not decided (GAP-13, GAP-19): this is
// a placeholder that exercises collision, NPCs, rooms and movement. Re-run with
//   node tools/mapgen/generate-plaza.mjs
// and bump MAP_VERSION whenever the output changes, so clients with an old map
// are told to reload (W-08).
import { writeFileSync, mkdirSync } from 'node:fs';
import { deflateSync } from 'node:zlib';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const MAP_ID = 'plaza';
const MAP_VERSION = 'plaza@1';
const OUT = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'maps');
const T = 32;
const COLS = 8;
const ROWS = 6;

// ---------------------------------------------------------------------------
// Tiny RGBA canvas + PNG encoder
// ---------------------------------------------------------------------------
class Canvas {
  constructor(w, h) { this.w = w; this.h = h; this.d = new Uint8ClampedArray(w * h * 4); }
  px(x, y, c, a = 1) {
    x = Math.floor(x); y = Math.floor(y);
    if (x < 0 || y < 0 || x >= this.w || y >= this.h) return;
    const i = (y * this.w + x) * 4;
    const [r, g, b] = c;
    const sa = a, da = this.d[i + 3] / 255, oa = sa + da * (1 - sa);
    if (oa === 0) return;
    this.d[i] = (r * sa + this.d[i] * da * (1 - sa)) / oa;
    this.d[i + 1] = (g * sa + this.d[i + 1] * da * (1 - sa)) / oa;
    this.d[i + 2] = (b * sa + this.d[i + 2] * da * (1 - sa)) / oa;
    this.d[i + 3] = oa * 255;
  }
  rect(x, y, w, h, c, a = 1) { for (let j = 0; j < h; j++) for (let i = 0; i < w; i++) this.px(x + i, y + j, c, a); }
  ellipse(cx, cy, rx, ry, c, a = 1) {
    for (let y = Math.floor(cy - ry); y <= Math.ceil(cy + ry); y++)
      for (let x = Math.floor(cx - rx); x <= Math.ceil(cx + rx); x++) {
        const dx = (x + 0.5 - cx) / rx, dy = (y + 0.5 - cy) / ry;
        if (dx * dx + dy * dy <= 1) this.px(x, y, c, a);
      }
  }
  png() {
    const raw = Buffer.alloc((this.w * 4 + 1) * this.h);
    for (let y = 0; y < this.h; y++) {
      raw[y * (this.w * 4 + 1)] = 0;
      Buffer.from(this.d.buffer, y * this.w * 4, this.w * 4).copy(raw, y * (this.w * 4 + 1) + 1);
    }
    const chunk = (type, data) => {
      const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
      const td = Buffer.concat([Buffer.from(type), data]);
      const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(td) >>> 0);
      return Buffer.concat([len, td, crc]);
    };
    const ihdr = Buffer.alloc(13);
    ihdr.writeUInt32BE(this.w, 0); ihdr.writeUInt32BE(this.h, 4);
    ihdr[8] = 8; ihdr[9] = 6; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0;
    return Buffer.concat([
      Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
      chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw, { level: 9 })), chunk('IEND', Buffer.alloc(0)),
    ]);
  }
}
const CRC = new Int32Array(256).map((_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c; });
function crc32(buf) { let c = -1; for (const b of buf) c = CRC[(c ^ b) & 255] ^ (c >>> 8); return c ^ -1; }

// Deterministic PRNG so the output (and MAP_VERSION) is reproducible.
function mulberry(seed) { return () => { seed |= 0; seed = seed + 0x6d2b79f5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; }; }
const hex = (h) => [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];

// ---------------------------------------------------------------------------
// Tiles
// ---------------------------------------------------------------------------
const TILE = {
  grass: 0, grassDark: 1, flowersY: 2, flowersR: 3, path: 4, dirt: 5, sand: 6, plaza: 7,
  water: 8, waterEdge: 9, soil: 10, sprouts: 11, wood: 12, rugRed: 13, rugBlue: 14, flowerbed: 15,
  canopy: 16, tree: 17, bush: 18, rock: 19, fenceH: 20, fenceV: 21, fencePost: 22, bench: 23,
  fTL: 24, fTC: 25, fTR: 26, fML: 27, fMC: 28, fMR: 29, lamp: 30, sign: 31,
  fBL: 32, fBC: 33, fBR: 34, roofL: 35, roofM: 36, roofR: 37, counter: 38, crate: 39,
  blocked: 40,
};
const BLOCKING = new Set([TILE.tree, TILE.bush, TILE.rock, TILE.fenceH, TILE.fenceV, TILE.fencePost, TILE.bench,
  TILE.fTL, TILE.fTC, TILE.fTR, TILE.fML, TILE.fMC, TILE.fMR, TILE.fBL, TILE.fBC, TILE.fBR, TILE.lamp, TILE.sign,
  TILE.roofL, TILE.roofM, TILE.roofR, TILE.counter, TILE.crate, TILE.water, TILE.waterEdge]);

const sheet = new Canvas(COLS * T, ROWS * T);
const rnd = mulberry(20261021);
function tile(id, draw) {
  const ox = (id % COLS) * T, oy = Math.floor(id / COLS) * T;
  draw({
    px: (x, y, c, a) => x >= 0 && y >= 0 && x < T && y < T && sheet.px(ox + x, oy + y, hex(c), a),
    rect: (x, y, w, h, c, a) => { for (let j = 0; j < h; j++) for (let i = 0; i < w; i++) if (x + i >= 0 && y + j >= 0 && x + i < T && y + j < T) sheet.px(ox + x + i, oy + y + j, hex(c), a); },
    ellipse: (cx, cy, rx, ry, c, a) => {
      for (let y = 0; y < T; y++) for (let x = 0; x < T; x++) {
        const dx = (x + 0.5 - cx) / rx, dy = (y + 0.5 - cy) / ry;
        if (dx * dx + dy * dy <= 1) sheet.px(ox + x, oy + y, hex(c), a);
      }
    },
  });
}
const speckle = (g, base, spots, n) => { g.rect(0, 0, T, T, base); for (let i = 0; i < n; i++) g.px(rnd() * T, rnd() * T, spots[Math.floor(rnd() * spots.length)]); };
const grassBase = (g) => speckle(g, '#6dbb4f', ['#5ea944', '#7fcb5c', '#64b249'], 70);

tile(TILE.grass, grassBase);
tile(TILE.grassDark, (g) => speckle(g, '#62ad47', ['#559c3d', '#70bb54'], 80));
tile(TILE.flowersY, (g) => { grassBase(g); for (let i = 0; i < 5; i++) { const x = 4 + rnd() * 24, y = 4 + rnd() * 24; g.ellipse(x, y, 2, 2, '#ffe066'); g.px(x, y, '#f08c00'); } });
tile(TILE.flowersR, (g) => { grassBase(g); for (let i = 0; i < 5; i++) { const x = 4 + rnd() * 24, y = 4 + rnd() * 24; g.ellipse(x, y, 2, 2, '#ff8fab'); g.px(x, y, '#c9184a'); } });
tile(TILE.path, (g) => {
  g.rect(0, 0, T, T, '#cdbd9c');
  for (let r = 0; r < 4; r++) for (let c = 0; c < 4; c++) {
    const off = r % 2 ? 4 : 0;
    g.rect(c * 8 + off + 1, r * 8 + 1, 6, 6, rnd() < 0.5 ? '#d8c9a8' : '#c4b290');
  }
});
tile(TILE.dirt, (g) => speckle(g, '#b8925f', ['#a8834f', '#c7a170'], 60));
tile(TILE.sand, (g) => speckle(g, '#ecdba6', ['#e0cc8f', '#f5e7bd'], 60));
tile(TILE.plaza, (g) => { g.rect(0, 0, T, T, '#ddd3c2'); g.rect(0, 0, T, 1, '#c8bca8'); g.rect(0, 0, 1, T, '#c8bca8'); g.rect(16, 0, 1, T, '#cfc4b1'); g.rect(0, 16, T, 1, '#cfc4b1'); });
tile(TILE.water, (g) => { g.rect(0, 0, T, T, '#4aa3df'); for (let i = 0; i < 6; i++) { const x = rnd() * 26, y = rnd() * 30; g.rect(x, y, 6, 1, '#7cc4f0'); } });
tile(TILE.waterEdge, (g) => { g.rect(0, 0, T, T, '#4aa3df'); g.rect(0, 0, T, 6, '#ecdba6'); g.rect(0, 6, T, 2, '#3b8cc4'); for (let i = 0; i < 4; i++) g.rect(rnd() * 26, 10 + rnd() * 20, 6, 1, '#7cc4f0'); });
tile(TILE.soil, (g) => { g.rect(0, 0, T, T, '#8b5a2b'); for (let y = 3; y < T; y += 8) g.rect(0, y, T, 2, '#6e4420'); });
tile(TILE.sprouts, (g) => { g.rect(0, 0, T, T, '#8b5a2b'); for (let y = 3; y < T; y += 8) g.rect(0, y, T, 2, '#6e4420'); for (let x = 4; x < T; x += 8) for (let y = 6; y < T; y += 8) { g.rect(x, y - 3, 1, 3, '#3f8f2f'); g.px(x - 1, y - 3, '#5cb646'); g.px(x + 1, y - 4, '#5cb646'); } });
tile(TILE.wood, (g) => { g.rect(0, 0, T, T, '#b9834b'); for (let y = 0; y < T; y += 8) g.rect(0, y, T, 1, '#94643a'); for (let y = 0; y < T; y += 8) g.rect((y * 3) % T, y, 1, 8, '#94643a'); });
tile(TILE.rugRed, (g) => { g.rect(0, 0, T, T, '#c0392b'); g.rect(2, 2, T - 4, T - 4, '#d35445'); g.rect(6, 6, T - 12, T - 12, '#e8b04a'); g.rect(9, 9, T - 18, T - 18, '#d35445'); });
tile(TILE.rugBlue, (g) => { g.rect(0, 0, T, T, '#2e5c8a'); g.rect(3, 3, T - 6, T - 6, '#3d77b0'); for (let i = 6; i < T - 6; i += 5) g.rect(i, 6, 2, T - 12, '#f1e3b8', 0.6); });
tile(TILE.flowerbed, (g) => { g.rect(0, 0, T, T, '#7a5230'); g.rect(1, 1, T - 2, T - 2, '#5d3b1f'); for (let i = 0; i < 9; i++) { const x = 5 + (i % 3) * 11, y = 5 + Math.floor(i / 3) * 11; g.ellipse(x, y, 3, 3, ['#ff6b9a', '#ffd43b', '#b197fc'][i % 3]); g.px(x, y, '#fff3bf'); } });
// Tree: canopy top half lives in the "above" layer, canopy bottom + trunk blocks.
tile(TILE.canopy, (g) => { g.ellipse(16, 30, 15, 14, '#2f6e2c'); g.ellipse(15, 28, 13, 12, '#3e8e3a'); g.ellipse(11, 24, 6, 5, '#57a84e'); });
tile(TILE.tree, (g) => { g.ellipse(16, 27, 11, 4, '#000000', 0.18); g.rect(13, 12, 6, 15, '#7a4f2a'); g.rect(13, 12, 2, 15, '#8f6136'); g.ellipse(16, 2, 15, 13, '#2f6e2c'); g.ellipse(15, 0, 13, 11, '#3e8e3a'); });
tile(TILE.bush, (g) => { g.ellipse(16, 25, 13, 5, '#000000', 0.18); g.ellipse(16, 18, 13, 10, '#3f8a35'); g.ellipse(12, 15, 7, 6, '#4fa144'); g.ellipse(21, 16, 6, 5, '#5cb34f'); g.px(10, 14, '#ff8fab'); g.px(22, 13, '#ff8fab'); });
tile(TILE.rock, (g) => { g.ellipse(16, 25, 12, 4, '#000000', 0.18); g.ellipse(16, 19, 12, 9, '#868d94'); g.ellipse(13, 16, 7, 5, '#a3aab1'); g.rect(18, 20, 6, 1, '#6d747b'); });
tile(TILE.fenceH, (g) => { g.rect(0, 12, T, 4, '#c08a50'); g.rect(0, 20, T, 4, '#c08a50'); g.rect(0, 15, T, 1, '#94643a'); g.rect(0, 23, T, 1, '#94643a'); for (const x of [3, 25]) { g.rect(x, 8, 4, 20, '#a8733f'); g.rect(x, 8, 4, 2, '#d9a066'); } });
tile(TILE.fenceV, (g) => { g.rect(12, 0, 4, T, '#c08a50'); g.rect(15, 0, 1, T, '#94643a'); for (const y of [4, 20]) { g.rect(10, y, 8, 4, '#a8733f'); } });
tile(TILE.fencePost, (g) => { g.rect(11, 6, 8, 22, '#a8733f'); g.rect(11, 6, 8, 3, '#d9a066'); g.rect(17, 9, 2, 19, '#8a5c30'); });
tile(TILE.bench, (g) => { g.ellipse(16, 26, 14, 3, '#000000', 0.15); g.rect(3, 12, 26, 5, '#a0682f'); g.rect(3, 12, 26, 1, '#c98d4f'); g.rect(3, 18, 26, 4, '#8a5826'); g.rect(5, 22, 3, 5, '#5c3a1a'); g.rect(24, 22, 3, 5, '#5c3a1a'); });
// Fountain 3x3: stone rim with water.
const fountain = (id, sx, sy) => tile(id, (g) => {
  for (let y = 0; y < T; y++) for (let x = 0; x < T; x++) {
    const X = sx * T + x + 0.5, Y = sy * T + y + 0.5, d = Math.hypot(X - 48, Y - 48);
    if (d <= 46) g.px(x, y, d > 40 ? (d > 44 ? '#8f8a80' : '#b9b2a6') : d < 10 ? '#d0ecff' : (Math.floor(X + Y) % 9 === 0 ? '#7cc4f0' : '#4aa3df'));
  }
  if (sx === 1 && sy === 1) { g.ellipse(16, 16, 6, 6, '#b9b2a6'); g.ellipse(16, 14, 3, 5, '#e7f6ff'); }
});
[[TILE.fTL, 0, 0], [TILE.fTC, 1, 0], [TILE.fTR, 2, 0], [TILE.fML, 0, 1], [TILE.fMC, 1, 1], [TILE.fMR, 2, 1], [TILE.fBL, 0, 2], [TILE.fBC, 1, 2], [TILE.fBR, 2, 2]].forEach(([id, x, y]) => fountain(id, x, y));
tile(TILE.lamp, (g) => { g.ellipse(16, 28, 7, 2, '#000000', 0.2); g.rect(14, 8, 4, 20, '#3d4451'); g.rect(10, 26, 12, 3, '#3d4451'); g.ellipse(16, 6, 6, 6, '#fff3bf'); g.ellipse(16, 6, 3, 3, '#ffffff'); g.rect(10, 1, 12, 2, '#3d4451'); });
tile(TILE.sign, (g) => { g.rect(15, 14, 3, 14, '#7a4f2a'); g.rect(5, 5, 22, 12, '#c08a50'); g.rect(5, 5, 22, 2, '#d9a066'); g.rect(8, 9, 16, 1, '#6e4420'); g.rect(8, 12, 12, 1, '#6e4420'); });
const roof = (id, left, right) => tile(id, (g) => { for (let x = 0; x < T; x++) g.rect(x, 4, 1, 24, Math.floor(x / 8) % 2 ? '#e8590c' : '#fff4e6'); g.rect(0, 26, T, 3, '#c2410c'); if (left) g.rect(0, 4, 3, 24, '#c2410c'); if (right) g.rect(T - 3, 4, 3, 24, '#c2410c'); });
roof(TILE.roofL, true, false); roof(TILE.roofM, false, false); roof(TILE.roofR, false, true);
tile(TILE.counter, (g) => { g.rect(0, 2, T, 24, '#a0682f'); g.rect(0, 2, T, 4, '#c98d4f'); g.rect(0, 14, T, 1, '#7a4f2a'); g.ellipse(16, 28, 15, 3, '#000000', 0.15); });
tile(TILE.crate, (g) => { g.rect(4, 6, 24, 22, '#b9834b'); g.rect(4, 6, 24, 2, '#d9a066'); g.rect(4, 16, 24, 2, '#94643a'); g.rect(4, 6, 2, 22, '#94643a'); g.rect(26, 6, 2, 22, '#94643a'); g.ellipse(16, 8, 4, 2, '#e8590c'); });
tile(TILE.blocked, (g) => { g.rect(0, 0, T, T, '#ff0000', 0.25); for (let i = 0; i < T; i++) { g.px(i, i, '#ff0000', 0.8); g.px(T - 1 - i, i, '#ff0000', 0.8); } });

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------
const W = 48, H = 32;
const layer = () => new Array(W * H).fill(-1);
const ground = layer(), decor = layer(), obstacles = layer(), above = layer();
const put = (l, x, y, id) => { if (x >= 0 && y >= 0 && x < W && y < H) l[y * W + x] = id; };
const get = (l, x, y) => l[y * W + x];
const fill = (l, x0, y0, x1, y1, id) => { for (let y = y0; y <= y1; y++) for (let x = x0; x <= x1; x++) put(l, x, y, typeof id === 'function' ? id(x, y) : id); };
const r2 = mulberry(7);

fill(ground, 0, 0, W - 1, H - 1, () => { const v = r2(); return v < 0.12 ? TILE.grassDark : v < 0.15 ? TILE.flowersY : v < 0.18 ? TILE.flowersR : TILE.grass; });

// Paths and the central plaza.
fill(ground, 2, 15, W - 3, 16, TILE.path);
fill(ground, 23, 2, 24, H - 3, TILE.path);
fill(ground, 17, 11, 30, 21, TILE.plaza);
fill(obstacles, 22, 14, 24, 16, (x, y) => [[TILE.fTL, TILE.fTC, TILE.fTR], [TILE.fML, TILE.fMC, TILE.fMR], [TILE.fBL, TILE.fBC, TILE.fBR]][y - 14][x - 22]);
for (const [x, y] of [[17, 11], [30, 11], [17, 21], [30, 21]]) put(obstacles, x, y, TILE.lamp);
for (const [x, y] of [[19, 12], [28, 12], [19, 20], [28, 20]]) put(obstacles, x, y, TILE.bench);
for (const [x, y] of [[20, 13], [27, 13], [20, 19], [27, 19]]) put(decor, x, y, TILE.flowerbed);

// Farm field (top right) with the farm shop stall at its gate.
fill(ground, 33, 3, 44, 10, TILE.dirt);
fill(decor, 34, 4, 43, 9, (x, y) => (y % 2 ? TILE.sprouts : TILE.soil));
for (let x = 33; x <= 44; x++) { put(obstacles, x, 3, TILE.fenceH); if (x < 37 || x > 40) put(obstacles, x, 10, TILE.fenceH); }
for (let y = 4; y <= 9; y++) { put(obstacles, 33, y, TILE.fenceV); put(obstacles, 44, y, TILE.fenceV); }
for (const [x, y] of [[33, 3], [44, 3], [33, 10], [44, 10], [36, 10], [41, 10]]) put(obstacles, x, y, TILE.fencePost);
fill(ground, 37, 10, 40, 14, TILE.dirt);
fill(ground, 25, 13, 40, 14, TILE.dirt);
put(obstacles, 42, 12, TILE.roofL); put(obstacles, 43, 12, TILE.roofM); put(obstacles, 44, 12, TILE.roofR);
put(obstacles, 42, 13, TILE.counter); put(obstacles, 43, 13, TILE.counter); put(obstacles, 44, 13, TILE.counter);
put(obstacles, 45, 13, TILE.crate); put(obstacles, 41, 12, TILE.sign);

// Fashion corner (top left).
fill(ground, 4, 3, 13, 10, TILE.wood);
fill(decor, 6, 7, 11, 9, (x, y) => ((x + y) % 2 ? TILE.rugRed : TILE.rugBlue));
put(obstacles, 7, 4, TILE.roofL); put(obstacles, 8, 4, TILE.roofM); put(obstacles, 9, 4, TILE.roofM); put(obstacles, 10, 4, TILE.roofR);
put(obstacles, 7, 5, TILE.counter); put(obstacles, 8, 5, TILE.counter); put(obstacles, 9, 5, TILE.counter); put(obstacles, 10, 5, TILE.counter);
fill(ground, 9, 11, 10, 14, TILE.dirt);

// Pond (bottom left) with a sand shore.
for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) {
  const d = ((x + 0.5 - 10) / 6.5) ** 2 + ((y + 0.5 - 24) / 4.2) ** 2;
  if (d <= 1.45) put(ground, x, y, TILE.sand);
}
for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) {
  const d = ((x + 0.5 - 10) / 5.2) ** 2 + ((y + 0.5 - 24) / 3.1) ** 2;
  if (d <= 1) put(ground, x, y, TILE.water);
}
for (let y = 1; y < H; y++) for (let x = 0; x < W; x++) if (get(ground, x, y) === TILE.water && get(ground, x, y - 1) === TILE.sand) put(ground, x, y, TILE.waterEdge);

// Gardens: flower beds and scattered bushes/rocks away from paths.
for (const [x, y] of [[30, 25], [31, 25], [36, 24], [37, 24], [14, 18], [41, 20]]) put(decor, x, y, TILE.flowerbed);
const free = (x, y) => get(obstacles, x, y) === -1 && [TILE.grass, TILE.grassDark, TILE.flowersY, TILE.flowersR].includes(get(ground, x, y));
for (const [x, y, id] of [[15, 6, TILE.bush], [16, 6, TILE.bush], [29, 6, TILE.rock], [5, 13, TILE.bush], [13, 13, TILE.rock], [35, 20, TILE.bush],
  [42, 22, TILE.rock], [44, 26, TILE.bush], [27, 26, TILE.bush], [19, 27, TILE.rock], [33, 28, TILE.bush], [5, 19, TILE.rock], [39, 17, TILE.bush],
  [12, 27, TILE.bush], [17, 25, TILE.bush], [46, 18, TILE.rock]]) if (free(x, y)) put(obstacles, x, y, id);

// Trees: a forest border plus a few inside. The canopy tile above each trunk
// is drawn over players.
const plantTree = (x, y) => { put(obstacles, x, y, TILE.tree); if (y > 0) put(above, x, y - 1, TILE.canopy); };
for (let x = 0; x < W; x++) { plantTree(x, 1); plantTree(x, H - 1); }
for (let y = 2; y < H - 1; y++) { plantTree(0, y); plantTree(W - 1, y); }
for (let x = 0; x < W; x++) { put(obstacles, x, 0, TILE.tree); }
for (const [x, y] of [[14, 9], [31, 8], [3, 15], [44, 16], [15, 29], [38, 27], [26, 4], [20, 5], [46, 9]]) if (free(x, y)) plantTree(x, y);

// Collision: anything blocking in obstacles or ground (water).
const collision = new Array(W * H).fill(0);
for (let i = 0; i < W * H; i++) if (BLOCKING.has(obstacles[i]) || BLOCKING.has(ground[i])) collision[i] = TILE.blocked + 1;

// ---------------------------------------------------------------------------
// Objects: NPCs (collider rect at their feet) and spawn points.
// ---------------------------------------------------------------------------
let nextObjectId = 1;
const npc = (code, tx, ty) => {
  const fx = tx * T + 16, fy = ty * T + 30; // feet
  return {
    id: nextObjectId++, name: code, type: 'npc', x: fx - 13, y: fy - 16, width: 26, height: 16, rotation: 0, visible: true,
    properties: [
      { name: 'npc_code', type: 'string', value: code },
      { name: 'interact_radius', type: 'int', value: 80 },
    ],
  };
};
const npcs = [npc('FARM_SHOP', 41, 14), npc('FASHION_SHOP', 8, 6)];
const spawns = [
  { id: nextObjectId++, name: 'default', type: 'spawn', point: true, x: 23.5 * T, y: 19 * T, width: 0, height: 0, rotation: 0, visible: true },
];

// Sanity: NPC colliders must not overlap blocked tiles or each other.
for (const o of npcs) {
  for (let y = Math.floor(o.y / T); y <= Math.floor((o.y + o.height - 1) / T); y++)
    for (let x = Math.floor(o.x / T); x <= Math.floor((o.x + o.width - 1) / T); x++)
      if (collision[y * W + x]) throw new Error(`${o.name} overlaps blocked tile ${x},${y}`);
}

const tiledLayer = (id, name, data, extra = {}) => ({
  id, name, type: 'tilelayer', width: W, height: H, x: 0, y: 0, opacity: 1, visible: true,
  data: data.map((v) => (v < 0 ? 0 : v + 1)), ...extra,
});
const map = {
  compressionlevel: -1, type: 'map', version: '1.10', tiledversion: '1.10.2',
  orientation: 'orthogonal', renderorder: 'right-down', infinite: false,
  width: W, height: H, tilewidth: T, tileheight: T,
  nextlayerid: 9, nextobjectid: nextObjectId,
  properties: [
    { name: 'map_id', type: 'string', value: MAP_ID },
    { name: 'map_version', type: 'string', value: MAP_VERSION },
    { name: 'display_name', type: 'string', value: 'Quảng trường (mẫu)' },
    { name: 'note', type: 'string', value: 'DEV SAMPLE map: layout, NPC positions and art are placeholders (GAP-13, GAP-19).' },
  ],
  tilesets: [{
    firstgid: 1, name: 'placeholder', image: 'tiles_placeholder.png', imagewidth: COLS * T, imageheight: ROWS * T,
    tilewidth: T, tileheight: T, tilecount: COLS * ROWS, columns: COLS, margin: 0, spacing: 0,
  }],
  layers: [
    tiledLayer(1, 'ground', ground),
    tiledLayer(2, 'decor', decor),
    tiledLayer(3, 'obstacles', obstacles),
    tiledLayer(4, 'above', above),
    tiledLayer(5, 'collision', collision.map((v) => v - 1), { visible: false, opacity: 0.5 }),
    { id: 6, name: 'npcs', type: 'objectgroup', draworder: 'topdown', x: 0, y: 0, opacity: 1, visible: true, objects: npcs },
    { id: 7, name: 'spawns', type: 'objectgroup', draworder: 'topdown', x: 0, y: 0, opacity: 1, visible: true, objects: spawns },
  ],
};

mkdirSync(OUT, { recursive: true });
writeFileSync(join(OUT, 'tiles_placeholder.png'), sheet.png());
writeFileSync(join(OUT, `${MAP_ID}.json`), JSON.stringify(map) + '\n');
writeFileSync(join(OUT, 'maps.json'), JSON.stringify({
  note: 'Map registry. Public maps are auto-split into rooms (P0-134). DEV SAMPLE until the MVP map list is decided (GAP-19).',
  default_map: MAP_ID,
  maps: [{ map_id: MAP_ID, file: `${MAP_ID}.json`, public: true }],
}, null, 2) + '\n');
console.log(`wrote ${MAP_ID}.json (${MAP_VERSION}), tiles_placeholder.png, maps.json to ${OUT}`);
