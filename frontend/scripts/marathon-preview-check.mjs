// Marathon stage 8: «Режим предпросмотра» in the browser. The owner opens a lesson from the «Урок» tab, sees the
// preview banner, answers an instant test, takes a timed test to the end, comes back (re-entry keeps the preview
// attempts) and resets the preview. Student-facing data — statistics, rating, review counters, the student's
// lessons and answers, exercise statistics — is compared before and after: it must not change. Students cannot
// open the preview (page and API).
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e_s8 node scripts/marathon-preview-check.mjs
// Screenshots go to a new folder per run (SHOTS_DIR), never over the screenshots of earlier stages.
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e_s8';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/marathons/stage8-run-${new Date().toISOString().slice(0, 10)}`;
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
const noNow = (v) => JSON.stringify(v, (k, x) => (k === 'server_now' ? undefined : x));

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local').id;
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон предпросмотра ${RUN}`, direction: 'other' }, 201);
  const course = await must(T.p, 'POST', `/courses`, { title: `Курс предпросмотра ${RUN}` }, 201);
  const L = [];
  let timed;
  for (const [i, title] of ['Урок 1', 'Урок 2'].entries()) {
    const l = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title }, 201);
    const sec = await must(T.p, 'POST', `/lessons/${l.id}/sections`, { title: 'Section 1' }, 201);
    const ex = await must(T.p, 'POST', `/lessons/${l.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
      settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 5 },
      questions: [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] },
        { prompt: 'They ___ friends.', multiple: false, options: [{ label: 'are', is_correct: true }, { label: 'is', is_correct: false }] }] }, 201);
    if (i === 0) {
      timed = await must(T.p, 'POST', `/lessons/${l.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'На время',
        settings: { check_mode: 'after', scoring_enabled: true, points_per_correct: 1, timer_enabled: true, timer_seconds: 600 },
        questions: [{ prompt: 'I ___ here.', multiple: false, options: [{ label: 'am', is_correct: true }, { label: 'is', is_correct: false }] }] }, 201);
    }
    await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: l.id }, 200);
    L.push({ id: l.id, ex: ex.id, title });
  }
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID] }, 200);
  await must(T.p, 'POST', `/marathons/${m.id}/mode`, { mode: 'progressive', expected_epoch: 1, confirm_reset: true }, 200);
  await must(T.p, 'PUT', `/marathons/${m.id}/review-settings`, { teacher_check: true }, 200);
  await must(T.p, 'PUT', `/marathons/${m.id}/stats-settings`, { stats_enabled: true, rating_enabled: true }, 200);
  // The student's real work on lesson 1: one right, one wrong, sent for review.
  const sv = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${L[0].ex}/attempt`, null, 200);
  for (const [qi, label] of ['is', 'is'].entries()) {
    const q = sv.version.questions[qi];
    await must(S.p, 'PUT', `/attempts/${sv.attempt.id}/answers/${q.id}`, { option_ids: [q.options.find((o) => o.label === label).id], check: true }, 200);
  }
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${L[0].id}/submit`, null, 200);
  const base = `${WEB}/cabinet/school/marathons/marathon/${m.id}`;

  const snapshot = async () => noNow(await Promise.all([
    call(T.p, 'GET', `/marathons/${m.id}/stats`), call(T.p, 'GET', `/marathons/${m.id}/rating`), call(S.p, 'GET', `/marathons/${m.id}/rating`),
    call(S.p, 'GET', `/marathons/${m.id}/me`), call(S.p, 'GET', `/marathons/${m.id}/me/results`), call(T.p, 'GET', `/marathons/${m.id}/lessons`),
    call(T.p, 'GET', `/marathons/${m.id}/students`), call(T.p, 'GET', `/classrooms?type=marathon`), call(T.p, 'GET', `/exercises/${L[0].ex}/stats`),
    call(T.p, 'GET', `/exercises/${timed.id}/stats`), call(T.p, 'GET', `/marathons/${m.id}/students/${S_ID}/lessons/${L[0].id}`),
    call(S.p, 'GET', `/classrooms/${m.id}/lessons/${L[0].id}/exercises?scope=lesson`),
  ]));
  const before = await snapshot();

  // ---- Owner: «Предпросмотр» from the lesson list, the banner ----
  await T.p.goto(`${base}/lessons`);
  await settle(T.p);
  await T.p.getByTestId('lesson-preview').first().click();
  await T.p.waitForURL(new RegExp(`/preview/${L[0].id}$`));
  await T.p.getByTestId('preview-banner').waitFor();
  await settle(T.p);
  await T.p.screenshot({ path: `${OUT}/01-preview-banner.png` });
  ok('«Урок» → «Предпросмотр»: страница урока с баннером «Режим предпросмотра», «Сбросить предпросмотр», «Выйти из предпросмотра»');

  // ---- Taking the lesson as a student ----
  const test = T.p.getByRole('article', { name: 'Тест' });
  await test.getByRole('radio', { name: 'is' }).first().click();
  await test.getByText('Верно').first().waitFor();
  const timedCard = T.p.getByRole('article', { name: 'На время' });
  await timedCard.getByRole('button', { name: 'Начать' }).click();
  await timedCard.getByRole('timer').waitFor();
  await timedCard.getByRole('radio', { name: 'am' }).click();
  await T.p.waitForTimeout(500);
  await timedCard.getByRole('button', { name: 'Завершить' }).click();
  await T.p.getByRole('dialog').getByRole('button', { name: 'Завершить' }).click();
  await timedCard.getByText('Затраченное время').waitFor();
  await settle(T.p);
  await timedCard.scrollIntoViewIfNeeded();
  await T.p.screenshot({ path: `${OUT}/02-preview-answers-and-timed-result.png` });
  ok('Предпросмотр: мгновенная проверка «Верно», тест на время с таймером, «Завершить» с подтверждением, итог попытки');

  // ---- Re-entry keeps the preview attempts ----
  await T.p.reload();
  await settle(T.p);
  await T.p.getByRole('article', { name: 'На время' }).getByText('Затраченное время').waitFor();
  await T.p.getByRole('article', { name: 'Тест' }).getByText('Верно').first().waitFor();
  const pv = await must(T.p, 'GET', `/preview/lessons/${L[0].id}/exercises`, null, 200);
  assert(pv.items.length === 2 && pv.items[0].attempt?.status === 'in_progress' && pv.items[1].attempt?.status === 'finished', `preview state: ${JSON.stringify(pv.items.map((x) => x.attempt?.status))}`);
  ok('Повторный вход: ответы и итог предпросмотра на месте');

  // ---- Isolation ----
  assert(await snapshot() === before, 'the preview changed student-facing data');
  const me = await must(S.p, 'GET', `/marathons/${m.id}/me`, null, 200);
  assert(me.items[0].status === 'submitted' && me.items[1].access === 'locked' && me.completed_lessons === 0, `student: ${JSON.stringify(me.items.map((x) => [x.status, x.access]))}`);
  await S.p.goto(`${base}/lesson/${L[0].id}`);
  await settle(S.p);
  assert(await S.p.getByTestId('finish-state').getAttribute('data-state') === 'submitted', 'student lesson state');
  await T.p.goto(`${base}/statistics`);
  await settle(T.p);
  assert((await T.p.getByTestId('stats-points').allInnerTexts()).join('|') === '5 / 21', `statistics: ${await T.p.getByTestId('stats-points').allInnerTexts()}`);
  ok('Изоляция: статистика (5 / 21), рейтинг, счётчики проверки, урок и ответы ученика, статистика упражнений не изменились; урок 2 у ученика закрыт');

  // ---- Rights: a student has no preview ----
  await must(S.p, 'GET', `/preview/lessons/${L[0].id}/exercises`, null, 403);
  await must(S.p, 'POST', `/preview/exercises/${L[0].ex}/attempt`, null, 403);
  await S.p.goto(`${base}/preview/${L[0].id}`);
  await S.p.waitForURL((u) => !u.pathname.includes('/preview/'));
  ok('Ученик: API предпросмотра — 403, страница предпросмотра уводит на страницу марафона');

  // ---- «Сбросить предпросмотр» ----
  await T.p.goto(`${base}/preview/${L[0].id}`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Сбросить предпросмотр' }).click();
  const rd = T.p.getByRole('dialog', { name: 'Сбросить предпросмотр' });
  await rd.waitFor();
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/03-preview-reset-confirm.png` });
  await rd.getByRole('button', { name: 'Сбросить предпросмотр' }).click();
  await T.p.getByRole('article', { name: 'На время' }).getByRole('button', { name: 'Начать' }).waitFor();
  const after = await must(T.p, 'GET', `/preview/lessons/${L[0].id}/exercises`, null, 200);
  assert(after.items.every((x) => !x.attempt) && after.items.every((x) => x.reset_count === 1), `after reset: ${JSON.stringify(after.items.map((x) => [x.attempt, x.reset_count]))}`);
  assert(await snapshot() === before, 'resetting the preview changed student-facing data');
  await settle(T.p);
  await T.p.screenshot({ path: `${OUT}/04-preview-after-reset.png` });
  ok('«Сбросить предпросмотр» с подтверждением: упражнения снова не начаты; данные учеников те же');

  // ---- Phone width ----
  await T.p.setViewportSize({ width: 390, height: 844 });
  await T.p.reload();
  await settle(T.p);
  const sw = await T.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(sw <= 0, `horizontal scroll on phone: ${sw}px`);
  await T.p.screenshot({ path: `${OUT}/05-phone-preview.png` });
  ok('Телефон 390 px: баннер и урок без горизонтальной прокрутки');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
