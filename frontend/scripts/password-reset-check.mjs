// Browser check of the password reset: «Забыли пароль?» → e-mail → "check your mail" (the same for an unknown
// address, which gets no letter) → change email / resend → newest link → new password → sign-in screen. Then: the
// new password works, the old one is refused, the old session is closed, the account keeps its id and role, used,
// superseded and expired links are refused, /admin still opens.
// Needs running API + web, seeded demo data, the local SMTP receiver (cmd/mailsink) as the API's MAIL_SMTP_ADDR and
// a throwaway test database (the expired link is made by moving its expiry in that database through PG_CONTAINER).
// It changes the teacher's password: run it on a test stand only. Links, tokens and passwords are never printed.
//   MAIL_DIR=… PG_CONTAINER=… PG_USER=… PG_DB=… node scripts/password-reset-check.mjs
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const WEB = process.env.WEB_URL || 'http://localhost:3448';
const API = process.env.API_URL || 'http://localhost:8448';
const MAIL_DIR = process.env.MAIL_DIR || '../backend/data/mailsink';
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/password-reset';
const PG = { container: process.env.PG_CONTAINER, user: process.env.PG_USER, db: process.env.PG_DB };
if (!PG.container || !PG.user || !PG.db) throw new Error('set PG_CONTAINER, PG_USER and PG_DB of the throwaway test database');
// Demo passwords come from the environment or from the local seed settings; none is written in this script.
const seedEnv = Object.fromEntries(readFileSync(new URL('../../backend/.env.example', import.meta.url), 'utf8')
  .split('\n').map((l) => l.match(/^(SEED_[A-Z]+_PASSWORD)=(.*)$/)).filter(Boolean).map((m) => [m[1], m[2].trim()]));
const PW = {
  teacher: process.env.TEACHER_PASSWORD || seedEnv.SEED_TEACHER_PASSWORD,
  admin: process.env.ADMIN_PASSWORD || seedEnv.SEED_ADMIN_PASSWORD,
};
if (!PW.teacher || !PW.admin) throw new Error('set TEACHER_PASSWORD and ADMIN_PASSWORD');
const TEACHER = 'teacher@ortalyq.local';
const OTHER = 'student@ortalyq.local';
const newPassword = `Reset-${Math.random().toString(36).slice(2)}-${Date.now()}`;
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: true });
const ctxOpts = { viewport: { width: 1280, height: 860 }, locale: 'ru-RU', timezoneId: 'Asia/Almaty' };
const errors = [];
const watch = (p) => {
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => m.type() === 'error' && !/Failed to load resource/.test(m.text()) && errors.push(`console: ${m.text()}`));
  return p;
};
const page = async (opts = {}) => watch(await (await browser.newContext({ ...ctxOpts, ...opts })).newPage());
let step = 0;
const shot = async (p, name) => { await p.waitForTimeout(300); await p.screenshot({ path: `${OUT}/${String(++step).padStart(2, '0')}-${name}.png` }); };
const log = (s) => console.log(`✓ ${s}`);
const visible = async (loc, what, timeout = 15000) => {
  await loc.first().waitFor({ state: 'visible', timeout }).catch(() => { throw new Error(`not visible: ${what}`); });
};
const letters = (to) => readdirSync(MAIL_DIR).filter((f) => f.endsWith(`-${to.replace('@', '_at_')}.txt`)).sort().map((f) => readFileSync(join(MAIL_DIR, f), 'utf8'));
// Letters sent before this run (e.g. by the sign-up check) are not counted: n is the n-th letter of this run.
const baseline = {};
const waitLetter = async (to, n) => {
  baseline[to] ??= 0;
  for (let i = 0; i < 60; i++) {
    const l = letters(to).slice(baseline[to]);
    if (l.length >= n) return l[n - 1];
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`no letter #${n} to ${to}`);
};
const linkRe = /https?:\/\/\S+\/reset-password#[A-Za-z0-9_-]{43}/;
const linkOf = (letter) => { const l = letter.match(linkRe)?.[0]; if (!l) throw new Error('no reset link in the letter'); return l; };
const sql = (q) => execFileSync('docker', ['exec', PG.container, 'psql', '-U', PG.user, '-d', PG.db, '-tAc', q], { encoding: 'utf8' }).trim();
const me = (p) => p.evaluate(async (api) => { const r = await fetch(`${api}/api/auth/me`, { credentials: 'include' }); return r.ok ? r.json() : r.status; }, API);
const login = async (p, email, password) => {
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль', { exact: true }).fill(password);
  await p.getByRole('button', { name: 'Войти' }).click();
};

for (const to of [TEACHER, OTHER]) baseline[to] = letters(to).length;

