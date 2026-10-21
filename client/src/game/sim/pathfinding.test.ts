import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { loadMap, step, collides, type TiledMap } from './movement.ts';
import { findPath, clearLine } from './pathfinding.ts';
import { PathFollower } from './follow.ts';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', '..');
const map = loadMap(JSON.parse(readFileSync(join(root, 'shared', 'maps', 'plaza.json'), 'utf8')) as TiledMap);
const T = map.world.tileSize;

test('a path around the fountain avoids walls and NPCs', () => {
  const from: [number, number] = [23 * T + T / 2, 19 * T + T / 2]; // south of the fountain
  const to: [number, number] = [23 * T + T / 2, 12 * T + T / 2]; // north of it
  const path = findPath(map.world, from, to);
  assert.ok(path && path.length >= 2, 'needs to go around');
  let prev = from;
  for (const p of path) {
    assert.ok(clearLine(map.world, prev, p), `segment to ${p} is blocked`);
    prev = p;
  }
  assert.deepEqual(path[path.length - 1], to);
});

test('clicking inside a wall walks to the nearest free tile', () => {
  const from: [number, number] = [map.spawn[0], map.spawn[1]];
  const fountain: [number, number] = [23 * T + T / 2, 15 * T + T / 2];
  const path = findPath(map.world, from, fountain);
  assert.ok(path);
  const end = path[path.length - 1];
  assert.equal(collides(map.world, end[0], end[1]), false);
});

test('following a path with server-identical steps reaches the target', () => {
  const farm = map.npcs.find((n) => n.code === 'FARM_SHOP')!;
  const target: [number, number] = [farm.x, farm.y + 2600];
  let pos: [number, number] = [map.spawn[0], map.spawn[1]];
  const path = findPath(map.world, pos, target);
  assert.ok(path);
  const f = new PathFollower(path, 800);
  for (let i = 0; i < 2000 && !f.done; i++) {
    const d = f.next(pos);
    if (!d) break;
    pos = step(map.world, pos[0], pos[1], d[0], d[1], 800);
  }
  assert.ok(f.done, 'path not finished');
  assert.ok(Math.hypot(pos[0] - target[0], pos[1] - target[1]) <= 100, `ended at ${pos}, target ${target}`);
});
