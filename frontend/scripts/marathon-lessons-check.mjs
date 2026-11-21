// Marathon stage 2: lessons inside a marathon — create, settings, order, copy, "Редактировать" with the way back,
// removal from the marathon (template and progress kept) and the server-side lock on a removed lesson.
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e node scripts/marathon-lessons-check.mjs
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/lessons';
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
const must = async (p, method, path, body, want) => {
  const r = await call(p, method, path, body);
  assert(r.status === want, `${method} ${path}: ${r.status} ${JSON.stringify(r.body)}`);
  return r.body;
};
const titles = (p) => p.getByTestId('marathon-lessons').locator('li p.font-medium').allInnerTexts();
const menu = async (p, title, item) => {
  await p.getByRole('button', { name: `Действия с уроком «${title}»` }).click();
  await p.getByRole('menuitem', { name: item }).click();
};

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');

  // ---- Setup through the API: marathon, a personal lesson with a test, one student ----
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local')?.id;
  assert(S_ID, 'student@ortalyq.local is assigned to the teacher');
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон уроков ${RUN}`, direction: 'other', cover_color: '#9254DE' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Личный курс ${RUN}` }, 201);
  const personal = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title: 'Глагол to be' }, 201);
  const sec = await must(T.p, 'POST', `/lessons/${personal.id}/sections`, { title: 'Section 1' }, 201);
  const ex = await must(T.p, 'POST', `/lessons/${personal.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
    settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 5 },
    questions: [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] }] }, 201);
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID] }, 200);
  const url = `${WEB}/cabinet/school/marathons/marathon/${m.id}/lessons`;

  // ---- Empty tab → "Добавить урок" chooser → create a new lesson ----
  await T.p.goto(url);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить урок' }).first().click();
  const chooser = T.p.getByRole('dialog', { name: 'Добавить урок' });
  await chooser.getByRole('button', { name: 'Создать новый урок' }).waitFor();
  await T.p.waitForTimeout(400); // let the dialog finish its opening animation
  await T.p.screenshot({ path: `${OUT}/01-add-lesson-chooser.png` });
  await chooser.getByRole('button', { name: 'Создать новый урок' }).click();
  const create = T.p.getByRole('dialog', { name: 'Создать новый урок' });
  assert(await create.getByRole('button', { name: 'Создать' }).isDisabled(), '"Создать" is disabled without a title');
  await create.getByLabel('Название').fill('Present Simple');
  await create.getByLabel('Описание').fill('Утвердительные предложения');
  await create.getByRole('button', { name: 'Цвет обложки #16C784' }).click();
  await T.p.waitForTimeout(400); // let the dialog finish its opening animation
  await T.p.screenshot({ path: `${OUT}/02-create-lesson.png` });
  await create.getByRole('button', { name: 'Создать' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Урок создан и добавлен в марафон' }).waitFor();
  assert((await titles(T.p)).join('|') === 'Present Simple', 'new lesson listed');
  assert(sql(`SELECT co.title FROM marathon_settings ms JOIN courses co ON co.id = ms.course_id WHERE ms.classroom_id = '${m.id}'`) === `Марафон уроков ${RUN}`,
    'own course of the marathon');
  ok('Добавить урок → выбор способа → «Создать новый урок»; урок в собственном курсе марафона');

  // ---- A personal lesson (picker is covered by marathon-finished-check), then order ----
  await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: personal.id }, 200);
  await T.p.reload();
  await settle(T.p);
  assert((await titles(T.p)).join('|') === 'Present Simple|Глагол to be', 'two lessons');
  await menu(T.p, 'Глагол to be', 'Переместить выше');
  await settle(T.p);
  assert((await titles(T.p)).join('|') === 'Глагол to be|Present Simple', 'moved up');
  await T.p.reload();
  await settle(T.p);
  assert((await titles(T.p)).join('|') === 'Глагол to be|Present Simple', 'order kept after reload');
  ok('Порядок: «Переместить выше», сохраняется после обновления страницы');

  // ---- Lesson settings ----
  await menu(T.p, 'Present Simple', 'Настройки урока');
  const st = T.p.getByRole('dialog', { name: 'Настройки урока' });
  assert(await st.getByLabel('Название').inputValue() === 'Present Simple', 'settings prefilled');
  await st.getByLabel('Название').fill('Present Simple (A1)');
  await T.p.waitForTimeout(400); // let the dialog finish its opening animation
  await T.p.screenshot({ path: `${OUT}/03-lesson-settings.png` });
  await st.getByRole('tab', { name: 'Дополнительно' }).click();
  // Since stage 5 the open mode has «Дата открытия» here (see marathon-modes-check.mjs).
  assert(await st.getByRole('switch', { name: 'Дата открытия' }).isVisible(), 'opening date in the open mode');
  await st.getByRole('tab', { name: 'Основное' }).click();
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await settle(T.p);
  assert((await titles(T.p)).join('|') === 'Глагол to be|Present Simple (A1)', 'renamed');
  ok('Настройки урока: название сохраняется, во вкладке «Дополнительно» — «Дата открытия»');

  // ---- Copy ----
  await menu(T.p, 'Глагол to be', 'Копировать');
  await T.p.getByRole('status').filter({ hasText: 'Копия урока добавлена в конец списка' }).waitFor();
  await settle(T.p);
  assert((await titles(T.p)).join('|') === 'Глагол to be|Present Simple (A1)|Глагол to be (копия)', 'copy appended');
  const copyId = sql(`SELECT ml.lesson_id FROM marathon_lessons ml JOIN lessons l ON l.id = ml.lesson_id
    WHERE ml.classroom_id = '${m.id}' AND l.title = 'Глагол to be (копия)'`);
  assert(sql(`SELECT count(*) FROM exercises WHERE lesson_id = '${copyId}' AND copied_from_id = '${ex.id}'`) === '1', 'exercise copied');
  ok('Копировать: копия в конце списка, упражнение скопировано');

  // ---- "Редактировать" opens the editor with the way back ----
  await T.p.getByRole('link', { name: 'Редактировать' }).first().click();
  await T.p.waitForURL(new RegExp(`/materials/lessons/${personal.id}\\?marathon=${m.id}`));
  await settle(T.p);
  await T.p.screenshot({ path: `${OUT}/04-editor-back-to-marathon.png` });
  await T.p.getByRole('link', { name: 'К марафону' }).click();
  await T.p.waitForURL(new RegExp(`/marathon/${m.id}/lessons$`));
  ok('«Редактировать» → редактор материалов → «К марафону»');

  // ---- Student works on the personal lesson, then the teacher removes it ----
  const v = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex.id}/attempt`, null, 200);
  const q = v.version.questions[0];
  await must(S.p, 'PUT', `/attempts/${v.attempt.id}/answers/${q.id}`, { option_ids: [q.options[0].id], check: true }, 200);
  await must(S.p, 'POST', `/attempts/${v.attempt.id}/finish`, null, 200);
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${personal.id}/complete`, null, 200);

  await settle(T.p);
  await menu(T.p, 'Глагол to be', 'Удалить');
  const confirm = T.p.getByRole('dialog', { name: 'Удалить урок из марафона?' });
  await confirm.waitFor();
  await T.p.waitForTimeout(400); // let the dialog finish its opening animation
  await T.p.screenshot({ path: `${OUT}/05-remove-confirmation.png` });
  await confirm.getByRole('button', { name: 'Удалить' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Урок удалён из марафона' }).waitFor();
  await settle(T.p);
  assert((await titles(T.p)).join('|') === 'Present Simple (A1)|Глагол to be (копия)', 'removed from the list');
  await T.p.screenshot({ path: `${OUT}/06-after-remove.png` });
  await must(T.p, 'GET', `/lessons/${personal.id}`, null, 200);
  assert(sql(`SELECT count(*) FROM attempts WHERE classroom_id = '${m.id}' AND exercise_id = '${ex.id}'`) === '1', 'attempt kept');
  assert(sql(`SELECT count(*) FROM marathon_lesson_progress WHERE classroom_id = '${m.id}' AND lesson_id = '${personal.id}'`) === '1', 'progress kept');
  ok('Удалить: урок пропал из марафона, шаблон, попытка и прогресс сохранены');

  // Server lock on the removed lesson: list, direct lesson URL and the attempts API.
  const sl = await must(S.p, 'GET', `/marathons/${m.id}/lessons`, null, 200);
  assert(sl.total_lessons === 2 && !sl.items.some((l) => l.lesson_id === personal.id), 'student list without the removed lesson');
  await must(S.p, 'GET', `/classrooms/${m.id}/lessons/${personal.id}/exercises`, null, 404);
  await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex.id}/attempt`, null, 404);
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${personal.id}/complete`, null, 404);
  await S.p.goto(`${WEB}/cabinet/school/marathons/marathon/${m.id}/lesson/${personal.id}`);
  await settle(S.p);
  await S.p.screenshot({ path: `${OUT}/07-student-removed-lesson-direct-url.png` });
  ok('Ученик: удалённого урока нет в списке, прямой адрес и API попыток отвечают 404');

  // Adding it again restores the progress.
  await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: personal.id }, 200);
  const back = await must(S.p, 'GET', `/marathons/${m.id}/lessons`, null, 200);
  assert(back.completed_lessons === 1, 'progress restored');
  ok('Повторное добавление возвращает урок с прогрессом ученика');

  // ---- Phone width: no horizontal scroll ----
  await T.p.setViewportSize({ width: 390, height: 844 });
  await T.p.reload();
  await settle(T.p);
  const sw = await T.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(sw <= 0, `horizontal scroll on phone: ${sw}px`);
  await T.p.screenshot({ path: `${OUT}/08-phone.png` });
  ok('Телефон 390 px: без горизонтальной прокрутки');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
