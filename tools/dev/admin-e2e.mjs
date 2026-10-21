// LOCAL DEVELOPMENT ONLY: end-to-end check of the Admin API through the web
// origin (Vite proxy). Reads the bootstrap output written to
// deploy/local-secrets/admin-<login>.json and computes TOTP codes itself.
//
//   node tools/dev/admin-e2e.mjs <player character name to compensate>
import { readFileSync } from 'node:fs';
import { createHmac } from 'node:crypto';

const BASE = process.env.BASE ?? 'http://localhost:5173';
const target = process.argv[2] ?? 'Thành ★';

function base32(s) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const c of s.replace(/=+$/, '')) bits += alphabet.indexOf(c).toString(2).padStart(5, '0');
  const out = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) out.push(parseInt(bits.slice(i, i + 8), 2));
  return Buffer.from(out);
}
function totp(secret, t = Date.now()) {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(t / 30000)));
  const h = createHmac('sha1', base32(secret)).update(counter).digest();
  const o = h[h.length - 1] & 15;
  return String((h.readUInt32BE(o) & 0x7fffffff) % 1_000_000).padStart(6, '0');
}

class Admin {
  constructor(login) {
    this.cred = JSON.parse(readFileSync(new URL(`../../deploy/local-secrets/admin-${login}.json`, import.meta.url), 'utf8'));
    this.cookie = '';
  }
  async call(method, path, body) {
    const res = await fetch(BASE + '/admin/api/v1' + path, {
      method, headers: { 'Content-Type': 'application/json', Origin: BASE, Cookie: this.cookie },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const set = res.headers.get('set-cookie');
    if (set) this.cookie = set.split(';')[0];
    return { status: res.status, json: await res.json() };
  }
  async login() {
    const a = await this.call('POST', '/login', { login: this.cred.login, password: this.cred.password });
    if (!a.json.ok) throw new Error('password step: ' + JSON.stringify(a.json));
    const b = await this.call('POST', '/login/totp', { challenge: a.json.result.challenge, code: totp(this.cred.totp_secret) });
    if (!b.json.ok) throw new Error('totp step: ' + JSON.stringify(b.json));
  }
}

const check = (cond, msg) => { console.log((cond ? 'PASS ' : 'FAIL ') + msg); if (!cond) process.exitCode = 1; };

const lead = new Admin('ops_lead');
const ops = new Admin('game_ops');
const noTotp = await lead.call('POST', '/login', { login: lead.cred.login, password: lead.cred.password });
const me0 = await new Admin('ops_lead').call('GET', '/me');
check(noTotp.json.ok && me0.status === 401, 'A-11 password alone gives no Admin session');
await lead.login();
await ops.login();
const me = await lead.call('GET', '/me');
check(me.json.result.permissions['admin.manage'] === true, 'Super Admin permissions loaded');

const players = await lead.call('GET', '/players?q=' + encodeURIComponent(target));
const player = players.json.result.players[0];
check(!!player, `found player ${target}`);
const before = await lead.call('GET', '/players/' + player.character_id);
const xuBefore = before.json.result.state.wallets.XU.balance;

const prop = await lead.call('POST', '/adjustments', { character_id: player.character_id, asset_kind: 'XU', amount_delta: 777, reason: 'E2E: đền bù thử nghiệm' });
check(prop.json.ok && prop.json.result.state === 'pending', 'proposal created (pending)');
const self = await lead.call('POST', `/adjustments/${prop.json.result.id}/approve`, { note: 'tự duyệt' });
check(self.status === 403 && self.json.error.code === 'SELF_APPROVAL_FORBIDDEN', 'T-09 Super Admin cannot approve own request');
const ok = await ops.call('POST', `/adjustments/${prop.json.result.id}/approve`, { note: 'đã kiểm tra' });
check(ok.json.ok && ok.json.result.state === 'executed', 'T-10 other Admin approves -> executed');
const again = await ops.call('POST', `/adjustments/${prop.json.result.id}/approve`, { note: 'lần 2' });
check(again.status === 409, 'second approval refused (executed once)');
const after = await lead.call('GET', '/players/' + player.character_id);
check(after.json.result.state.wallets.XU.balance === xuBefore + 777, `wallet ${xuBefore} -> ${after.json.result.state.wallets.XU.balance}`);
check(after.json.result.ledger[0].reason_code === 'ADMIN_ADJUSTMENT', 'ledger row ADMIN_ADJUSTMENT');

// O-06: publish only in maintenance.
const versions = (await ops.call('GET', '/config/versions')).json.result.versions;
const live = versions.find((v) => v.state === 'published');
const content = (await ops.call('GET', '/config/versions/' + live.id)).json.result;
const draft = await ops.call('POST', '/config/drafts', { id: 0, label: `E2E-${Date.now()}`, content });
check(draft.json.ok, 'draft created from the published version');
const early = await ops.call('POST', `/config/versions/${draft.json.result.id}/publish`);
check(early.status === 409 && early.json.error.code === 'MAINTENANCE_REQUIRED', 'O-06 publish refused outside maintenance');
await ops.call('POST', '/maintenance', { active: true, message: 'E2E kiểm thử phát hành' });
const pub = await ops.call('POST', `/config/versions/${draft.json.result.id}/publish`);
check(pub.json.ok, 'published during maintenance');
await ops.call('POST', '/maintenance', { active: false, message: '' });
const audit = (await lead.call('GET', '/audit')).json.result.entries.map((e) => e.action);
check(audit.includes('config.publish') && audit.includes('asset_adjustment.executed') && audit.includes('maintenance.set'), 'audit has publish/execute/maintenance');
