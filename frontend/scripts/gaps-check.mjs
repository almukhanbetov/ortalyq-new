// «Слова и пропуски» in the browser. Teacher: lesson editor → «Добавить упражнение» → «Слова и пропуски» → the phrase
// «I ___ a student.» and the answer «am» → save → reload → edit (a new version) → reload. Student in the class: the
// phrase with an empty gap and no answer in any API response before the check → « AM » → «Проверить» → «Верно»
// (case and spaces around do not matter) → reload keeps it; a wrong word shows the accepted one only after the check.
// Teacher sees the student's word. Plus: no console errors, no external requests.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/gaps-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/gaps/check-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });

async function login(email, password) {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 4(01|03|04|09|22)/.test(m.text())) errors.push(m.text()); });
  p.on('request', (r) => { const u = new URL(r.url()); if (/^https?:$/.test(u.protocol) && !['localhost', '127.0.0.1'].includes(u.hostname)) external.add(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль', { exact: true }).fill(password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL((u) => !u.pathname.startsWith('/login'));
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return { ctx, p };
}
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(300));

try {
  const users = await makeUsers(stack, RUN, { students: ['Данияр'] });
  const [SU] = users.students;
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const course = await api.must('POST', '/courses', { title: `English ${RUN}` }, 201);
  const lesson = await api.must('POST', `/courses/${course.id}/lessons`, { title: 'Урок 1. To be' }, 201);
  await api.must('POST', `/lessons/${lesson.id}/sections`, { title: 'Практика' }, 201);
  const cls = await api.must('POST', '/classrooms', { name: `Пропуски ${RUN}`, student_id: SU.id }, 201);

  // ---- 1. Teacher creates the exercise in the lesson editor ----
  const T = await login(users.teacher.email, users.teacher.password);
  const tp = T.p;
  await tp.goto(`${WEB}/materials/lessons/${lesson.id}`);
  await tp.getByRole('button', { name: 'Добавить упражнение' }).first().click();
  const catalog = tp.getByRole('dialog', { name: 'Добавить упражнение' });
  const gapsBtn = catalog.getByRole('button', { name: 'Слова и пропуски' });
  assert(await gapsBtn.isEnabled(), '«Слова и пропуски» is enabled in the catalog');
  await tp.screenshot({ path: `${OUT}/01-catalog.png` });
  await gapsBtn.click();
  const ed = tp.getByRole('dialog', { name: 'Новое задание «Слова и пропуски»' });
  await ed.getByLabel('Заголовок').fill('To be: пропуск');
  // Client validation: a phrase without a gap is refused before any request.
  await ed.getByLabel('Фраза с пропуском').fill('I am a student.');
  await ed.getByLabel('Правильный ответ').fill('am');
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.getByText('Отметьте пропуск: ___').waitFor();
  await ed.getByLabel('Фраза с пропуском').fill('I ___ a student.');
  await ed.getByText('Так увидит ученик').waitFor();
  await tp.screenshot({ path: `${OUT}/02-editor.png` });
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.waitFor({ state: 'detached' });
  const row = tp.locator('li.bg-surface').filter({ hasText: 'To be: пропуск' });
  await row.waitFor();
  assert((await row.innerText()).includes('I ___ a student.') && (await row.innerText()).includes('v1'), 'row shows the phrase, v1');
  ok('teacher creates «Слова и пропуски»: I ___ a student. / am (a phrase without a gap is refused)');

  // ---- 2. Reload, edit: a new version; reload keeps it ----
  await tp.reload();
  await row.getByRole('button', { name: /To be: пропуск/ }).click();
  const edit = tp.getByRole('dialog', { name: 'Редактирование задания «Слова и пропуски»' });
  assert((await edit.getByLabel('Фраза с пропуском').inputValue()) === 'I ___ a student.', 'phrase prefilled');
  assert((await edit.getByLabel('Правильный ответ').inputValue()) === 'am', 'answer prefilled');
  await edit.getByLabel('Заголовок').fill('To be: впишите слово');
  await edit.getByRole('button', { name: 'Сохранить' }).click();
  await edit.waitFor({ state: 'detached' });
  await tp.reload();
  const row2 = tp.locator('li.bg-surface').filter({ hasText: 'To be: впишите слово' });
  await row2.waitFor();
  assert((await row2.innerText()).includes('v2'), 'edit made version 2');
  await tp.screenshot({ path: `${OUT}/03-lesson-editor.png` });
  ok('editing saves a new version (v2) and survives a reload');

  // A second exercise for the wrong-answer case (made through the API, the editor is covered above).
  const sections = (await api.must('GET', `/lessons/${lesson.id}`, undefined, 200)).sections;
  await api.must('POST', `/lessons/${lesson.id}/exercises`, { scope: 'lesson', section_id: sections[0].id, kind: 'gaps', title: 'She: пропуск',
    settings: { check_mode: 'instant' }, questions: [{ prompt: 'She ___ happy.', options: [{ label: 'is', is_correct: true }] }] }, 201);
  await api.must('PUT', `/classrooms/${cls.id}/current-lesson`, { lesson_id: lesson.id }, 200);

  // ---- 3. Student: the answer is not sent before the check ----
  const S = await login(SU.email, SU.password);
  const sp = S.p;
  const bodies = [];
  sp.on('response', async (r) => { if (r.url().includes('/api/') && (r.headers()['content-type'] ?? '').includes('json')) bodies.push(await r.text().catch(() => '')); });
  await sp.goto(`${WEB}/classes/${cls.id}`);
  const ex1 = sp.getByRole('article').filter({ hasText: 'To be: впишите слово' });
  await ex1.waitFor();
  await settle(sp);
  const leaked = bodies.filter((x) => /"label":"am"|"correct_answers"|"is_correct"/.test(x));
  assert(leaked.length === 0, `key in a response before the check: ${leaked[0]?.slice(0, 200)}`);
  const gap = ex1.getByRole('textbox', { name: 'Пропуск: впишите слово' });
  assert(await gap.isVisible(), 'gap input');
  assert((await ex1.innerText()).includes('I') && (await ex1.innerText()).includes('a student.'), 'phrase around the gap');
  await sp.screenshot({ path: `${OUT}/04-student-before.png` });
  ok('student sees «I [ ] a student.»; no answer key in any API response before the check');

  // ---- 4. « AM » → «Проверить» → «Верно»; reload keeps it ----
  await gap.fill('  AM ');
  await ex1.getByRole('button', { name: 'Проверить' }).click();
  await ex1.getByText('Верно').waitFor();
  await sp.screenshot({ path: `${OUT}/05-student-correct.png` });
  await sp.reload();
  await ex1.getByText('Верно').waitFor();
  assert((await ex1.getByLabel('Ответ ученика').innerText()).trim() === 'AM', 'saved word shown trimmed after reload');
  assert((await ex1.getByRole('textbox').count()) === 0, 'checked gap is read-only');
  ok('« AM » is accepted as «am» (case and spaces around ignored); the result survives a reload');

  // ---- 5. Wrong word: the accepted one appears only after the check ----
  const ex2 = sp.getByRole('article').filter({ hasText: 'She: пропуск' });
  assert(!(await ex2.innerText()).includes('Правильный ответ'), 'no key before the check');
  await ex2.getByRole('textbox', { name: 'Пропуск: впишите слово' }).fill('are');
  await ex2.getByRole('textbox').press('Enter');
  await ex2.getByText('Неверно').waitFor();
  assert((await ex2.innerText()).includes('Правильный ответ: is'), 'key after the check');
  await sp.screenshot({ path: `${OUT}/06-student-wrong.png` });
  ok('a wrong word is marked «Неверно» and only then shows «Правильный ответ: is»');

  // ---- 6. Teacher sees the student's word in the class ----
  await tp.goto(`${WEB}/classes/${cls.id}`);
  const tex = tp.getByRole('article').filter({ hasText: 'To be: впишите слово' });
  await tex.getByText('Верно').waitFor();
  assert((await tex.getByLabel('Ответ ученика').innerText()).trim() === 'AM', 'teacher sees the word');
  await tp.screenshot({ path: `${OUT}/07-teacher-class.png` });
  ok('teacher sees the student\'s word and the verdict in the class');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
