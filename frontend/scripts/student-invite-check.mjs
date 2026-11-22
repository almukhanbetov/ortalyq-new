// Browser check of a private teacher's student invitation, the whole path on a test stand:
//   new private teacher (sign-up by e-mail) → «Новый класс» shows the invitation hint and «Пригласить ученика» →
//   letter through the local SMTP receiver → the student creates an account from the link → the student appears in
//   the teacher's list → individual class → the student sees the class. Then: another private teacher cannot choose
//   this student (not in the list, refused by the server) nor open the class; an existing student accepts after
//   signing in (no second account); a signed-in wrong account is refused; a used link is refused.
// Needs running API + web, seeded demo data, cmd/mailsink as the API's MAIL_SMTP_ADDR and a throwaway database
// (PG_CONTAINER/PG_USER/PG_DB, read-only queries). Links, tokens and passwords are never printed.
//   WEB_URL=… API_URL=… MAIL_DIR=… PG_CONTAINER=… PG_USER=… PG_DB=… node scripts/student-invite-check.mjs
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const WEB = process.env.WEB_URL || 'http://localhost:3448';
const API = process.env.API_URL || 'http://localhost:8448';
const MAIL_DIR = process.env.MAIL_DIR || '../backend/data/mailsink';
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/student-invite';
const PG = { container: process.env.PG_CONTAINER, user: process.env.PG_USER, db: process.env.PG_DB };
if (!PG.container || !PG.user || !PG.db) throw new Error('set PG_CONTAINER, PG_USER and PG_DB of the throwaway test database');
const seedEnv = Object.fromEntries(readFileSync(new URL('../../backend/.env.example', import.meta.url), 'utf8')
  .split('\n').map((l) => l.match(/^(SEED_[A-Z]+_PASSWORD)=(.*)$/)).filter(Boolean).map((m) => [m[1], m[2].trim()]));
const STUDENT_PW = process.env.STUDENT_PASSWORD || seedEnv.SEED_STUDENT_PASSWORD;
if (!STUDENT_PW) throw new Error('set STUDENT_PASSWORD');
const CLASSES = '/cabinet/school/classes';
mkdirSync(OUT, { recursive: true });

