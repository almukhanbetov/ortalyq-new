// Marathon stage 3: manual deactivation/activation of students (row menu → selection → confirmation), the
// progress window of a deactivated student, removal from the marathon, and the server-side lock (stage 4
// requirement): a deactivated student is refused in the list, by a direct lesson URL and in the attempts API,
// including an attempt started before; after activation everything is back with answers and progress.
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e node scripts/marathon-students-check.mjs
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/students';
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
  // 401 before login and the expected 403/404 refusals are not page errors.
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
const reasonOf = (p, id) => p.locator(`[data-student="${id}"] [data-testid="student-status"]`).getAttribute('data-reason');

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const T2 = await login('teacher2@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');
  const S2 = await login('student2@ortalyq.local', 'student-local-123');

  // ---- Setup through the API ----
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local')?.id;
  const S2_ID = mine.items.find((u) => u.email === 'student2@ortalyq.local')?.id;
  assert(S_ID && S2_ID, 'both demo students are assigned to the teacher');
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон учеников ${RUN}`, direction: 'other' }, 201);
  await must(T.p, 'PATCH', `/marathons/${m.id}/settings`, { end_at: new Date(Date.now() + 3 * 864e5).toISOString() }, 200);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс учеников ${RUN}` }, 201);
  const lesson = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title: 'Глагол to be' }, 201);
  const sec = await must(T.p, 'POST', `/lessons/${lesson.id}/sections`, { title: 'Section 1' }, 201);
  const mk = () => must(T.p, 'POST', `/lessons/${lesson.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
    settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 5 },
    questions: [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] }] }, 201);
  const ex1 = await mk();
  const ex2 = await mk();
  await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: lesson.id }, 200);
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID, S2_ID] }, 200);
  // Student: exercise 1 finished, lesson completed, exercise 2 started and left in progress.
  const v1 = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex1.id}/attempt`, null, 200);
  const q1 = v1.version.questions[0];
  await must(S.p, 'PUT', `/attempts/${v1.attempt.id}/answers/${q1.id}`, { option_ids: [q1.options[0].id], check: true }, 200);
  await must(S.p, 'POST', `/attempts/${v1.attempt.id}/finish`, null, 200);
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${lesson.id}/complete`, null, 200);
  const v2 = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex2.id}/attempt`, null, 200);
  const q2 = v2.version.questions[0];
  const pointsBefore = await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${lesson.id}/progress`, null, 200);
  const url = `${WEB}/cabinet/school/marathons/marathon/${m.id}/students`;

  // ---- Another teacher cannot manage the marathon ----
  await must(T2.p, 'PUT', `/marathons/${m.id}/students/active`, { student_ids: [S_ID], active: false }, 404);
  await must(T2.p, 'DELETE', `/marathons/${m.id}/students/${S_ID}`, null, 404);
  await must(S.p, 'PUT', `/marathons/${m.id}/students/active`, { student_ids: [S_ID], active: false }, 403);
  ok('Чужой учитель получает 404, ученик — 403 на управление статусом и удаление');

  // ---- Row menu → «Деактивировать» → selection of both → confirmation ----
  await T.p.goto(url);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Действия с учеником Данияр Нурланов' }).click();
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/01-row-menu.png` });
  await T.p.getByRole('menuitem', { name: 'Деактивировать' }).click();
  const panel = T.p.getByRole('region', { name: 'Действие с выбранными учениками' });
  assert(await panel.getByRole('button', { name: 'Деактивировать 1' }).isVisible(), 'the student is preselected');
  await T.p.getByText('Выбрать всех').click();
  await panel.getByRole('button', { name: 'Деактивировать 2' }).waitFor();
  await T.p.screenshot({ path: `${OUT}/02-selection-mode.png` });
  await panel.getByRole('button', { name: 'Деактивировать 2' }).click();
  const deact = T.p.getByRole('dialog', { name: 'Деактивация' });
  await deact.waitFor();
  await T.p.waitForTimeout(400);
  await T.p.screenshot({ path: `${OUT}/03-deactivation-confirm.png` });
  await deact.getByRole('button', { name: 'Деактивировать' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Деактивировано учеников: 2' }).waitFor();
  await settle(T.p);
  assert(await reasonOf(T.p, S_ID) === 'manual' && await reasonOf(T.p, S2_ID) === 'manual', 'both rows show the manual deactivation');
  await T.p.screenshot({ path: `${OUT}/04-deactivated-list.png` });
  await T.p.reload();
  await settle(T.p);
  assert(await reasonOf(T.p, S_ID) === 'manual', 'kept after reload');
  const by = sql(`SELECT u.email FROM marathon_enrollments e JOIN users u ON u.id = e.deactivated_by WHERE e.classroom_id = '${m.id}' AND e.student_id = '${S_ID}'`);
  assert(by === 'teacher@ortalyq.local', `author stored: ${by}`);
  ok('Меню строки → «Деактивировать» → «Выбрать всех» → «Деактивировать 2» → подтверждение; статус и автор сохраняются после обновления');

  // ---- The deactivated student is refused: list, direct lesson URL, attempts API ----
  await S.p.goto(`${WEB}/cabinet/school/marathons/marathon/${m.id}/students`);
  await settle(S.p);
  await S.p.getByText('Преподаватель закрыл вам доступ к этому марафону').first().waitFor();
  await S.p.screenshot({ path: `${OUT}/05-student-deactivated-page.png` });
  await S.p.goto(`${WEB}/cabinet/school/marathons/marathon/${m.id}/lesson/${lesson.id}`);
  await settle(S.p);
  await S.p.getByText('Преподаватель закрыл вам доступ к этому марафону').first().waitFor();
  await S.p.screenshot({ path: `${OUT}/06-student-direct-lesson-url.png` });
  await must(S.p, 'GET', `/marathons/${m.id}/lessons`, null, 403, 'marathon_deactivated');
  await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${lesson.id}/exercises`, null, 403, 'marathon_deactivated');
  await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex1.id}/attempt`, null, 403, 'marathon_deactivated');
  await must(S.p, 'PUT', `/attempts/${v2.attempt.id}/answers/${q2.id}`, { option_ids: [q2.options[0].id] }, 403, 'marathon_deactivated');
  await must(S.p, 'POST', `/attempts/${v2.attempt.id}/finish`, null, 403, 'marathon_deactivated');
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${lesson.id}/complete`, null, 403, 'marathon_deactivated');
  assert(sql(`SELECT status FROM attempts WHERE id = '${v2.attempt.id}'`) === 'in_progress', 'started attempt untouched');
  ok('Ученик: страница марафона и прямой адрес урока закрыты; список, старт, ответ и завершение начатой попытки — 403 marathon_deactivated');

  // ---- Progress window of the deactivated student → «Активировать» with confirmation ----
  await T.p.getByRole('button', { name: 'Прогресс ученика: Данияр Нурланов' }).click();
  const prog = T.p.getByTestId('student-progress');
  await prog.getByTestId('no-access-banner').waitFor();
  const banner = await prog.getByTestId('deactivation-banner').innerText();
  assert(/Ученик деактивирован .*Причина: Решение преподавателя \(Айгерим Сапарова\)/.test(banner), `banner: ${banner}`);
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/07-progress-deactivated.png` });
  await prog.getByRole('button', { name: 'Активировать' }).click();
  const act = T.p.getByRole('dialog', { name: 'Активация' });
  await act.waitFor();
  await T.p.waitForTimeout(400);
  await T.p.screenshot({ path: `${OUT}/08-activation-confirm.png` });
  await act.getByRole('button', { name: 'Активировать' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Активировано учеников: 1' }).waitFor();
  await settle(T.p);
  assert(await prog.getByTestId('no-access-banner').count() === 0, 'no-access banner gone');
  await T.p.keyboard.press('Escape');
  await settle(T.p);
  assert(await reasonOf(T.p, S_ID) === '' && await reasonOf(T.p, S2_ID) === 'manual', 'only the first student activated');
  ok('Окно прогресса: дата, причина с именем преподавателя, баннер «нет доступа» → «Активировать» → подтверждение');

  // ---- After activation: access, answers, points and progress are back ----
  const back = await must(S.p, 'GET', `/marathons/${m.id}/lessons`, null, 200);
  assert(back.completed_lessons === 1, 'progress kept');
  const pointsAfter = await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${lesson.id}/progress`, null, 200);
  assert(JSON.stringify(pointsAfter) === JSON.stringify(pointsBefore), 'points and answers kept');
  await must(S.p, 'PUT', `/attempts/${v2.attempt.id}/answers/${q2.id}`, { option_ids: [q2.options[0].id] }, 200);
  await must(S.p, 'POST', `/attempts/${v2.attempt.id}/finish`, null, 200);
  await S.p.goto(`${WEB}/cabinet/school/marathons/marathon/${m.id}/students`);
  await settle(S.p);
  await S.p.getByTestId('student-lessons').waitFor();
  await S.p.screenshot({ path: `${OUT}/09-student-after-activation.png` });
  ok('После активации: список уроков, прогресс 1 из 1, баллы и ответы те же, начатая попытка продолжается');

  // ---- Remove the second student from the marathon ----
  await T.p.getByRole('button', { name: 'Действия с учеником Алия Ермекова' }).click();
  await T.p.getByRole('menuitem', { name: 'Удалить из марафона' }).click();
  const rm = T.p.getByRole('dialog', { name: 'Удалить ученика из марафона?' });
  await rm.waitFor();
  await T.p.waitForTimeout(400);
  await T.p.screenshot({ path: `${OUT}/10-remove-confirm.png` });
  await rm.getByRole('button', { name: 'Удалить из марафона' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Ученик удалён из марафона' }).waitFor();
  await settle(T.p);
  assert(await T.p.locator(`[data-student="${S2_ID}"]`).count() === 0, 'removed from the list');
  await must(S2.p, 'GET', `/classrooms/${m.id}`, null, 404);
  await must(S2.p, 'GET', `/marathons/${m.id}/lessons`, null, 404);
  assert(sql(`SELECT removed_by IS NOT NULL FROM marathon_enrollments WHERE classroom_id = '${m.id}' AND student_id = '${S2_ID}'`) === 't', 'removal stored');
  ok('Удалить из марафона: подтверждение, ученик пропал из списка и получает 404');

  // ---- Phone width in selection mode ----
  await T.p.setViewportSize({ width: 390, height: 844 });
  await T.p.getByRole('button', { name: 'Действия с учеником Данияр Нурланов' }).click();
  await T.p.getByRole('menuitem', { name: 'Деактивировать' }).click();
  await settle(T.p);
  const sw = await T.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(sw <= 0, `horizontal scroll on phone: ${sw}px`);
  await T.p.screenshot({ path: `${OUT}/11-phone-selection.png` });
  await T.p.getByRole('region', { name: 'Действие с выбранными учениками' }).getByRole('button', { name: 'Отмена' }).click();
  ok('Телефон 390 px: режим выбора без горизонтальной прокрутки, «Отмена» выходит из режима');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
