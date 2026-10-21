import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { decodeMove, decodeSnapshot, encodeMove, encodeSnapshot, type EntityState } from './proto.ts';

const golden = JSON.parse(readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', '..', 'shared', 'fixtures', 'proto_frames.json'), 'utf8')) as {
  client: { seq: number; dx: number; dy: number; hex: string }[];
  server: { server_tick: number; ack_seq: number; entities: EntityState[] | null; hex: string }[];
};

const hex = (b: Uint8Array) => Buffer.from(b).toString('hex');
const unhex = (h: string) => new Uint8Array(Buffer.from(h, 'hex'));

test('client frames encode exactly like the Go codec', () => {
  for (const c of golden.client) {
    assert.equal(hex(encodeMove(c)), c.hex);
    assert.deepEqual(decodeMove(unhex(c.hex)), { seq: c.seq, dx: c.dx, dy: c.dy });
  }
});

test('server snapshots decode and re-encode', () => {
  for (const s of golden.server) {
    const snap = decodeSnapshot(unhex(s.hex));
    assert.ok(snap);
    assert.equal(snap.server_tick, s.server_tick);
    assert.equal(snap.ack_seq, s.ack_seq);
    assert.deepEqual(snap.entities, s.entities ?? []);
    assert.equal(hex(encodeSnapshot(snap)), s.hex);
  }
});

test('garbage is rejected, unknown fields are skipped', () => {
  assert.throws(() => decodeSnapshot(new Uint8Array([0x0a, 0xff])));
  // field 9 (varint) before the snapshot must be skipped
  const withUnknown = new Uint8Array([0x48, 0x05, ...unhex(golden.server[1].hex)]);
  assert.equal(decodeSnapshot(withUnknown)?.server_tick, golden.server[1].server_tick);
});
