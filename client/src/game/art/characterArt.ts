// Original procedural chibi characters (placeholder art, GAP-13): drawn with
// Canvas 2D so the same code feeds Phaser textures and React previews.
// Until fashion items exist, colours are derived from a seed (the character
// id) so every player looks different but stable.

export interface Palette {
  skin: string;
  hair: string;
  hairStyle: number; // 0 short, 1 long, 2 spiky, 3 bun
  shirt: string;
  shirtDark: string;
  pants: string;
  shoes: string;
  accessory?: 'apron';
  hat?: 'straw' | 'cap' | 'flower';
  hatColor?: string;
}

/** Equipped fashion as sent by the server (gamedb.Appearance). */
export interface Appearance {
  version: number;
  items: Record<string, { item_code: string; style: Record<string, string> }>;
}

/** Base look from the seed, with equipped fashion drawn over it. */
export function paletteWith(base: Palette, a?: Appearance | null): Palette {
  const p = { ...base };
  const top = a?.items?.top?.style;
  if (top) {
    p.shirt = top.shirt ?? p.shirt;
    p.shirtDark = top.shirt_dark ?? p.shirtDark;
  }
  const bottom = a?.items?.bottom?.style;
  if (bottom?.pants) p.pants = bottom.pants;
  const hat = a?.items?.hat?.style;
  if (hat?.hat === 'straw' || hat?.hat === 'cap' || hat?.hat === 'flower') {
    p.hat = hat.hat;
    p.hatColor = hat.color;
  }
  return p;
}

/** Short stable key of an appearance, for texture caching. */
export function appearanceKey(a?: Appearance | null): string {
  if (!a || !a.items || Object.keys(a.items).length === 0) return 'base';
  return Object.keys(a.items).sort().map((k) => `${k}=${a.items[k].item_code}`).join(',');
}

export type Facing = 'front' | 'back' | 'side';

const SKINS = ['#f6d5b8', '#eec29b', '#d9a77c', '#b98059', '#f3c7a5'];
// Hair colours are chosen to stay distinct from every skin tone above.
const HAIRS = ['#2b2118', '#4a2f1b', '#6b3f1f', '#f2c94c', '#1f2a44', '#7a2e2e', '#c96f9c', '#2f6b55'];
const SHIRTS: [string, string][] = [['#4dabf7', '#1c7ed6'], ['#ff8787', '#e03131'], ['#69db7c', '#2f9e44'], ['#ffd43b', '#f08c00'],
  ['#b197fc', '#7048e8'], ['#ffa94d', '#e8590c'], ['#63e6be', '#0ca678'], ['#f783ac', '#d6336c']];
const PANTS = ['#364fc7', '#495057', '#5c3d2e', '#2b8a3e', '#862e9c', '#1864ab'];

function hash(s: string): number {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

export function paletteFor(seed: string): Palette {
  let h = hash(seed);
  const pick = <T,>(arr: T[]) => { const v = arr[h % arr.length]; h = Math.floor(h / arr.length) ^ hash(String(h)); return v; };
  const [shirt, shirtDark] = pick(SHIRTS);
  return { skin: pick(SKINS), hair: pick(HAIRS), hairStyle: pick([0, 1, 2, 3]), shirt, shirtDark, pants: pick(PANTS), shoes: '#3b2f2f' };
}

export const NPC_PALETTES: Record<string, Palette> = {
  FARM_SHOP: { skin: '#e8b58e', hair: '#ced4da', hairStyle: 0, shirt: '#8fbc5a', shirtDark: '#5c8a33', pants: '#7c5a3a', shoes: '#4a3524', hat: 'straw' },
  FASHION_SHOP: { skin: '#f6d5b8', hair: '#7a2e2e', hairStyle: 3, shirt: '#f783ac', shirtDark: '#c2255c', pants: '#5f3dc4', shoes: '#2b2118', accessory: 'apron' },
};

export const ART_W = 32;
export const ART_H = 48;
/** Feet position inside the 32x48 art box. */
export const FEET_X = 16;
export const FEET_Y = 46;

function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}

/**
 * Draws one frame into ctx, in a 32x48 box (scale ctx beforehand for more
 * resolution). frame: -1 idle, 0..3 walk cycle.
 */
