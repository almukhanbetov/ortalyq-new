// Marathon stage 9: invitation by e-mail in the browser, with a local SMTP test receiver (nothing leaves the
// machine). The teacher invites from «Добавить ученика → Вручную» (letter language, per-address results), sees the
// invitation and its delivery; the letter's link opens the invitation page; a wrong account is refused; an
// anonymous visitor signs in and accepts; the used link, an expired link and a revoked link explain themselves.
// It CREATES data (its own teacher and two students, through the administrator), so it runs only against an isolated
// stack with its own DB and the mailsink receiver; the web address comes from WEB_URL (the API's APP_URL must match):
//   (backend) go run ./cmd/mailsink -addr 127.0.0.1:2525 -dir $MAIL_DIR
//   (backend) MAIL_DRIVER=smtp MAIL_SMTP_ADDR=127.0.0.1:2525 MAIL_FROM=noreply@ortalyq.local APP_URL=$WEB_URL … api serve
//   WEB_URL=… API_URL=… E2E_DB=<isolated db> MAIL_DIR=… node scripts/marathon-invite-check.mjs
// Screenshots go to a new folder per run (SHOTS_DIR), never over the screenshots of earlier stages.
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { isolatedStack, makeUsers, reEscape } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB, API } = stack;
const DB = process.env.E2E_DB;
const MAIL_DIR = process.env.MAIL_DIR;
if (!DB || DB === 'ortalyq_new') throw new Error('E2E_DB (the isolated database of the stack) is required');
if (!MAIL_DIR) throw new Error('MAIL_DIR (the mailsink folder) is required');
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/marathons/stage9-run-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });

const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const RUN = Date.now().toString(36);
const LINK_RE = new RegExp(`${reEscape(WEB)}/invite/[A-Za-z0-9_-]{43}`);
const mask = (e) => `${e.slice(0, 2)}***${e.slice(e.indexOf('@'))}`;
// The isolated DB only: used to move an invitation past its expiry.
const COMPOSE = fileURLToPath(new URL('../../docker-compose.yml', import.meta.url));
const sql = (q) => execFileSync('docker', ['compose', '-f', COMPOSE, 'exec', '-T', 'postgres', 'psql', '-U', 'ortalyq_new', '-d', DB, '-Atc', q]).toString().trim();

