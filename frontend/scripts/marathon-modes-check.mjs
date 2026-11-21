// Marathon stage 5: "Дата открытия" of a lesson (open mode), the mode picker with the original's confirmation,
// "По мере прохождения", "Закрытый" with opening for everyone / one student, refusals by direct lesson URL and
// API, and answers/points kept across mode changes. "По интервалам" / "По дням недели" are shown unavailable.
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e node scripts/marathon-modes-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/modes';
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
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local')?.id;
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон режимов ${RUN}`, direction: 'other' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс режимов ${RUN}` }, 201);
  const L = [];
  for (const title of ['Урок 1', 'Урок 2']) {
    const l = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title }, 201);
    const sec = await must(T.p, 'POST', `/lessons/${l.id}/sections`, { title: 'Section 1' }, 201);
    const ex = await must(T.p, 'POST', `/lessons/${l.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
      settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 5 },
      questions: [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] }] }, 201);
    await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: l.id }, 200);
    L.push({ id: l.id, ex: ex.id, title });
  }
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID] }, 200);
  const studentsUrl = `${WEB}/cabinet/school/marathons/marathon/${m.id}/students`;
  const lessonsUrl = `${WEB}/cabinet/school/marathons/marathon/${m.id}/lessons`;
  const lesson2Url = `${WEB}/cabinet/school/marathons/marathon/${m.id}/lesson/${L[1].id}`;
  const openPassage = async () => {
    await T.p.getByRole('button', { name: 'Настройки' }).click();
    const st = T.p.getByRole('dialog', { name: 'Настройки марафона' });
    await st.getByRole('tab', { name: 'Прохождение' }).click();
    return st;
  };
  const pickMode = async (name) => {
    const p = await openPassage();
    await p.getByRole('radio', { name: new RegExp(`^${name}`) }).click();
    return p;
  };

  // ---- Open mode: «Дата открытия» of lesson 2 ----
  await T.p.goto(lessonsUrl);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Действия с уроком «Урок 2»' }).click();
  await T.p.getByRole('menuitem', { name: 'Настройки урока' }).click();
  const ls = T.p.getByRole('dialog', { name: 'Настройки урока' });
  await ls.getByRole('tab', { name: 'Дополнительно' }).click();
  await ls.getByRole('switch', { name: 'Дата открытия' }).click();
  await ls.getByText(/Урок откроется ученикам \d/).waitFor();
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/01-lesson-open-date.png` });
  await ls.getByRole('button', { name: 'Сохранить' }).click();
  await T.p.getByTestId('lesson-opens-at').waitFor();
  ok('Настройки урока → «Дополнительно» → «Дата открытия»: сохранено, у урока метка «Откроется …»');

  await S.p.goto(studentsUrl);
  await settle(S.p);
  let locks = await lockTexts(S.p);
  assert(locks[0] === '' && locks[1].startsWith('Откроется'), `student locks: ${locks}`);
  await S.p.screenshot({ path: `${OUT}/02-student-open-date.png` });
  await S.p.goto(lesson2Url);
  await settle(S.p);
  await S.p.getByText('Урок пока закрыт').waitFor();
  await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${L[1].ex}/attempt`, null, 403, 'marathon_lesson_locked');
  ok('Ученик: урок 2 «Откроется …», прямой адрес — «Урок пока закрыт», API — 403 marathon_lesson_locked');

  // ---- The mode picker: all five modes are available; the schedules show their parameters first ----
  await T.p.reload();
  await settle(T.p);
  let st = await openPassage();
  assert(await st.getByRole('radio', { name: /^По интервалам/ }).isEnabled(), 'interval enabled');
  assert(await st.getByRole('radio', { name: /^По дням недели/ }).isEnabled(), 'weekdays enabled');
  await T.p.keyboard.press('Escape');
  await must(T.p, 'PUT', `/marathons/${m.id}/lessons/${L[1].id}/open-at`, { open_at: null }, 200);
  ok('«По интервалам» и «По дням недели» доступны в выборе способа прохождения');

  // The student completes lesson 1 in the open mode (epoch 1).
  const v = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${L[0].ex}/attempt`, null, 200);
  const q = v.version.questions[0];
  await must(S.p, 'PUT', `/attempts/${v.attempt.id}/answers/${q.id}`, { option_ids: [q.options[0].id], check: true }, 200);
  await must(S.p, 'POST', `/attempts/${v.attempt.id}/finish`, null, 200);
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${L[0].id}/complete`, null, 200);
  const pointsBefore = await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${L[0].id}/progress`, null, 200);

  // ---- «По мере прохождения» with the original's confirmation ----
  await T.p.reload();
  await settle(T.p);
  await pickMode('По мере прохождения');
  const confirm = T.p.getByRole('dialog', { name: 'Смена способа прохождения' });
  await confirm.waitFor();
  await T.p.waitForTimeout(400);
  await T.p.screenshot({ path: `${OUT}/04-mode-change-confirm.png` });
  await confirm.getByRole('button', { name: 'Продолжить' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Способ прохождения изменён' }).waitFor();
  await T.p.keyboard.press('Escape');
  await settle(T.p);
  const me = await must(S.p, 'GET', `/marathons/${m.id}/me`, null, 200);
  assert(me.mode === 'progressive' && me.completed_lessons === 0, `statuses start over: ${JSON.stringify(me)}`);
  const pointsAfter = await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${L[0].id}/progress`, null, 200);
  assert(JSON.stringify(pointsAfter) === JSON.stringify(pointsBefore), 'answers and points kept');
  await S.p.goto(studentsUrl);
  await settle(S.p);
  locks = await lockTexts(S.p);
  assert(locks[0] === '' && locks[1] === 'Откроется после завершения предыдущего урока', `progressive locks: ${locks}`);
  await S.p.screenshot({ path: `${OUT}/05-student-progressive.png` });
  await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${L[1].ex}/attempt`, null, 403, 'marathon_lesson_locked');
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${L[0].id}/complete`, null, 200);
  await S.p.reload();
  await settle(S.p);
  locks = await lockTexts(S.p);
  assert(locks[1] === '', 'lesson 2 opens after lesson 1');
  ok('«По мере прохождения»: подтверждение, прогресс 0 из 2, баллы те же; урок 2 открывается после завершения урока 1');

  // ---- A date in progressive (Н3 = A) is reset only after the second confirmation when switching to «Закрытый» ----
  await must(T.p, 'PUT', `/marathons/${m.id}/lessons/${L[1].id}/open-at`, { open_at: new Date(Date.now() + 864e5).toISOString() }, 200);
  await T.p.reload();
  await settle(T.p);
  await pickMode('Закрытый');
  await T.p.getByRole('dialog', { name: 'Смена способа прохождения' }).getByRole('button', { name: 'Продолжить' }).click();
  const reset = T.p.getByRole('dialog', { name: 'Изменить способ прохождения' });
  await reset.waitFor();
  await T.p.waitForTimeout(400);
  await T.p.screenshot({ path: `${OUT}/03-dates-reset-confirm.png` });
  await reset.getByRole('button', { name: 'Изменить' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Способ прохождения изменён' }).first().waitFor();
  const afterReset = await must(T.p, 'GET', `/marathons/${m.id}/lessons`, null, 200);
  assert(afterReset.items.every((l) => l.open_at === null), 'dates reset after the confirmation');
  ok('Переход в «Закрытый» при датах: второе окно «Изменить способ прохождения» → «Изменить», даты сброшены');
  await T.p.keyboard.press('Escape');
  await T.p.goto(lessonsUrl);
  await settle(T.p);
  assert((await T.p.getByTestId('lesson-unlock-state').allInnerTexts()).join('|') === 'Закрыт|Закрыт', 'closed chips');
  await T.p.getByRole('button', { name: 'Действия с уроком «Урок 1»' }).click();
  await T.p.getByRole('menuitem', { name: 'Открыть всем ученикам' }).click();
  const ua = T.p.getByRole('dialog', { name: 'Открыть урок всем ученикам?' });
  await ua.waitFor();
  await T.p.waitForTimeout(400);
  await T.p.screenshot({ path: `${OUT}/06-unlock-for-all-confirm.png` });
  await ua.getByRole('button', { name: 'Открыть' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Урок открыт всем ученикам' }).waitFor();
  await settle(T.p);
  assert((await T.p.getByTestId('lesson-unlock-state').allInnerTexts()).join('|') === 'Открыт всем|Закрыт', 'lesson 1 open for all');
  await S.p.reload();
  await settle(S.p);
  locks = await lockTexts(S.p);
  assert(locks[0] === '' && locks[1] === 'Урок откроет преподаватель', `closed locks: ${locks}`);
  await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${L[1].id}/exercises`, null, 403, 'marathon_lesson_locked');

  await T.p.goto(studentsUrl);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Прогресс ученика: Данияр Нурланов' }).click();
  const prog = T.p.getByTestId('student-progress');
  await prog.getByRole('button', { name: 'Открыть этому ученику' }).click();
  const us = T.p.getByRole('dialog', { name: 'Открыть урок этому ученику?' });
  await us.waitFor();
  await us.getByRole('button', { name: 'Открыть' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Урок открыт ученику' }).waitFor();
  await settle(T.p);
  await T.p.screenshot({ path: `${OUT}/07-progress-after-unlock.png` });
  await S.p.reload();
  await settle(S.p);
  locks = await lockTexts(S.p);
  assert(locks[1] === '', 'lesson 2 open for this student');
  await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${L[1].id}/exercises`, null, 200);
  ok('«Закрытый»: всё закрыто; «Открыть всем ученикам» и «Открыть этому ученику» с подтверждением; прямой API следует решению сервера');

  // ---- Phone width ----
  await T.p.setViewportSize({ width: 390, height: 844 });
  await T.p.keyboard.press('Escape');
  await T.p.goto(studentsUrl);
  await settle(T.p);
  await openPassage();
  await settle(T.p);
  const sw = await T.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(sw <= 0, `horizontal scroll on phone: ${sw}px`);
  await T.p.screenshot({ path: `${OUT}/08-phone-modes.png` });
  ok('Телефон 390 px: выбор режима без горизонтальной прокрутки');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