const sql = (q) => execFileSync('docker', ['exec', PG.container, 'psql', '-U', PG.user, '-d', PG.db, '-tAc', q], { encoding: 'utf8' }).trim();
const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: true });
const ctxOpts = { viewport: { width: 1280, height: 860 }, locale: 'ru-RU', timezoneId: 'Asia/Almaty' };
const errors = [];
const page = async (opts = {}) => {
  const p = await (await browser.newContext({ ...ctxOpts, ...opts })).newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => m.type() === 'error' && !/Failed to load resource/.test(m.text()) && errors.push(`console: ${m.text()}`));
  return p;
};
let step = 0;
const shot = async (p, name) => { await p.waitForTimeout(300); await p.screenshot({ path: `${OUT}/${String(++step).padStart(2, '0')}-${name}.png` }); };
const log = (s) => console.log(`✓ ${s}`);
const visible = async (loc, what, timeout = 15000) => {
  await loc.first().waitFor({ state: 'visible', timeout }).catch(() => { throw new Error(`not visible: ${what}`); });
};
const letters = (to) => readdirSync(MAIL_DIR).filter((f) => f.endsWith(`-${to.replace('@', '_at_')}.txt`)).sort().map((f) => readFileSync(join(MAIL_DIR, f), 'utf8'));
const waitLetter = async (to, n, re) => {
  for (let i = 0; i < 60; i++) {
    const l = letters(to).filter((x) => re.test(x));
    if (l.length >= n) return l[n - 1].match(re)[0].replace(/=\r?\n/g, '');
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`no letter #${n} to ${to}`);
};
const signupRe = /https?:\/\/\S+\/signup\/confirm#[A-Za-z0-9_-]{43}/;
const inviteRe = /https?:\/\/\S+\/student-invite#[A-Za-z0-9_-]{43}/;
const run = Date.now();
const secret = () => `Pw-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;

/** A new private teacher through the real sign-up screens; returns the signed-in page. */
async function newTeacher(email) {
  const P = await page();
  await P.goto(`${WEB}/signup`);
  await P.getByLabel('Email').fill(email);
  await P.getByText('Я принимаю').click();
  await P.getByRole('button', { name: 'Продолжить' }).click();
  const link = await waitLetter(email, 1, signupRe);
  await P.goto(link);
  const pw = secret();
  await P.getByLabel(/^Пароль/).fill(pw);
  await P.getByLabel('Повторите пароль').fill(pw);
  await P.getByRole('button', { name: 'Создать аккаунт' }).click();
  await P.waitForURL('**/signup/welcome');
  return P;
}

async function openIndividualForm(P) {
  await P.goto(`${WEB}${CLASSES}`);
  await P.getByRole('button', { name: 'Создать класс' }).first().click();
  await P.getByRole('button', { name: /Индивидуальный класс/ }).click();
  await visible(P.getByRole('dialog').getByText('Новый класс'), 'new class form');
}

const apiCall = (P, method, path, body) => P.evaluate(async ([m, url, b]) => {
  const r = await fetch(url, { method: m, credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: b ? JSON.stringify(b) : undefined });
  return { status: r.status, body: await r.json().catch(() => null) };
}, [method, `${API}/api${path}`, body]);

try {
  // 1. A new private teacher; «Новый класс» → the invitation hint instead of the administrator one.
  const tutorEmail = `tutor-${run}@bilimway.test`;
  const T = await newTeacher(tutorEmail);
  log('new private teacher signed up by e-mail');
  await openIndividualForm(T);
  const dlg = T.getByRole('dialog');
  await visible(dlg.getByText('У вас пока нет учеников. Пригласите ученика по email'), 'invitation hint');
  if (await dlg.getByText('Администратор ещё не назначил вам учеников').count()) throw new Error('administrator hint shown to a private teacher');
  await visible(dlg.getByRole('button', { name: 'Пригласить ученика' }), 'invite button');
  await shot(T, 'new-class-private-empty');
  log('«Новый класс»: invitation hint and «Пригласить ученика» next to «Ученик»');

  // 2. Invite a new address; the letter arrives through the test SMTP receiver.
  const studentEmail = `pupil-${run}@bilimway.test`;
  await dlg.getByRole('button', { name: 'Пригласить ученика' }).click();
  await dlg.getByLabel(/^Email ученика/).fill('not-an-email');
  await dlg.getByRole('button', { name: 'Отправить приглашение' }).click();
  await visible(dlg.getByText('Введите корректный email'), 'server email validation');
  await dlg.getByLabel(/^Email ученика/).fill(studentEmail);
  await dlg.getByRole('button', { name: 'Отправить приглашение' }).click();
  await visible(dlg.getByText(`Приглашение отправлено на ${studentEmail}`), 'sent notice');
  await shot(T, 'invite-sent');
  const link = await waitLetter(studentEmail, 1, inviteRe);
  log('invitation letter delivered to the test SMTP receiver');

  // 3. The student opens the link: a new account is created from it.
  const S = await page();
  await S.goto(link);
  await visible(S.locator('[data-testid="student-invite"][data-state="register"]'), 'registration form');
  if (S.url().includes('#')) throw new Error('token stayed in the address bar');
  await visible(S.getByText(`Приглашение отправлено на ${studentEmail}`), 'invited address');
  await shot(S, 'student-register');
  const pupilPw = secret();
  await S.getByLabel(/^Имя и фамилия/).fill('Асель Ученица');
  await S.getByLabel(/^Пароль/).fill(pupilPw);
  await S.getByLabel(/^Повторите пароль/).fill(pupilPw);
  await S.getByRole('button', { name: 'Создать аккаунт и принять' }).click();
  await visible(S.getByText('Чтобы продолжить, примите условия и политику'), 'consent required');
  await S.getByText('Я принимаю').click();
  await S.getByRole('button', { name: 'Создать аккаунт и принять' }).click();
  await visible(S.locator('[data-testid="student-invite-accepted"]'), 'accepted');
  await shot(S, 'student-accepted');
  if (sql(`SELECT role FROM users WHERE email = '${studentEmail}'`) !== 'student') throw new Error('student account not created');
  log('student account created from the link and the invitation accepted');

  // 4. The student appears in the teacher's list (refreshes by itself); an individual class is created.
  // <option> of a closed <select> is never "visible": wait until it is in the list.
  await dlg.locator('option', { hasText: studentEmail }).first().waitFor({ state: 'attached', timeout: 30000 })
    .catch(() => { throw new Error('not in the list: invited student'); });
  await dlg.getByLabel(/^Название класса/).fill('Английский с Асель');
  await dlg.getByLabel(/^Ученик/).selectOption({ label: `Асель Ученица · ${studentEmail}` });
  await shot(T, 'new-class-student-chosen');
  await dlg.getByRole('button', { name: 'Создать', exact: true }).click();
  await T.waitForURL(/\/classes\/[0-9a-f-]{36}/);
  const classId = T.url().match(/\/classes\/([0-9a-f-]{36})/)[1];
  log('individual class created with the invited student');

  // 5. The student sees the class and can open it.
  await S.getByRole('link', { name: 'Перейти к моим классам' }).click();
  await visible(S.getByText('Английский с Асель'), 'class in the student list');
  await shot(S, 'student-classes');
  const own = await apiCall(S, 'GET', `/classrooms/${classId}`);
  if (own.status !== 200) throw new Error(`student cannot open the class: ${own.status}`);
  log('the student sees the class');

  // 6. Another private teacher: not in the list, refused by the server, the class does not open.
  const U = await newTeacher(`tutor2-${run}@bilimway.test`);
  await openIndividualForm(U);
  const dlg2 = U.getByRole('dialog');
  if (await dlg2.locator('option', { hasText: studentEmail }).count()) throw new Error('foreign student in the list');
  await shot(U, 'other-teacher-form');
  const studentId = sql(`SELECT id FROM users WHERE email = '${studentEmail}'`);
  const forged = await apiCall(U, 'POST', '/classrooms', { name: 'Чужой', student_id: studentId, lesson_type: 'individual' });
  if (forged.status !== 422 || !forged.body?.error?.fields?.student_id) throw new Error(`foreign student accepted: ${forged.status}`);
  const peek = await apiCall(U, 'GET', `/classrooms/${classId}`);
  if (peek.status !== 404) throw new Error(`foreign class opened: ${peek.status}`);
  await U.goto(`${WEB}/classes/${classId}`);
  await U.waitForTimeout(1500);
  if (await U.getByText('Английский с Асель').count()) throw new Error('foreign class rendered');
  await shot(U, 'other-teacher-foreign-class');
  log('another private teacher cannot choose the student (422) nor open the class (404)');

  // 7. An existing student accepts after signing in with the invited address; no second account.
  const existing = 'student@ortalyq.local';
  await T.goto(`${WEB}${CLASSES}`);
  await openIndividualForm(T);
  await dlg.getByRole('button', { name: 'Пригласить ученика' }).click();
  const before = letters(existing).filter((x) => inviteRe.test(x)).length;
  await dlg.getByLabel(/^Email ученика/).fill(existing);
  await dlg.getByRole('button', { name: 'Отправить приглашение' }).click();
  await visible(dlg.getByText(`Приглашение отправлено на ${existing}`), 'sent to existing');
  const exLink = await waitLetter(existing, before + 1, inviteRe);
  const E = await page();
  await E.goto(exLink);
  await visible(E.locator('[data-testid="student-invite"][data-state="sign-in"]'), 'sign-in state');
  await shot(E, 'existing-sign-in');
  await E.getByRole('link', { name: 'Войти в аккаунт' }).click();
  await E.getByLabel('Email').fill(existing);
  await E.getByLabel(/^Пароль/).fill(STUDENT_PW);
  await E.getByRole('button', { name: 'Войти' }).click();
  await E.waitForURL('**/student-invite');
  await visible(E.locator('[data-testid="student-invite"][data-state="accept"]'), 'accept state');
  await E.getByRole('button', { name: 'Принять приглашение' }).click();
  await visible(E.locator('[data-testid="student-invite-accepted"]'), 'existing accepted');
  await shot(E, 'existing-accepted');
  if (sql(`SELECT count(*) FROM users WHERE lower(email) = '${existing}'`) !== '1') throw new Error('duplicate account');
  const list = await apiCall(T, 'GET', '/teacher/students');
  if (!list.body?.items?.some((u) => u.email === existing)) throw new Error('existing student not linked');
  log('existing student accepted after sign-in, one account');

  // 8. The used link is refused; a signed-in wrong account is refused.
  const R = await page();
  await R.goto(link);
  await visible(R.getByRole('heading', { name: 'Приглашение уже принято' }), 'used link');
  await shot(R, 'link-used');
  const strangerEmail = `stranger-${run}@bilimway.test`;
  await openIndividualForm(T);
  await dlg.getByRole('button', { name: 'Пригласить ученика' }).click();
  await dlg.getByLabel(/^Email ученика/).fill(strangerEmail);
  await dlg.getByRole('button', { name: 'Отправить приглашение' }).click();
  const strangerLink = await waitLetter(strangerEmail, 1, inviteRe);
  await E.goto(strangerLink);
  await visible(E.locator('[data-testid="student-invite"][data-state="blocked"]'), 'wrong account');
  await shot(E, 'wrong-account');
  log('used link and wrong account are refused');

  if (errors.length) throw new Error(`page errors:\n${errors.join('\n')}`);
  console.log('\nALL CHECKS PASSED');
} catch (e) {
  console.error(`✗ ${e.message}`);
  process.exitCode = 1;
} finally {
  await browser.close();
}