const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
async function context() {
  const ctx = await b.newContext({ viewport: { width: 1862, height: 961 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 4(01|03|04|09|10)/.test(m.text())) errors.push(m.text()); });
  return { ctx, p };
}
async function signIn(p, email, pw) {
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(pw);
  await p.getByRole('button', { name: 'Войти' }).click();
}
async function login(email, pw) {
  const c = await context();
  await c.p.goto(`${WEB}/login`);
  await signIn(c.p, email, pw);
  await c.p.waitForURL(/\/classes$/);
  await c.p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return c;
}
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(400));
const call = (p, method, path, body) => p.evaluate(async ({ API, method, path, body }) => {
  const r = await fetch(API + '/api' + path, { method, credentials: 'include', headers: body ? { 'Content-Type': 'application/json' } : {}, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  return { status: r.status, body: text ? JSON.parse(text) : null };
}, { API, method, path, body });
const must = async (p, method, path, body, want) => {
  const r = await call(p, method, path, body);
  assert(r.status === want, `${method} ${path}: ${r.status} ${JSON.stringify(r.body)}`);
  return r.body;
};
// The newest letter to an address in the mailsink folder, with its invitation link.
const letterTo = async (to, after = 0) => {
  for (let i = 0; i < 50; i++) {
    const files = readdirSync(MAIL_DIR).filter((f) => f.includes(to.replace('@', '_at_'))).sort();
    if (files.length > after) {
      const text = readFileSync(join(MAIL_DIR, files[files.length - 1]), 'utf8');
      const link = text.match(LINK_RE)?.[0];
      return { text, link, count: files.length };
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`no letter to ${to}`);
};
const count = (to) => readdirSync(MAIL_DIR).filter((f) => f.includes(to.replace('@', '_at_'))).length;

try {
  // Own users: the invited student (S) and another assigned student (S2) of a new teacher.
  const users = await makeUsers(stack, RUN, { students: ['Данияр', 'Алия'] });
  const [SU, SU2] = users.students;
  const T = await login(users.teacher.email, users.teacher.password);
  const S2 = await login(SU2.email, SU2.password);
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон по приглашению ${RUN}`, direction: 'other' }, 201);
  const base = `${WEB}/cabinet/school/marathons/marathon/${m.id}`;
  const before = count(SU.email);

  // ---- «Добавить ученика → Вручную» ----
  await T.p.goto(`${base}/students`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить ученика' }).click();
  const dlg = T.p.getByRole('dialog', { name: 'Добавить учеников' });
  await dlg.getByRole('tab', { name: 'Вручную' }).click();
  for (const email of [SU.email, 'nobody@example.com']) {
    await dlg.getByLabel('Email ученика').fill(email);
    await dlg.getByRole('button', { name: 'Добавить в список' }).click();
  }
  await dlg.getByLabel('Язык писем').selectOption('kk');
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/01-manual-invite.png` });
  await dlg.getByRole('button', { name: 'Пригласить (2)' }).click();
  await dlg.getByTestId('invite-results').waitFor();
  const results = await dlg.getByTestId('invite-results').locator('li').evaluateAll((lis) => lis.map((li) => li.dataset.result));
  assert(JSON.stringify(results) === JSON.stringify(['invited', 'unavailable']), `results: ${results}`);
  await T.p.screenshot({ path: `${OUT}/02-invite-results.png` });
  await dlg.getByRole('button', { name: 'Готово' }).click();
  ok('«Вручную»: два адреса, язык писем — казахский; результат по адресу: отправлено / нет назначенного ученика');

  // ---- The letter (local receiver) and the invitation list ----
  const letter = await letterTo(SU.email, before);
  assert(letter.link && letter.text.includes('Сәлеметсіз') && letter.text.includes(`Марафон по приглашению ${RUN}`), `letter: ${letter.text}`);
  await T.p.waitForFunction(() => document.querySelector('[data-testid="invitations"] li')?.getAttribute('data-delivery') === 'sent', null, { timeout: 15000 });
  await T.p.getByTestId('invitations').scrollIntoViewIfNeeded();
  await T.p.screenshot({ path: `${OUT}/03-invitations-list.png` });
  ok('Письмо пришло в тестовый приёмник (казахский текст, ссылка); в «Приглашения» — «Приглашён», «Письмо отправлено»');

  // ---- A wrong account cannot accept ----
  await S2.p.goto(letter.link);
  await S2.p.getByTestId('invitation-page').waitFor();
  await S2.p.getByRole('button', { name: 'Принять приглашение' }).click();
  await S2.p.getByRole('alert').filter({ hasText: mask(SU.email) }).waitFor();
  await S2.p.screenshot({ path: `${OUT}/04-wrong-account.png` });
  ok(`Чужая учётная запись: «Приглашение отправлено на ${mask(SU.email)}. Войдите под этой учётной записью.»`);

  // ---- An anonymous visitor signs in and accepts ----
  const A = await context();
  await A.p.goto(letter.link);
  await A.p.getByTestId('invitation-page').waitFor();
  await settle(A.p);
  await A.p.screenshot({ path: `${OUT}/05-invitation-page-anonymous.png` });
  await A.p.getByRole('link', { name: 'Войти, чтобы принять' }).click();
  await A.p.waitForURL(/\/login\?next=/);
  await signIn(A.p, SU.email, SU.password);
  await A.p.waitForURL(/\/invite\//);
  await A.p.getByRole('button', { name: 'Принять приглашение' }).click();
  await A.p.waitForURL(new RegExp(`/marathons/marathon/${m.id}/`));
  await settle(A.p);
  await A.p.screenshot({ path: `${OUT}/06-student-joined.png` });
  ok('Аноним: страница приглашения → «Войти, чтобы принять» → вход учеником → возврат на приглашение → «Принять» → страница марафона');

  // ---- The used link; the teacher's view ----
  await S2.p.goto(letter.link);
  await S2.p.getByText('Приглашение уже использовано.').waitFor();
  await T.p.reload();
  await settle(T.p);
  assert(await T.p.locator('[data-testid="invitations"] li').first().getAttribute('data-status') === 'accepted', 'accepted in the list');
  assert((await T.p.getByTestId('marathon-students').innerText()).includes(SU.name), 'student in the list');
  ok('Использованная ссылка: «Приглашение уже использовано»; у учителя — «Принято», ученик в списке');

  // ---- Revocation from the list ----
  const before2 = count(SU2.email);
  await must(T.p, 'POST', `/marathons/${m.id}/invitations`, { emails: [SU2.email], locale: 'ru' }, 200);
  const l2 = await letterTo(SU2.email, before2);
  await T.p.reload();
  await settle(T.p);
  await T.p.getByRole('button', { name: `Действия с приглашением ${SU2.email}` }).click();
  await T.p.getByRole('menuitem', { name: 'Отменить приглашение' }).click();
  await T.p.getByRole('dialog', { name: 'Отменить приглашение?' }).getByRole('button', { name: 'Отменить приглашение' }).click();
  await T.p.waitForFunction(() => [...document.querySelectorAll('[data-testid="invitations"] li')].some((li) => li.dataset.status === 'revoked'));
  await S2.p.goto(l2.link);
  await S2.p.getByText('Преподаватель отменил это приглашение.').waitFor();
  await must(S2.p, 'POST', `/invitations/${l2.link.split('/').pop()}/accept`, null, 410);
  ok('«Отменить приглашение» с подтверждением: ссылка показывает отмену, принятие — 410');

  // ---- Expiry (moved in the isolated DB) and «Отправить повторно» ----
  const before3 = count(SU2.email);
  await must(T.p, 'POST', `/marathons/${m.id}/invitations`, { emails: [SU2.email], locale: 'ru' }, 200);
  const l3 = await letterTo(SU2.email, before3);
  sql(`UPDATE marathon_invitations SET expires_at = now() - interval '1 second' WHERE classroom_id = '${m.id}' AND email = '${SU2.email}' AND revoked_at IS NULL`);
  await S2.p.goto(l3.link);
  await S2.p.getByText('Срок действия приглашения истёк.', { exact: false }).first().waitFor();
  await S2.p.screenshot({ path: `${OUT}/07-expired-link.png` });
  await T.p.reload();
  await settle(T.p);
  await T.p.getByRole('button', { name: `Действия с приглашением ${SU2.email}` }).click();
  await T.p.getByRole('menuitem', { name: 'Отправить повторно' }).click();
  const l4 = await letterTo(SU2.email, before3 + 1);
  assert(l4.link !== l3.link, 'a new link');
  await S2.p.goto(l3.link);
  await S2.p.getByText('Приглашение не найдено или больше недействительно.').waitFor();
  await S2.p.goto(l4.link);
  await S2.p.getByRole('button', { name: 'Принять приглашение' }).click();
  await S2.p.waitForURL(new RegExp(`/marathons/marathon/${m.id}/`));
  ok('Истёкшая ссылка объясняет себя; «Отправить повторно» — новая ссылка, старая не найдена, новая принимается');

  // ---- Phone width ----
  await A.p.setViewportSize({ width: 390, height: 844 });
  await A.p.goto(l4.link);
  await settle(A.p);
  const sw = await A.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(sw <= 0, `horizontal scroll: ${sw}px`);
  await A.p.screenshot({ path: `${OUT}/08-phone-invitation-page.png` });
  ok('Телефон 390 px: страница приглашения без горизонтальной прокрутки');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