try {
  // 0. Screens on a wide and a phone screen.
  for (const [vp, size] of [['wide', { width: 1440, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
    const P = await page({ viewport: size, ...(vp === 'mobile' ? { isMobile: true, hasTouch: true, deviceScaleFactor: 2 } : {}) });
    await P.goto(`${WEB}/forgot-password`);
    await visible(P.getByRole('button', { name: 'Отправить ссылку' }), 'forgot form');
    if (await P.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)) throw new Error(`horizontal scroll (${vp})`);
    await P.screenshot({ path: `${OUT}/forgot-${vp}.png`, fullPage: true });
    await P.context().close();
  }

  // 1. An open session of the teacher, to be closed by the reset.
  const S = await page();
  await login(S, TEACHER, PW.teacher);
  await S.waitForURL('**/cabinet/school/classes**');
  const before = await me(S);
  if (!before?.id || before.role !== 'teacher') throw new Error('teacher session not open');
  log('teacher signed in (old session kept open)');

  // 2. «Забыли пароль?» on /login; address validation.
  const A = await page();
  await A.goto(`${WEB}/login`);
  await A.getByRole('link', { name: 'Забыли пароль?' }).click();
  await A.waitForURL('**/forgot-password');
  await visible(A.getByRole('heading', { name: 'Восстановление пароля' }), 'forgot title');
  await shot(A, 'forgot');
  await A.getByLabel('Email').fill('not-an-email');
  await A.getByRole('button', { name: 'Отправить ссылку' }).click();
  await visible(A.getByText('Введите корректный email'), 'server email validation');
  log('«Забыли пароль?» opens the form; invalid address refused');

  // 3. Unknown address: the same "check your mail" answer, no letter.
  const ghost = `nobody-${Date.now()}@bilimway.test`;
  await A.getByLabel('Email').fill(ghost);
  await A.getByRole('button', { name: 'Отправить ссылку' }).click();
  await visible(A.getByRole('heading', { name: 'Проверьте почту' }), 'check mail for unknown address');
  const ghostText = await A.getByText(/Если аккаунт с адресом/).textContent();
  await shot(A, 'sent-unknown');

  // 4. «Изменить email» → the existing address: the same text, and a letter.
  await A.getByRole('button', { name: 'Изменить email' }).click();
  const focused = await A.evaluate(() => (document.activeElement instanceof HTMLInputElement ? document.activeElement.value : null));
  if (focused !== ghost) throw new Error('email field not focused for editing');
  await A.keyboard.type(TEACHER);
  await A.getByRole('button', { name: 'Отправить ссылку' }).click();
  await visible(A.getByRole('heading', { name: 'Проверьте почту' }), 'check mail for the account');
  const realText = await A.getByText(/Если аккаунт с адресом/).textContent();
  if (ghostText.replace(ghost, '{email}') !== realText.replace(TEACHER, '{email}')) throw new Error('answers differ for unknown and existing addresses');
  await visible(A.getByRole('button', { name: /^Отправить ещё раз через \d+:\d\d$/ }), 'resend countdown');
  await shot(A, 'sent');
  const first = linkOf(await waitLetter(TEACHER, 1));
  await new Promise((r) => setTimeout(r, 2000));
  if (letters(ghost).length) throw new Error('a letter was sent to an unknown address');
  log('unknown and existing address: same screen and text; letter only to the account (link not printed)');

  // 5. «Отправить ещё раз» after the server pause: a new letter; the first link stops working.
  const resend = A.getByRole('button', { name: 'Отправить ещё раз', exact: true });
  await resend.waitFor({ state: 'visible', timeout: 75000 });
  await resend.click();
  await visible(A.getByText(/мы отправили новое письмо/), 'resent notice');
  const latest = linkOf(await waitLetter(TEACHER, 2));
  if (latest === first) throw new Error('each letter must carry its own link');
  const O = await page();
  await O.goto(first);
  await visible(O.getByRole('heading', { name: 'Срок действия ссылки истёк' }), 'superseded link');
  log('resend after the pause: new link; the previous one refused');

  // 6. An expired link (another account; its expiry moved into the past in the test database).
  const B = await page();
  await B.goto(`${WEB}/forgot-password`);
  await B.getByLabel('Email').fill(OTHER);
  await B.getByRole('button', { name: 'Отправить ссылку' }).click();
  await visible(B.getByRole('heading', { name: 'Проверьте почту' }), 'check mail (other account)');
  const other = linkOf(await waitLetter(OTHER, 1));
  sql(`UPDATE password_resets SET expires_at = now() - interval '1 second' WHERE email = '${OTHER}' AND used_at IS NULL`);
  await O.goto(other);
  await visible(O.getByRole('heading', { name: 'Срок действия ссылки истёк' }), 'expired link');
  await shot(O, 'link-expired');
  log('expired link refused');

  // 7. Newest link → new password (token leaves the address bar) → sign-in screen with a notice.
  const C = await page();
  await C.goto(latest);
  await visible(C.getByRole('heading', { name: 'Новый пароль' }), 'new password form');
  if (C.url().includes('#')) throw new Error('token stayed in the address bar');
  await C.getByLabel(/^Новый пароль/).fill(newPassword);
  await C.getByLabel(/^Повторите пароль/).fill(`${newPassword}x`);
  await C.getByRole('button', { name: 'Сохранить пароль' }).click();
  await visible(C.getByText('Пароли не совпадают'), 'mismatch');
  await C.getByLabel(/^Новый пароль/).fill('short');
  await C.getByLabel(/^Повторите пароль/).fill('short');
  await C.getByRole('button', { name: 'Сохранить пароль' }).click();
  await visible(C.getByText('Пароль должен содержать не менее 8 символов'), 'server password validation');
  await C.getByLabel(/^Новый пароль/).fill(newPassword);
  await C.getByLabel(/^Повторите пароль/).fill(newPassword);
  await shot(C, 'new-password');
  await C.getByRole('button', { name: 'Сохранить пароль' }).click();
  await C.waitForURL('**/login?reset=done');
  await visible(C.getByText('Пароль изменён. Войдите с новым паролем.'), 'done notice');
  await shot(C, 'login-after-reset');
  log('new password saved; back on the sign-in screen');

  // 8. The old session is closed.
  if ((await me(S)) !== 401) throw new Error('old session still open');
  await S.goto(`${WEB}/cabinet/school/classes`);
  await S.waitForURL('**/login**');
  log('old session closed');

  // 9. Old password refused; new password signs in to the same account with the same role and rights.
  await login(C, TEACHER, PW.teacher);
  await visible(C.getByText('Неверный email или пароль'), 'old password refused');
  await login(C, TEACHER, newPassword);
  await C.waitForURL('**/cabinet/school/classes**');
  const after = await me(C);
  if (after.id !== before.id || after.role !== before.role || after.email !== before.email) throw new Error('account changed by the reset');
  await visible(C.getByRole('navigation').getByRole('link', { name: 'Ученики' }), 'school teacher keeps students');
  await C.goto(`${WEB}/materials`);
  await visible(C.getByRole('heading', { name: 'Материалы' }), 'materials still open');
  log('old password refused; new one works; same id, role and rights');

  // 10. The used link is refused.
  await O.goto(latest);
  await visible(O.getByRole('heading', { name: 'Ссылка уже использована' }), 'used link');
  await shot(O, 'link-used');
  await O.goto(`${WEB}/reset-password#${'A'.repeat(43)}`);
  await visible(O.getByRole('heading', { name: 'Ссылка недействительна' }), 'invalid link');
  log('used and invalid links refused');

  // 11. Database: only hashes, the link closed, no sessions of the teacher except the new one.
  const plain = [first, latest, other].map((l) => l.split('#')[1]);
  const stored = Number(sql(`SELECT count(*) FROM password_resets WHERE ${plain.map((t) => `position('${t}' in encode(token_hash, 'escape')) > 0`).join(' OR ')}`));
  const used = Number(sql(`SELECT count(*) FROM password_resets WHERE email = '${TEACHER}' AND used_at IS NOT NULL`));
  const bodies = Number(sql(`SELECT count(*) FROM email_outbox WHERE kind = 'password_reset' AND body LIKE '%reset-password#%'`));
  const ghostRows = sql(`SELECT count(*) || ':' || count(token_hash) FROM password_resets WHERE email = '${ghost}'`);
  if (stored !== 0 || used !== 1 || bodies !== 0 || ghostRows !== '1:0') throw new Error(`storage: plain=${stored} used=${used} bodies=${bodies} unknown=${ghostRows}`);
  log('only token hashes stored; letter bodies cleared; the unknown address got a row without a link');

  // 12. Administrator still reaches /admin.
  const F = await page();
  await login(F, 'admin@ortalyq.local', PW.admin);
  await F.waitForURL('**/admin');
  await visible(F.getByText(TEACHER), 'admin list');
  log('administrator opens /admin');

  if (errors.length) throw new Error(`browser errors:\n${errors.join('\n')}`);
  console.log(`\nAll password reset checks passed. Screenshots: ${OUT}`);
} catch (e) {
  console.error(`FAILED: ${e.message}`);
  process.exitCode = 1;
} finally {
  await browser.close();
}
