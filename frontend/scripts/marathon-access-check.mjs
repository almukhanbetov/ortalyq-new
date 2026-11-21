// Marathon stage 4: delayed start and auto-deactivation set in "Настройки → Прохождение", the student's page from
// GET /me, the direct lesson URL and the attempts API refused by the server, and progress kept after access returns.
// It CREATES data and MOVES an activation date with SQL, so run it only against an isolated stack with its own DB:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e node scripts/marathon-access-check.mjs
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/access';
mkdirSync(OUT, { recursive: true });

const sql = (q) => execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'ortalyq_new', '-d', DB, '-Atc', q], { cwd: '..' }).toString().trim();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const RUN = Date.now().toString(36);

const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
async function login(email, pw) {
  const ctx = await b.newContext({ viewport: { width: 1862, height: 961 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 40[134]/.test(m.text())) errors.push(m.text()); });
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

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local')?.id;
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон доступа ${RUN}`, direction: 'other' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс доступа ${RUN}` }, 201);
  const lesson = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title: 'Глагол to be' }, 201);
  const sec = await must(T.p, 'POST', `/lessons/${lesson.id}/sections`, { title: 'Section 1' }, 201);
  const mk = () => must(T.p, 'POST', `/lessons/${lesson.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
    settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 5 },
    questions: [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] }] }, 201);
  const ex1 = await mk();
  const ex2 = await mk();
  await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: lesson.id }, 200);
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID] }, 200);
  const page = `${WEB}/cabinet/school/marathons/marathon/${m.id}/students`;
  const lessonUrl = `${WEB}/cabinet/school/marathons/marathon/${m.id}/lesson/${lesson.id}`;
  const openPassage = async () => {
    await T.p.getByRole('button', { name: 'Настройки' }).click();
    const st = T.p.getByRole('dialog', { name: 'Настройки марафона' });
    await st.getByRole('tab', { name: 'Прохождение' }).click();
    return st;
  };

  // ---- Delayed start through the settings dialog (default suggestion: tomorrow 10:00) ----
  await T.p.goto(page);
  await settle(T.p);
  let st = await openPassage();
  await st.getByRole('switch', { name: 'Отложенный старт' }).click();
  await st.getByText(/Первый урок откроется участникам/).waitFor();
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/01-settings-delayed-start.png` });
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await settle(T.p);
  await T.p.getByTestId('marathon-starts-banner').waitFor();
  await T.p.screenshot({ path: `${OUT}/02-teacher-starts-banner.png` });
  ok('Настройки → «Отложенный старт»: сохранено, у учителя баннер «Марафон начнётся…»');

  await S.p.goto(page);
  await settle(S.p);
  assert(await S.p.getByTestId('marathon-starts-banner').innerText().then((x) => x.startsWith('Марафон начнётся')), 'student sees the start banner');
  assert(await S.p.getByTestId('student-lessons').getByRole('button', { name: 'Открыть урок' }).isDisabled(), 'lesson locked before start');
  await S.p.screenshot({ path: `${OUT}/03-student-not-started.png` });
  await S.p.goto(lessonUrl);
  await settle(S.p);
  await S.p.getByText('Урок ещё закрыт: марафон не начался').waitFor();
  const ns = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex1.id}/attempt`, null, 403, 'marathon_not_started');
  assert(ns.error.fields?.opens_at, 'opens_at in the refusal');
  const me0 = await must(S.p, 'GET', `/marathons/${m.id}/me`, null, 200);
  assert(me0.access.status === 'not_started' && me0.access.opens_at, 'me: not_started');
  ok('Ученик до старта: баннер «Марафон начнётся…», урок закрыт по прямому адресу, API — 403 marathon_not_started с opens_at');

  // ---- Start switched off → open; the student works ----
  st = await openPassage();
  await st.getByRole('switch', { name: 'Отложенный старт' }).click();
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await settle(T.p);
  const v1 = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex1.id}/attempt`, null, 200);
  const q1 = v1.version.questions[0];
  await must(S.p, 'PUT', `/attempts/${v1.attempt.id}/answers/${q1.id}`, { option_ids: [q1.options[0].id], check: true }, 200);
  await must(S.p, 'POST', `/attempts/${v1.attempt.id}/finish`, null, 200);
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${lesson.id}/complete`, null, 200);
  const v2 = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex2.id}/attempt`, null, 200);
  const q2 = v2.version.questions[0];
  const pointsBefore = await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${lesson.id}/progress`, null, 200);
  ok('Старт выключен: ученик проходит урок, вторая попытка оставлена незавершённой');

  // ---- Auto-deactivation 30 days ----
  st = await openPassage();
  await st.getByRole('switch', { name: 'Авто-деактивация' }).click();
  await st.getByLabel('Через, дней').fill('30');
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/04-settings-auto-deactivation.png` });
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await settle(T.p);
  await S.p.goto(page);
  await settle(S.p);
  assert(await S.p.getByTestId('student-access').innerText().then((x) => x.startsWith('Ваш доступ к марафону действует до')), 'student sees own deadline');
  await S.p.screenshot({ path: `${OUT}/05-student-access-until.png` });
  ok('«Авто-деактивация» 30 дней: ученик видит «Ваш доступ к марафону действует до …»');

  // The student was added 31 days ago (moved with SQL): the period is over.
  sql(`UPDATE marathon_enrollments SET activated_at = now() - interval '31 days' WHERE classroom_id = '${m.id}' AND student_id = '${S_ID}'`);
  await S.p.reload();
  await settle(S.p);
  assert(await S.p.getByTestId('student-access').innerText().then((x) => x.startsWith('Срок вашего доступа к марафону истёк')), 'expired notice');
  assert(await S.p.getByTestId('student-lessons').getByRole('button', { name: 'Открыть урок' }).isDisabled(), 'lesson locked after expiry');
  await S.p.screenshot({ path: `${OUT}/06-student-auto-expired.png` });
  await S.p.goto(lessonUrl);
  await settle(S.p);
  await S.p.getByText('Срок вашего доступа к марафону истёк').first().waitFor();
  await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex1.id}/attempt`, null, 403, 'marathon_auto_deactivated');
  await must(S.p, 'PUT', `/attempts/${v2.attempt.id}/answers/${q2.id}`, { option_ids: [q2.options[0].id] }, 403, 'marathon_auto_deactivated');
  await must(S.p, 'POST', `/attempts/${v2.attempt.id}/finish`, null, 403, 'marathon_auto_deactivated');
  assert(sql(`SELECT status FROM attempts WHERE id = '${v2.attempt.id}'`) === 'in_progress', 'started attempt untouched');
  await T.p.reload();
  await settle(T.p);
  const hint = await T.p.locator(`[data-student="${S_ID}"]`).innerText();
  assert(hint.includes('Авто-деактивация'), `teacher sees the reason: ${hint}`);
  await T.p.screenshot({ path: `${OUT}/07-teacher-auto-deactivated.png` });
  ok('Срок истёк: у ученика уведомление и закрытый урок; старт, ответ и завершение начатой попытки — 403; учитель видит «Авто-деактивация»');

  // ---- A longer period: access, progress, points and the started attempt are back ----
  st = await openPassage();
  await st.getByLabel('Через, дней').fill('60');
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await settle(T.p);
  const me = await must(S.p, 'GET', `/marathons/${m.id}/me`, null, 200);
  assert(me.access.status === 'open' && me.completed_lessons === 1, 'open again with progress');
  const pointsAfter = await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${lesson.id}/progress`, null, 200);
  assert(JSON.stringify(pointsAfter) === JSON.stringify(pointsBefore), 'points kept');
  await must(S.p, 'PUT', `/attempts/${v2.attempt.id}/answers/${q2.id}`, { option_ids: [q2.options[0].id] }, 200);
  await must(S.p, 'POST', `/attempts/${v2.attempt.id}/finish`, null, 200);
  await S.p.goto(page);
  await settle(S.p);
  assert(await S.p.getByTestId('student-lessons').getByRole('link', { name: 'Открыть урок' }).isVisible(), 'lesson opens again');
  ok('Срок увеличен до 60 дней: доступ, прогресс 1 из 1, баллы и начатая попытка вернулись');

  // ---- Phone width ----
  await S.p.setViewportSize({ width: 390, height: 844 });
  await S.p.reload();
  await settle(S.p);
  const sw = await S.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(sw <= 0, `horizontal scroll on phone: ${sw}px`);
  ok('Телефон 390 px: без горизонтальной прокрутки');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
