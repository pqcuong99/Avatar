// Client-side preview of the character name rules (P0-020..025). The server
// re-validates everything (transaction-service/internal/names); this only
// gives instant feedback and must stay in sync with it.

export const MIN_GRAPHEMES = 3;
export const MAX_GRAPHEMES = 16;

const VI_LETTERS = 'aàáảãạăằắẳẵặâầấẩẫậbcdđeèéẻẽẹêềếểễệfghiìíỉĩịjklmnoòóỏõọôồốổỗộơờớởỡợpqrstuùúủũụưừứửữựvwxyỳýỷỹỵz';
const LETTERS = new Set([...VI_LETTERS, ...VI_LETTERS.toUpperCase()]);
const SYMBOLS = new Set([' ', '_', '-', '.', '★', '♥', '☆', '♡']);

export type NameProblem = 'too_short' | 'too_long' | 'invalid_char' | 'edge_space' | 'repeated_space';

export const PROBLEM_TEXT: Record<NameProblem, string> = {
  too_short: `Tên cần ít nhất ${MIN_GRAPHEMES} ký tự.`,
  too_long: `Tên tối đa ${MAX_GRAPHEMES} ký tự.`,
  invalid_char: 'Chỉ dùng chữ tiếng Việt, số, dấu cách và _ - . ★ ♥ ☆ ♡',
  edge_space: 'Không để dấu cách ở đầu hoặc cuối tên.',
  repeated_space: 'Không dùng hai dấu cách liền nhau.',
};

export function graphemeCount(s: string): number {
  const Seg = (Intl as unknown as { Segmenter?: new (l: string, o: { granularity: string }) => { segment(s: string): Iterable<unknown> } }).Segmenter;
  if (Seg) return [...new Seg('vi', { granularity: 'grapheme' }).segment(s)].length;
  return [...s].length;
}

export function checkName(raw: string): { name: string; problem: NameProblem | null; length: number } {
  const name = raw.normalize('NFC');
  const length = graphemeCount(name);
  for (const ch of name) {
    if (!LETTERS.has(ch) && !SYMBOLS.has(ch) && !(ch >= '0' && ch <= '9')) return { name, problem: 'invalid_char', length };
  }
  if (name.startsWith(' ') || name.endsWith(' ')) return { name, problem: 'edge_space', length };
  if (name.includes('  ')) return { name, problem: 'repeated_space', length };
  if (length < MIN_GRAPHEMES) return { name, problem: 'too_short', length };
  if (length > MAX_GRAPHEMES) return { name, problem: 'too_long', length };
  return { name, problem: null, length };
}
