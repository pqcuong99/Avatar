import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { clampDir, collides, loadMap, step, type TiledMap } from './movement.ts';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', '..');
const readJSON = (...p: string[]) => JSON.parse(readFileSync(join(root, ...p), 'utf8'));

interface Golden {
  map: string;
  map_version: string;
  speed: number;
  clamp: [number, number, number, number][];
  walks: { start: [number, number]; inputs: [number, number][]; positions: [number, number][] }[];
}

const golden = readJSON('shared', 'fixtures', 'movement_cases.json') as Golden;
const map = loadMap(readJSON('shared', 'maps', golden.map) as TiledMap);

test('fixture matches the map version', () => {
  assert.equal(map.version, golden.map_version);
});

test('clampDir matches the Go server', () => {
  for (const [dx, dy, cx, cy] of golden.clamp) {
    assert.deepEqual(clampDir(dx, dy), [cx, cy], `clamp(${dx},${dy})`);
  }
});

test('prediction replays every golden walk exactly like the server', () => {
  golden.walks.forEach((w, i) => {
    let [x, y] = w.start;
    w.inputs.forEach(([dx, dy], j) => {
      [x, y] = step(map.world, x, y, dx, dy, golden.speed);
      assert.deepEqual([x, y], w.positions[j], `walk ${i} step ${j}`);
    });
  });
});

test('spawn and NPC colliders', () => {
  assert.equal(collides(map.world, map.spawn[0], map.spawn[1]), false);
  const farm = map.npcs.find((n) => n.code === 'FARM_SHOP');
  assert.ok(farm);
  assert.equal(collides(map.world, farm.x, farm.y - 500), true, 'cannot stand inside an NPC');
});
