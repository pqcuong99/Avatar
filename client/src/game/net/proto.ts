// Minimal Protocol Buffers wire codec for shared/proto/realtime.proto.
// Golden encodings in shared/fixtures/proto_frames.json are checked by both
// this module's tests and the Go server's, so the codecs cannot drift.

export interface MoveInput { seq: number; dx: number; dy: number }
export interface EntityState { entity_id: number; x: number; y: number; dir: number; moving: boolean }
export interface Snapshot { server_tick: number; ack_seq: number; entities: EntityState[] }

class Writer {
  private bytes: number[] = [];
  varint(v: number) {
    // v is a non-negative integer below 2^53.
    while (v >= 0x80) {
      this.bytes.push((v % 0x80) | 0x80);
      v = Math.floor(v / 0x80);
    }
    this.bytes.push(v);
  }
  tag(field: number, wire: number) { this.varint(field * 8 + wire); }
  uint(field: number, v: number) { if (v !== 0) { this.tag(field, 0); this.varint(v); } }
  sint(field: number, v: number) { if (v !== 0) { this.tag(field, 0); this.varint(v >= 0 ? v * 2 : -v * 2 - 1); } }
  message(field: number, inner: Writer) { this.tag(field, 2); this.varint(inner.bytes.length); this.bytes.push(...inner.bytes); }
  finish() { return new Uint8Array(this.bytes); }
}

class Reader {
  pos = 0;
  private buf: Uint8Array;
  private end: number;
  constructor(buf: Uint8Array, end = buf.length) {
    this.buf = buf;
    this.end = end;
  }
  get done() { return this.pos >= this.end; }
  varint(): number {
    let result = 0, mul = 1;
    for (let i = 0; i < 10; i++) {
      if (this.pos >= this.end) throw new Error('proto: truncated varint');
      const b = this.buf[this.pos++];
      result += (b & 0x7f) * mul;
      if (b < 0x80) return result;
      mul *= 0x80;
    }
    throw new Error('proto: varint too long');
  }
  sub(): Reader {
    const len = this.varint();
    if (this.pos + len > this.end) throw new Error('proto: truncated message');
    const r = new Reader(this.buf, this.pos + len);
    r.pos = this.pos;
    this.pos += len;
    return r;
  }
  skip(wire: number) {
    if (wire === 0) this.varint();
    else if (wire === 1) this.pos += 8;
    else if (wire === 2) this.pos += this.varint();
    else if (wire === 5) this.pos += 4;
    else throw new Error(`proto: unsupported wire type ${wire}`);
    if (this.pos > this.end) throw new Error('proto: truncated field');
  }
}

const zigzag = (n: number) => (n % 2 === 0 ? n / 2 : -(n + 1) / 2);

export function encodeMove(m: MoveInput): Uint8Array {
  const inner = new Writer();
  inner.uint(1, m.seq);
  inner.sint(2, m.dx);
  inner.sint(3, m.dy);
  const w = new Writer();
  w.message(1, inner);
  return w.finish();
}

export function decodeSnapshot(buf: Uint8Array): Snapshot | null {
  const r = new Reader(buf);
  let snap: Snapshot | null = null;
  while (!r.done) {
    const key = r.varint();
    const field = Math.floor(key / 8), wire = key % 8;
    if (field === 1 && wire === 2) {
      snap = { server_tick: 0, ack_seq: 0, entities: [] };
      const s = r.sub();
      while (!s.done) {
        const k = s.varint();
        const f = Math.floor(k / 8), wt = k % 8;
        if (f === 1 && wt === 0) snap.server_tick = s.varint();
        else if (f === 2 && wt === 0) snap.ack_seq = s.varint();
        else if (f === 3 && wt === 2) {
          const e: EntityState = { entity_id: 0, x: 0, y: 0, dir: 0, moving: false };
          const er = s.sub();
          while (!er.done) {
            const ek = er.varint();
            const ef = Math.floor(ek / 8), ewt = ek % 8;
            if (ewt !== 0) { er.skip(ewt); continue; }
            const v = er.varint();
            if (ef === 1) e.entity_id = v;
            else if (ef === 2) e.x = zigzag(v);
            else if (ef === 3) e.y = zigzag(v);
            else if (ef === 4) e.dir = v;
            else if (ef === 5) e.moving = v !== 0;
          }
          snap.entities.push(e);
        } else s.skip(wt);
      }
    } else r.skip(wire);
  }
  return snap;
}

// Test helpers (also handy for bots): the server-side counterparts.
export function decodeMove(buf: Uint8Array): MoveInput | null {
  const r = new Reader(buf);
  let m: MoveInput | null = null;
  while (!r.done) {
    const key = r.varint();
    if (key === (1 << 3 | 2)) {
      m = { seq: 0, dx: 0, dy: 0 };
      const s = r.sub();
      while (!s.done) {
        const k = s.varint();
        const v = s.varint();
        if (k >> 3 === 1) m.seq = v;
        else if (k >> 3 === 2) m.dx = zigzag(v);
        else if (k >> 3 === 3) m.dy = zigzag(v);
      }
    } else r.skip(key % 8);
  }
  return m;
}

export function encodeSnapshot(s: Snapshot): Uint8Array {
  const inner = new Writer();
  inner.uint(1, s.server_tick);
  inner.uint(2, s.ack_seq);
  for (const e of s.entities) {
    const ew = new Writer();
    ew.uint(1, e.entity_id);
    ew.sint(2, e.x);
    ew.sint(3, e.y);
    ew.uint(4, e.dir);
    if (e.moving) ew.uint(5, 1);
    inner.message(3, ew);
  }
  const w = new Writer();
  w.message(1, inner);
  return w.finish();
}
