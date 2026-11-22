// Browser check of the private teacher sign-up: e-mail + consent → letter → link → password → welcome → cabinet,
// the used link, an existing address (same screen, letter without a link, account unchanged), existing sign-in
// and /admin. Needs running API + web, seeded demo data and the local SMTP receiver (cmd/mailsink) as the
// API's MAIL_SMTP_ADDR. Confirmation links and passwords are never printed.
//   MAIL_DIR=/path/to/mailsink-dir TEACHER_PASSWORD=… ADMIN_PASSWORD=… node scripts/signup-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const WEB = process.env.WEB_URL || 'http://localhost:3448';
const MAIL_DIR = process.env.MAIL_DIR || '../backend/data/mailsink';
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/signup';
// Demo passwords come from the environment or from the local seed settings; none is written in this script.
const seedEnv = Object.fromEntries(readFileSync(new URL('../../backend/.env.example', import.meta.url), 'utf8')
  .split('\n').map((l) => l.match(/^(SEED_[A-Z]+_PASSWORD)=(.*)$/)).filter(Boolean).map((m) => [m[1], m[2].trim()]));
const PW = {
  teacher: process.env.TEACHER_PASSWORD || seedEnv.SEED_TEACHER_PASSWORD,
  admin: process.env.ADMIN_PASSWORD || seedEnv.SEED_ADMIN_PASSWORD,
};
if (!PW.teacher || !PW.admin) throw new Error('set TEACHER_PASSWORD and ADMIN_PASSWORD');
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: true });
const ctxOpts = { viewport: { width: 1280, height: 860 }, locale: 'ru-RU', timezoneId: 'Asia/Almaty' };
const errors = [];
const page = async () => {
  const p = await (await browser.newContext(ctxOpts)).newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => m.type() === 'error' && !/Failed to load resource/.test(m.text()) && errors.push(`console: ${m.text()}`));
  return p;
};
let step = 0;
const shot = async (p, name) => { await p.waitForTimeout(300); await p.screenshot({ path: `${OUT}/${String(++step).padStart(2, '0')}-${name}.png` }); };
const log = (s) => console.log(`✓ ${s}`);
const visible = async (loc, what) => {
  await loc.first().waitFor({ state: 'visible', timeout: 15000 }).catch(() => { throw new Error(`not visible: ${what}`); });
};

const letters = (to) => readdirSync(MAIL_DIR).filter((f) => f.endsWith(`-${to.replace('@', '_at_')}.txt`)).sort().map((f) => readFileSync(join(MAIL_DIR, f), 'utf8'));
const waitLetter = async (to, n) => {
  for (let i = 0; i < 60; i++) {
    const l = letters(to);
    if (l.length >= n) return l[n - 1];
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`no letter #${n} to ${to}`);
};
const linkRe = /https?:\/\/\S+\/signup\/confirm#[A-Za-z0-9_-]{43}/;

const email = `tutor-${Date.now()}@bilimway.test`;
const typo = `tutor-${Date.now()}@bilimway.tset`;
const password = `Tutor-${Math.random().toString(36).slice(2)}-pass`;

