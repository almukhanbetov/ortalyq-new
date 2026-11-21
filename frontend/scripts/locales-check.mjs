// Fails when ru/kk/en locale files do not have the same set of keys (CLAUDE.md: the key sets must match).
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const dir = fileURLToPath(new URL('../src/locales/', import.meta.url));
const flat = (o, p = '') => Object.entries(o).flatMap(([k, v]) => (v && typeof v === 'object' ? flat(v, `${p}${k}.`) : [p + k]));
const keys = Object.fromEntries(['ru', 'kk', 'en'].map((l) => [l, new Set(flat(JSON.parse(readFileSync(`${dir}${l}.json`, 'utf8'))))]));
let bad = false;
for (const l of ['kk', 'en']) {
  for (const [a, b] of [['ru', l], [l, 'ru']]) {
    const missing = [...keys[a]].filter((k) => !keys[b].has(k));
    if (missing.length) {
      bad = true;
      console.error(`${b}.json is missing ${missing.length} keys of ${a}.json:`, missing.slice(0, 20).join(', '));
    }
  }
}
console.log(`locale keys: ru ${keys.ru.size}, kk ${keys.kk.size}, en ${keys.en.size}${bad ? ' — MISMATCH' : ' — ok'}`);
process.exit(bad ? 1 : 0);
