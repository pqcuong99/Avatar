import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkName } from './nameRules.ts';

// Combining marks are built from code points so no editor can normalize them.
const GRAVE = String.fromCodePoint(0x300);
const ACUTE = String.fromCodePoint(0x301);

test('same verdicts as the server rules', () => {
  const cases: [string, string | null][] = [
    ['Thành', null], ['Nguyễn Văn A', null], ['★Hoa♥', null], ['ab', 'too_short'],
    ['aaaaaaaaaaaaaaaaa', 'too_long'], ['ẫẫẫẫẫẫẫẫẫẫẫẫẫẫẫẫ', null], ['abc!', 'invalid_char'],
    [' abc', 'edge_space'], ['ab  c', 'repeated_space'], ['ñandu', 'invalid_char'],
    ['abc' + ACUTE + 'x', 'invalid_char'],
  ];
  for (const [name, want] of cases) assert.equal(checkName(name).problem, want, name);
});

test('decomposed input is normalized to NFC', () => {
  const r = checkName('Tha' + GRAVE + 'nh');
  assert.equal(r.problem, null);
  assert.equal(r.name, 'Thành');
  assert.equal(r.length, 5);
});