try {
  // 0. Both forms on a wide and a phone screen.
  for (const [vp, size] of [['wide', { width: 1440, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
    const P = await (await browser.newContext({ ...ctxOpts, viewport: size, ...(vp === 'mobile' ? { isMobile: true, hasTouch: true, deviceScaleFactor: 2 } : {}) })).newPage();
    P.on('pageerror', (e) => errors.push(e.message));
    for (const form of ['login', 'signup']) {
      await P.goto(`${WEB}/${form}`);
      await visible(P.getByRole('button', { name: form === 'login' ? 'Войти' : 'Продолжить' }), `${form} form`);
      if (await P.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)) throw new Error(`horizontal scroll on ${form} (${vp})`);
      await P.waitForTimeout(300);
      await P.screenshot({ path: `${OUT}/${form}-${vp}.png`, fullPage: true });
    }
    await P.context().close();
  }
  log('both forms render on wide and mobile screens without horizontal scroll');

  // 1. «Вход / Регистрация» switch.
  const A = await page();
  await A.goto(`${WEB}/login`);
  const sw = A.getByRole('navigation', { name: 'Вход или регистрация' });
  await visible(sw.getByRole('link', { name: 'Вход' }), 'switch');
  if ((await sw.getByRole('link', { name: 'Вход' }).getAttribute('aria-current')) !== 'page') throw new Error('login tab not current');
  await sw.getByRole('link', { name: 'Регистрация' }).click();
  await A.waitForURL('**/signup');
  await visible(A.getByRole('heading', { name: 'Регистрация преподавателя' }), 'sign-up title');
  await sw.getByRole('link', { name: 'Вход' }).click();
  await A.waitForURL('**/login');
  await visible(A.getByRole('heading', { name: 'Вход в Bilimway' }), 'login title after switch');
  await visible(A.getByText('Демонстрационные аккаунты доступны только в локальной среде'), 'demo hint in the local build');
  if (await A.getByRole('button', { name: /Google/ }).count()) throw new Error('Google button shown without the flow');
  // Password reset is connected: the sign-in form links to it.
  await visible(A.getByRole('link', { name: 'Забыли пароль?' }), 'forgot password link');
  await sw.getByRole('link', { name: 'Регистрация' }).click();
  await A.waitForURL('**/signup');
  if ((await A.getByRole('textbox').count()) !== 1) throw new Error('sign-up must have exactly one text field');
  log('switch between sign-in and sign-up; no Google button; «Забыли пароль?» link');
  await visible(A.getByRole('heading', { name: 'Регистрация преподавателя' }), 'sign-up title');
  await visible(A.getByRole('link', { name: 'условия использования' }), 'terms link');
  await visible(A.getByRole('link', { name: 'политику конфиденциальности' }), 'privacy link');
  await shot(A, 'signup-email');

  // 2. Consent is required; the address is checked on the server.
  await A.getByLabel('Email').fill(email);
  await A.getByRole('button', { name: 'Продолжить' }).click();
  await visible(A.getByText('Чтобы продолжить, примите условия и политику'), 'consent error');
  await A.getByText('Я принимаю').click();
  await A.getByLabel('Email').fill('not-an-email');
  await A.getByRole('button', { name: 'Продолжить' }).click();
  await visible(A.getByText('Введите корректный email'), 'server email validation');
  log('consent and e-mail validation');

  // 3. Letter to a mistyped address, then «Изменить email» returns to the field with the address selected.
  await A.getByLabel('Email').fill(typo);
  await A.getByRole('button', { name: 'Продолжить' }).click();
  await visible(A.getByRole('heading', { name: 'Проверьте почту' }), 'check mail screen');
  await visible(A.getByRole('button', { name: 'Изменить email' }), 'change email button');
  await A.getByRole('button', { name: 'Изменить email' }).click();
  await visible(A.getByRole('heading', { name: 'Регистрация преподавателя' }), 'back to the form');
  const focus = await A.evaluate(() => {
    const el = document.activeElement;
    return el instanceof HTMLInputElement ? { type: el.type, value: el.value } : null;
  });
  if (!focus || focus.type !== 'email' || focus.value !== typo) throw new Error(`email field not focused for editing: ${JSON.stringify(focus)}`);
  await shot(A, 'signup-change-email');
  // The address is selected: typing replaces it (type=email exposes no selection range to check directly).
  await A.keyboard.type(email);
  if ((await A.getByLabel('Email').inputValue()) !== email) throw new Error('old address not selected for replacement');
  await A.getByRole('button', { name: 'Продолжить' }).click();
  await visible(A.getByText(`Мы отправили письмо на ${email}`), 'sent to the corrected address');
  await visible(A.getByRole('button', { name: /^Отправить ещё раз через \d+:\d\d$/ }), 'resend countdown');
  if (!(await A.getByRole('button', { name: /^Отправить ещё раз через/ }).isDisabled())) throw new Error('resend enabled during the countdown');
  await shot(A, 'signup-sent');
  const first = (await waitLetter(email, 1)).match(linkRe)?.[0];
  if (!first) throw new Error('no confirmation link in the first letter');
  log('change email: back to the selected field, letter to the corrected address (link not printed)');

  // 3a. The same address again before the server allows it: wait message with a timer, the button is locked.
  await A.getByRole('button', { name: 'Изменить email' }).click();
  await A.getByRole('button', { name: 'Продолжить' }).click();
  const tooSoon = A.getByRole('alert').filter({ hasText: /Новое письмо можно запросить через \d+:\d\d/ });
  await visible(tooSoon, 'server asks to wait');
  if (!(await A.getByRole('button', { name: 'Продолжить' }).isDisabled())) throw new Error('submit enabled while the server asks to wait');
  const t1 = await tooSoon.textContent();
  await A.waitForTimeout(2100);
  if ((await tooSoon.textContent()) === t1) throw new Error('wait timer does not tick');
  await shot(A, 'signup-wait');
  if (letters(email).length !== 1) throw new Error('a letter was sent despite the server limit');
  log('too early: server limit, message with a running timer, no extra letter');

  // 3b. When the timer ends the form unlocks: the second letter. Then «Отправить ещё раз» after the next pause.
  await A.getByRole('button', { name: 'Продолжить' }).and(A.locator(':enabled')).waitFor({ timeout: 75000 });
  if (await tooSoon.count()) throw new Error('wait message stays after the timer');
  await A.getByRole('button', { name: 'Продолжить' }).click();
  await visible(A.getByText(`Мы отправили письмо на ${email}`), 'second letter sent');
  const second = (await waitLetter(email, 2)).match(linkRe)?.[0];
  const resend = A.getByRole('button', { name: 'Отправить ещё раз', exact: true });
  await resend.waitFor({ state: 'visible', timeout: 75000 });
  await resend.click();
  await visible(A.getByText(`Новое письмо отправлено на ${email}`), 'resent notice');
  await visible(A.getByRole('button', { name: /^Отправить ещё раз через/ }), 'countdown after resend');
  await shot(A, 'signup-resent');
  const link = (await waitLetter(email, 3)).match(linkRe)?.[0];
  if (!second || !link || new Set([first, second, link]).size !== 3) throw new Error('each letter must carry its own link');
  log('resend after the pause: third letter, new link');

  // 3c. Earlier links to the same address no longer work.
  const O = await page();
  for (const old of [first, second]) {
    await O.goto(old);
    await visible(O.getByRole('heading', { name: 'Срок действия ссылки истёк' }), 'superseded link');
  }
  await shot(O, 'link-superseded');
  log('previous links are void after a newer letter');

  // 4. Link → password form; the token leaves the address bar.
  const B = await page();
  await B.goto(link);
  await visible(B.getByRole('heading', { name: 'Создайте пароль' }), 'password form');
  if (B.url().includes('#')) throw new Error('token stayed in the address bar');
  await shot(B, 'signup-password');
  await B.getByLabel(/^Пароль/).fill(password);
  await B.getByLabel('Повторите пароль').fill(`${password}x`);
  await B.getByRole('button', { name: 'Создать аккаунт' }).click();
  await visible(B.getByText('Пароли не совпадают'), 'mismatch');
  await B.getByLabel(/^Пароль/).fill('short');
  await B.getByLabel('Повторите пароль').fill('short');
  await B.getByRole('button', { name: 'Создать аккаунт' }).click();
  await visible(B.getByText('Пароль должен содержать не менее 8 символов'), 'server password validation');
  log('password validation (client match, server length)');
  await B.getByLabel(/^Пароль/).fill(password);
  await B.getByLabel('Повторите пароль').fill(password);
  await B.getByRole('button', { name: 'Создать аккаунт' }).click();

  // 5. Welcome → teacher cabinet.
  await B.waitForURL('**/signup/welcome');
  await visible(B.getByRole('heading', { name: 'Добро пожаловать в Bilimway!' }), 'welcome');
  await shot(B, 'signup-welcome');
  await B.getByRole('link', { name: 'Перейти в кабинет' }).click();
  await B.waitForURL('**/cabinet/school/classes**');
  const nav = B.getByRole('navigation');
  await visible(nav.getByRole('link', { name: 'Материалы' }), 'materials in the menu');
  if (await nav.getByRole('link', { name: 'Ученики' }).count()) throw new Error('school students item shown to a private teacher');
  await shot(B, 'cabinet');
  await B.goto(`${WEB}/materials`);
  await visible(B.getByRole('heading', { name: 'Материалы' }), 'materials page opens');
  log('account created, signed in, teacher cabinet (no school students)');

  // 6. The same link again.
  const C = await page();
  await C.goto(link);
  await visible(C.getByRole('heading', { name: 'Ссылка уже использована' }), 'used link');
  await shot(C, 'link-used');
  await C.goto(`${WEB}/signup/confirm#${'A'.repeat(43)}`);
  await visible(C.getByRole('heading', { name: 'Ссылка недействительна' }), 'invalid link');
  log('used and invalid links');

  // 7. Existing address: the same screen; the letter has no link; the account still signs in with its password.
  const before = letters('teacher@ortalyq.local').length;
  const D = await page();
  await D.goto(`${WEB}/signup`);
  await D.getByLabel('Email').fill('teacher@ortalyq.local');
  await D.getByText('Я принимаю').click();
  await D.getByRole('button', { name: 'Продолжить' }).click();
  await visible(D.getByRole('heading', { name: 'Проверьте почту' }), 'same screen for an existing address');
  const existing = await waitLetter('teacher@ortalyq.local', before + 1);
  if (linkRe.test(existing) || !existing.includes('/login')) throw new Error('letter to an existing address must not carry a link');
  log('existing address: same answer, letter without a link');

  // 8. Sign-in errors, password visibility, existing sign-in; 9. administrator reaches /admin and sees the new role.
  const E = await page();
  await E.goto(`${WEB}/login`);
  await E.getByRole('button', { name: 'Войти' }).click();
  await visible(E.locator('[aria-invalid="true"]'), 'empty sign-in fields flagged');
  await E.getByLabel('Email').fill('teacher@ortalyq.local');
  await E.getByLabel('Пароль', { exact: true }).fill('wrong-password-1');
  await E.getByRole('button', { name: 'Войти' }).click();
  await visible(E.getByRole('alert'), 'wrong password message');
  await shot(E, 'login-error');
  const pw = E.getByLabel('Пароль', { exact: true });
  await E.getByRole('button', { name: 'Показать пароль' }).click();
  if ((await pw.getAttribute('type')) !== 'text') throw new Error('password not shown');
  await E.getByRole('button', { name: 'Скрыть пароль' }).click();
  if ((await pw.getAttribute('type')) !== 'password') throw new Error('password not hidden');
  log('sign-in errors and show/hide password');
  await E.getByLabel('Email').fill('teacher@ortalyq.local');
  await E.getByLabel('Пароль', { exact: true }).fill(PW.teacher);
  await E.getByRole('button', { name: 'Войти' }).click();
  await E.waitForURL('**/cabinet/school/classes**');
  await visible(E.getByRole('navigation').getByRole('link', { name: 'Ученики' }), 'school teacher keeps students');
  log('existing teacher signs in, role unchanged');
  const F = await page();
  await F.goto(`${WEB}/login`);
  await F.getByLabel('Email').fill('admin@ortalyq.local');
  await F.getByLabel('Пароль', { exact: true }).fill(PW.admin);
  await F.getByRole('button', { name: 'Войти' }).click();
  await F.waitForURL('**/admin');
  await visible(F.getByText(email), 'new account in the admin list');
  await shot(F, 'admin');
  log('administrator opens /admin and sees the new private teacher');

  if (errors.length) throw new Error(`browser errors:\n${errors.join('\n')}`);
  console.log(`\nAll sign-up checks passed. Screenshots: ${OUT}`);
} catch (e) {
  console.error(`FAILED: ${e.message}`);
  process.exitCode = 1;
} finally {
  await browser.close();
}
