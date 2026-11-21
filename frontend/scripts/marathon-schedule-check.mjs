// Marathon stage 5 schedules and automatic opening:
//  1) a lesson with an opening date opens by itself, without reloading, both in the student's lesson list and in
//     the lesson page left open on the lock screen (the page re-reads at the server moment);
//  2) «По интервалам» and «По дням недели» set in the settings dialog; the student's schedule counts from
//     max(start_at, the day they were added), lesson 1 opens at that moment (Н1 = A); a late student has their own
//     schedule; changing only the schedule keeps the progress epoch.
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e node scripts/marathon-schedule-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/schedule';
mkdirSync(OUT, { recursive: true });

const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const RUN = Date.now().toString(36);

const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
async function login(email, pw) {
  const ctx = await b.newContext({ viewport: { width: 1862, height: 961 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 40[1349]/.test(m.text())) errors.push(m.text()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(pw);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes$/);
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return { ctx, p };
}
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(400));
const call = (p, method, path, body) => p.evaluate(async ({ API, method, path, body }) => {
  const r = await fetch(API + '/api' + path, { method, credentials: 'include', headers: body ? { 'Content-Type': 'application/json' } : {}, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  return { status: r.status, body: text ? JSON.parse(text) : null };
}, { API, method, path, body });
const must = async (p, method, path, body, want, code) => {
  const r = await call(p, method, path, body);
  assert(r.status === want, `${method} ${path}: ${r.status} ${JSON.stringify(r.body)}`);
  if (code) assert(r.body?.error?.code === code, `${method} ${path}: code ${JSON.stringify(r.body)}`);
  return r.body;
};
const lockTexts = (p) => p.getByTestId('student-lessons').locator('li').evaluateAll((lis) => lis.map((li) => li.querySelector('[data-testid="lesson-lock"]')?.textContent ?? ''));

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');
  const S2 = await login('student2@ortalyq.local', 'student-local-123');
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local')?.id;
  const S2_ID = mine.items.find((u) => u.email === 'student2@ortalyq.local')?.id;
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон расписания ${RUN}`, direction: 'other' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс расписания ${RUN}` }, 201);
  const L = [];
  for (const title of ['Урок 1', 'Урок 2', 'Урок 3']) {
    const l = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title }, 201);
    await must(T.p, 'POST', `/lessons/${l.id}/sections`, { title: 'Section 1' }, 201);
    await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: l.id }, 200);
    L.push({ id: l.id, title });
  }
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID] }, 200);
  const studentsUrl = `${WEB}/cabinet/school/marathons/marathon/${m.id}/students`;

  // ---- 1. Automatic opening without reloading ----
  const opensAt = new Date(Date.now() + 25_000);
  await must(T.p, 'PUT', `/marathons/${m.id}/lessons/${L[1].id}/open-at`, { open_at: opensAt.toISOString() }, 200);
  // The lesson page is open in a second window of the same student (a separate context keeps both windows rendered).
  const S3 = await login('student@ortalyq.local', 'student-local-123');
  const P2 = S3.p;
  await S.p.goto(studentsUrl);
  await P2.goto(`${WEB}/cabinet/school/marathons/marathon/${m.id}/lesson/${L[1].id}`);
  await Promise.all([settle(S.p), settle(P2)]);
  let locks = await lockTexts(S.p);
  assert(locks[1].startsWith('Откроется'), `locked before the moment: ${locks}`);
  await P2.getByText('Урок пока закрыт').waitFor();
  await S.p.screenshot({ path: `${OUT}/01-before-opening.png` });
  const navigations = { list: 0, lesson: 0 };
  S.p.on('framenavigated', (f) => { if (f === S.p.mainFrame()) navigations.list++; });
  P2.on('framenavigated', (f) => { if (f === P2.mainFrame()) navigations.lesson++; });
  // No reload: the pages must re-read by themselves at the server moment.
  await S.p.getByTestId('student-lessons').locator('li').nth(1).getByRole('link', { name: 'Открыть урок' }).waitFor({ timeout: 60_000 });
  await P2.getByRole('heading', { name: 'Урок 2' }).waitFor({ timeout: 60_000 }).catch(async (e) => { await P2.screenshot({ path: `${OUT}/debug-lesson-page.png` }); throw e; });
  assert(Date.now() >= opensAt.getTime() - 1000, 'not opened before the moment');
  assert(navigations.list === 0 && navigations.lesson === 0, `pages were reloaded: ${JSON.stringify(navigations)}`);
  await S.p.screenshot({ path: `${OUT}/02-opened-without-reload.png` });
  await P2.screenshot({ path: `${OUT}/03-lesson-page-opened-without-reload.png` });
  await S3.ctx.close();
  ok('Урок с «Датой открытия» открылся сам, без обновления страницы: в списке ученика и на открытой странице урока');

  // ---- 2. «По интервалам» through the settings dialog ----
  await must(T.p, 'PUT', `/marathons/${m.id}/lessons/${L[1].id}/open-at`, { open_at: null }, 200);
  await T.p.goto(studentsUrl);
  await settle(T.p);
  const openPassage = async () => {
    await T.p.getByRole('button', { name: 'Настройки' }).click();
    const st = T.p.getByRole('dialog', { name: 'Настройки марафона' });
    await st.getByRole('tab', { name: 'Прохождение' }).click();
    return st;
  };
  let st = await openPassage();
  await st.getByRole('radio', { name: /^По интервалам/ }).click();
  const fields = st.getByTestId('schedule-fields');
  await fields.getByLabel('Как часто').selectOption('2');
  await fields.getByLabel(/^Время/).selectOption('08:00');
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/04-interval-settings.png` });
  await fields.getByRole('button', { name: 'Сменить способ прохождения' }).click();
  await T.p.getByRole('dialog', { name: 'Смена способа прохождения' }).getByRole('button', { name: 'Продолжить' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Способ прохождения изменён' }).waitFor();
  await T.p.keyboard.press('Escape');
  let info = await must(T.p, 'GET', `/classrooms/${m.id}`, null, 200);
  assert(info.marathon.mode === 'interval' && info.marathon.interval_days === 2 && info.marathon.open_time === '08:00', `interval saved: ${JSON.stringify(info.marathon)}`);
  const epoch = info.marathon.progress_epoch;
  const me = await must(S.p, 'GET', `/marathons/${m.id}/me`, null, 200);
  assert(me.items[0].access === 'open' && me.items[1].lock?.reason === 'scheduled', `interval me: ${JSON.stringify(me.items.map((l) => l.access))}`);
  await S.p.reload();
  await settle(S.p);
  locks = await lockTexts(S.p);
  assert(locks[0] === '' && /08:00/.test(locks[1]) && /08:00/.test(locks[2]), `interval locks: ${locks}`);
  await S.p.screenshot({ path: `${OUT}/05-student-interval.png` });
  await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${L[1].id}/exercises`, null, 403, 'marathon_lesson_locked');
  ok('«По интервалам» (раз в 2 дня в 08:00): урок 1 открыт сразу, уроки 2–3 «Откроется … 08:00», прямой API — 403');

  // A student added later starts their own schedule: lesson 1 at once.
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S2_ID] }, 200);
  const me2 = await must(S2.p, 'GET', `/marathons/${m.id}/me`, null, 200);
  assert(me2.items[0].access === 'open' && me2.items[1].lock?.reason === 'scheduled', 'late student: lesson 1 open');
  ok('Ученик, добавленный позже: первый урок открыт сразу, дальше — своё расписание');

  // Changing only the schedule keeps the epoch.
  st = await openPassage();
  await st.getByTestId('schedule-fields').getByLabel(/^Время/).selectOption('10:00');
  await st.getByTestId('schedule-fields').getByRole('button', { name: 'Сохранить расписание' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Расписание сохранено' }).waitFor();
  await T.p.keyboard.press('Escape');
  info = await must(T.p, 'GET', `/classrooms/${m.id}`, null, 200);
  assert(info.marathon.open_time === '10:00' && info.marathon.progress_epoch === epoch, 'schedule saved without a new epoch');
  ok('«Сохранить расписание»: время изменено, эпоха прогресса та же');

  // ---- 3. «По дням недели»: Mondays at 07:00 ----
  await T.p.reload();
  await settle(T.p);
  st = await openPassage();
  await st.getByRole('radio', { name: /^По дням недели/ }).click();
  const wf = st.getByTestId('schedule-fields');
  assert(await wf.getByRole('button', { name: 'Сменить способ прохождения' }).isDisabled(), 'no weekday → disabled');
  await wf.getByRole('button', { name: 'Пн' }).click();
  await wf.getByLabel(/^Время/).selectOption('07:00');
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/06-weekdays-settings.png` });
  await wf.getByRole('button', { name: 'Сменить способ прохождения' }).click();
  await T.p.getByRole('dialog', { name: 'Смена способа прохождения' }).getByRole('button', { name: 'Продолжить' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Способ прохождения изменён' }).first().waitFor();
  info = await must(T.p, 'GET', `/classrooms/${m.id}`, null, 200);
  assert(info.marathon.mode === 'weekdays' && JSON.stringify(info.marathon.weekdays) === '[1]' && info.marathon.open_time === '07:00', 'weekdays saved');
  const mw = await must(S.p, 'GET', `/marathons/${m.id}/me`, null, 200);
  const w1 = new Date(mw.items[1].lock.opens_at);
  const local = new Intl.DateTimeFormat('en-US', { timeZone: info.marathon.timezone, weekday: 'short', hour: '2-digit', hourCycle: 'h23' }).formatToParts(w1);
  const part = (t) => local.find((x) => x.type === t)?.value;
  assert(part('weekday') === 'Mon' && part('hour') === '07', `lesson 2 on Monday 07:00 in the marathon zone: ${w1.toISOString()}`);
  await S.p.reload();
  await settle(S.p);
  await S.p.screenshot({ path: `${OUT}/07-student-weekdays.png` });
  ok('«По дням недели» (Пн, 07:00): урок 2 открывается в ближайший понедельник в 07:00 по поясу марафона');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