export function drawCharacter(ctx: CanvasRenderingContext2D, p: Palette, facing: Facing, frame: number) {
  const walking = frame >= 0;
  const swing = walking ? [-2.2, 0, 2.2, 0][frame] : 0;
  const bob = walking && frame % 2 === 1 ? -1 : 0;
  ctx.save();
  ctx.lineJoin = 'round';
  ctx.lineWidth = 1;
  const outline = 'rgba(40,30,30,0.55)';

  // Legs and shoes
  const legY = 36 + bob;
  const legs = facing === 'side' ? [[13.5, swing], [16.5, -swing]] : [[11.5, swing * 0.6], [17, -swing * 0.6]];
  for (const [lx, s] of legs) {
    ctx.fillStyle = p.pants;
    roundRect(ctx, lx + (facing === 'side' ? s : 0), legY, 3.6, 8 - Math.abs(facing === 'side' ? 0 : s) * 0.4, 1.2);
    ctx.fill();
    ctx.fillStyle = p.shoes;
    roundRect(ctx, lx - 0.4 + (facing === 'side' ? s + 0.6 : 0), 43.5, facing === 'side' ? 5 : 4.4, 2.6, 1.2);
    ctx.fill();
  }

  // Torso
  ctx.fillStyle = p.shirt;
  ctx.strokeStyle = outline;
  roundRect(ctx, 9.5, 24 + bob, 13, 13.5, 4);
  ctx.fill();
  ctx.stroke();
  ctx.fillStyle = p.shirtDark;
  ctx.fillRect(9.8, 34 + bob, 12.4, 3.2);
  if (p.accessory === 'apron' && facing !== 'back') {
    ctx.fillStyle = '#fff4e6';
    roundRect(ctx, 11.5, 27 + bob, 9, 10, 2);
    ctx.fill();
    ctx.fillStyle = '#f783ac';
    ctx.fillRect(13.5, 30 + bob, 5, 1.2);
  }
  if (facing === 'front') {
    ctx.fillStyle = 'rgba(255,255,255,0.65)';
    ctx.beginPath();
    ctx.moveTo(13, 24.5 + bob);
    ctx.lineTo(16, 27.5 + bob);
    ctx.lineTo(19, 24.5 + bob);
    ctx.fill();
  }

  // Arms
  const armSwing = walking ? -swing * 0.8 : 0;
  ctx.fillStyle = p.shirt;
  ctx.strokeStyle = outline;
  if (facing === 'side') {
    roundRect(ctx, 14.5 + armSwing, 25.5 + bob, 3.6, 9, 1.8);
    ctx.fill();
    ctx.stroke();
    ctx.fillStyle = p.skin;
    ctx.beginPath();
    ctx.arc(16.3 + armSwing, 34.8 + bob, 1.9, 0, Math.PI * 2);
    ctx.fill();
  } else {
    for (const [ax, s] of [[6.8, armSwing], [21.6, -armSwing]] as const) {
      ctx.fillStyle = p.shirt;
      roundRect(ctx, ax, 25 + bob + s * 0.3, 3.6, 9, 1.8);
      ctx.fill();
      ctx.stroke();
      ctx.fillStyle = p.skin;
      ctx.beginPath();
      ctx.arc(ax + 1.8, 34.3 + bob + s * 0.3, 1.9, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  // Head
  const hy = 14 + bob;
  ctx.fillStyle = p.skin;
  ctx.strokeStyle = outline;
  ctx.beginPath();
  ctx.ellipse(16, hy, 10.5, 10, 0, 0, Math.PI * 2);
  ctx.fill();
  ctx.stroke();

  // Hair
  ctx.fillStyle = p.hair;
  if (facing === 'back') {
    ctx.beginPath();
    ctx.ellipse(16, hy - 0.5, 10.8, 10.3, 0, 0, Math.PI * 2);
    ctx.fill();
    if (p.hairStyle === 1) { roundRect(ctx, 7, hy, 18, 12, 4); ctx.fill(); }
  } else if (facing === 'side') {
    ctx.beginPath();
    ctx.ellipse(14.5, hy - 2.5, 10.4, 8.2, -0.2, Math.PI * 0.82, Math.PI * 2.05);
    ctx.fill();
    ctx.beginPath();
    ctx.ellipse(10.5, hy + 1, 5.5, 7.5, 0, 0, Math.PI * 2);
    ctx.fill();
    if (p.hairStyle === 1) { roundRect(ctx, 6.5, hy, 7, 12, 3); ctx.fill(); }
  } else {
    ctx.beginPath();
    ctx.ellipse(16, hy - 3.2, 10.8, 7.4, 0, Math.PI, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(5.5, hy - 2);
    ctx.quadraticCurveTo(10, hy + 1.5, 13, hy - 3);
    ctx.quadraticCurveTo(16, hy + 1, 19, hy - 3);
    ctx.quadraticCurveTo(22, hy + 1.5, 26.5, hy - 2);
    ctx.lineTo(26.5, hy - 5);
    ctx.lineTo(5.5, hy - 5);
    ctx.fill();
    if (p.hairStyle === 1) {
      roundRect(ctx, 4.8, hy - 3, 3.6, 13, 1.8); ctx.fill();
      roundRect(ctx, 23.6, hy - 3, 3.6, 13, 1.8); ctx.fill();
    }
  }
  if (p.hairStyle === 2 && facing !== 'back') {
    for (const sx of [9, 13, 17, 21]) {
      ctx.beginPath();
      ctx.moveTo(sx - 2.5, hy - 8);
      ctx.lineTo(sx, hy - 13);
      ctx.lineTo(sx + 2.5, hy - 8);
      ctx.fill();
    }
  }
  if (p.hairStyle === 3) {
    ctx.beginPath();
    ctx.arc(facing === 'side' ? 9 : 16, hy - 10.5, 4, 0, Math.PI * 2);
    ctx.fill();
  }

  // Face
  if (facing !== 'back') {
    const eyes = facing === 'front' ? [12, 20] : [20.5];
    for (const ex of eyes) {
      ctx.fillStyle = '#2b2d42';
      ctx.beginPath();
      ctx.ellipse(ex, hy + 2, 1.5, 2.1, 0, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = '#fff';
      ctx.beginPath();
      ctx.arc(ex + 0.5, hy + 1.2, 0.6, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.fillStyle = 'rgba(255,120,140,0.45)';
    for (const bx of facing === 'front' ? [9.5, 22.5] : [22.5]) {
      ctx.beginPath();
      ctx.ellipse(bx, hy + 5, 1.8, 1.1, 0, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.strokeStyle = '#8a4b3a';
    ctx.lineWidth = 0.9;
    ctx.beginPath();
    if (facing === 'front') ctx.arc(16, hy + 5.2, 1.6, 0.15 * Math.PI, 0.85 * Math.PI);
    else ctx.arc(22, hy + 5.4, 1.3, 0.2 * Math.PI, 0.8 * Math.PI);
    ctx.stroke();
  }

  drawHat(ctx, p, facing, hy);
  ctx.restore();
}

function drawHat(ctx: CanvasRenderingContext2D, p: Palette, facing: Facing, hy: number) {
  ctx.lineWidth = 1;
  if (p.hat === 'straw') {
    ctx.fillStyle = '#f2d16b';
    ctx.strokeStyle = '#b8902d';
    ctx.beginPath();
    ctx.ellipse(16, hy - 6, 15, 4.2, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.beginPath();
    ctx.ellipse(16, hy - 9, 8, 5.5, 0, Math.PI, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.fillStyle = '#c0392b';
    ctx.fillRect(8.2, hy - 9.2, 15.6, 1.8);
  } else if (p.hat === 'cap') {
    const c = p.hatColor ?? '#1c7ed6';
    ctx.fillStyle = c;
    ctx.strokeStyle = 'rgba(30,30,40,0.5)';
    ctx.beginPath();
    ctx.ellipse(16, hy - 5, 10.6, 7.5, 0, Math.PI, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.fillStyle = shade(c);
    ctx.beginPath();
    if (facing === 'front') ctx.ellipse(16, hy - 4.6, 9.5, 2.6, 0, 0, Math.PI);
    else if (facing === 'side') ctx.ellipse(25, hy - 5, 6, 2, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = '#ffffff';
    ctx.beginPath();
    ctx.arc(16, hy - 12.3, 1.3, 0, Math.PI * 2);
    ctx.fill();
  } else if (p.hat === 'flower') {
    const colors = [p.hatColor ?? '#f783ac', '#ffd43b', '#ffffff', '#b197fc'];
    for (let i = 0; i < 7; i++) {
      const a = Math.PI + (i / 6) * Math.PI;
      const x = 16 + Math.cos(a) * 10, y = hy - 3 + Math.sin(a) * 8;
      if (facing === 'back' && (i === 0 || i === 6)) continue;
      ctx.fillStyle = colors[i % colors.length];
      ctx.beginPath();
      ctx.arc(x, y, 2.4, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = '#f08c00';
      ctx.beginPath();
      ctx.arc(x, y, 0.8, 0, Math.PI * 2);
      ctx.fill();
    }
  }
}

function shade(hex: string): string {
  const n = parseInt(hex.slice(1), 16);
  const r = Math.max(0, (n >> 16) - 40), g = Math.max(0, ((n >> 8) & 255) - 40), b = Math.max(0, (n & 255) - 40);
  return `rgb(${r},${g},${b})`;
}

export function facingOf(dir: number): { facing: Facing; flip: boolean } {
  switch (dir) {
    case 4: return { facing: 'back', flip: false };
    case 3: return { facing: 'back', flip: true };
    case 5: return { facing: 'back', flip: false };
    case 2: return { facing: 'side', flip: true };
    case 6: return { facing: 'side', flip: false };
    case 1: return { facing: 'front', flip: true };
    default: return { facing: 'front', flip: false };
  }
}
