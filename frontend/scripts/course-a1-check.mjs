// «English A1 — Первые разговоры» in the browser. The course is loaded with `api seed-course <teacher>` for a teacher
// of this run (twice: the second load must change nothing). Teacher: «Материалы» → the course → a lesson in the editor
// → the class → «Выбрать урок» → Lesson 2. Student: the gap «I ___ from Almaty.» right (« AM »), a gap wrong (the key
// appears only after the check), the test right and wrong; everything survives a reload. Homework: the student fills
// the gap and the test and sends the work, the teacher accepts it. Plus: no console errors, no external requests.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 COURSE_DATABASE_URL=postgres://…/ortalyq_new_e2e \
//   SHOTS_DIR=… node scripts/course-a1-check.mjs
// With DEMO_TEACHER_PASSWORD it also signs in as teacher@ortalyq.local (read only) and shoots their «Материалы».
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const DB = process.env.COURSE_DATABASE_URL;
if (!DB || /\/ortalyq_new(\?|$)/.test(DB)) throw new Error('set COURSE_DATABASE_URL of the isolated stack database');
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/course-a1/check-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const COURSE = 'English A1 — Первые разговоры';
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();

const seedCourse = (email) => execFileSync('bash', ['-c', 'set -a && . ./.env.example && set +a && DATABASE_URL="$1" exec go run ./cmd/api seed-course "$2"', '_', DB, email],
  { cwd: '../backend', encoding: 'utf8' });

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
const article = (p, title) => p.getByRole('article').filter({ has: p.getByRole('heading', { name: title, exact: true }) });
/** Shows the room section that holds the exercise (the room shows one section at a time). */
async function show(p, title) {
  // A reload keeps the section the student was on, so rewind to the first one, then go forward.
  await p.getByRole('button', { name: 'Предыдущий раздел' }).waitFor();
  for (let i = 0; i < 4 && !(await article(p, title).isVisible()); i++) {
    const prev = p.getByRole('button', { name: 'Предыдущий раздел' });
    if (!(await prev.isVisible()) || !(await prev.isEnabled())) break;
    await prev.click();
    await settle(p);
  }
  for (let i = 0; i < 4 && !(await article(p, title).isVisible()); i++) {
    const next = p.getByRole('button', { name: 'Следующий раздел' });
    if (!(await next.isVisible())) break;
    await next.click();
    await settle(p);
  }
  await article(p, title).waitFor();
  return article(p, title);
}

try {
  const users = await makeUsers(stack, RUN, { students: ['Данияр'] });
  const [SU] = users.students;
  const first = seedCourse(users.teacher.email);
  assert(first.includes('created for') && (first.match(/\+ Lesson/g) ?? []).length === 3, `first load: ${first}`);
  const again = seedCourse(users.teacher.email);
  assert(again.includes('already in') && (again.match(/kept as is/g) ?? []).length === 3 && !again.includes('+ '), `second load: ${again}`);
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const courses = await api.must('GET', '/courses', undefined, 200);
  assert(courses.items.filter((c) => c.title === COURSE).length === 1, 'exactly one course after two loads');
  ok('seed-course: the first load creates the course with 3 lessons, the second one changes nothing');

  // ---- 1. Teacher: «Материалы» → the course → a lesson ----
  const T = await login(users.teacher.email, users.teacher.password);
  const tp = T.p;
  await tp.goto(`${WEB}/materials`);
  await tp.getByText(COURSE).first().waitFor();
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/01-materials.png` });
  await tp.getByText(COURSE).first().click();
  for (const l of ['Lesson 1. Hello!', 'Lesson 2. Where are you from?', 'Lesson 3. Meet my friend']) await tp.getByText(l).waitFor();
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/02-course.png`, fullPage: true });
  await tp.getByText('Lesson 2. Where are you from?').click();
  for (const s of ['Знакомство с темой', 'Практика', 'Разговор']) await tp.getByText(s, { exact: true }).first().waitFor();
  await tp.getByText('I ___ from Almaty.').waitFor();
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/03-lesson-editor.png`, fullPage: true });
  await tp.getByRole('tab', { name: 'Домашняя работа' }).click();
  await tp.getByText('Homework: answer the question').waitFor();
  await tp.getByText('Homework: choose the correct answer').waitFor();
  await tp.screenshot({ path: `${OUT}/04-lesson-homework.png` });
  ok('teacher sees the course in «Материалы», three lessons, three sections and the homework of Lesson 2');

  // ---- 2. Teacher opens the lesson in a test class ----
  const cls = await api.must('POST', '/classrooms', { name: `A1 ${RUN}`, student_id: SU.id }, 201);
  await tp.goto(`${WEB}/classes/${cls.id}`);
  await tp.getByRole('button', { name: 'Выбрать урок' }).click();
  const picker = tp.getByRole('dialog', { name: 'Другой урок' });
  await picker.locator('li').filter({ hasText: 'Lesson 2. Where are you from?' }).getByRole('button', { name: 'Открыть урок' }).click();
  await article(tp, 'Say where you are from').waitFor();
  await tp.screenshot({ path: `${OUT}/05-teacher-class.png` });
  ok('teacher opens Lesson 2 in the test class');

  // ---- 3. Student: gaps right and wrong ----
  const S = await login(SU.email, SU.password);
  const sp = S.p;
  const bodies = [];
  sp.on('response', async (r) => { if (r.url().includes('/api/') && (r.headers()['content-type'] ?? '').includes('json')) bodies.push(await r.text().catch(() => '')); });
  await sp.goto(`${WEB}/classes/${cls.id}`);
  let g1 = await show(sp, 'Say where you are from');
  await settle(sp);
  const leaked = bodies.filter((x) => /"is_correct"|"correct_answers"|"correct_option_ids"/.test(x));
  assert(leaked.length === 0, `key before the check: ${leaked[0]?.slice(0, 200)}`);
  await g1.getByRole('textbox', { name: 'Пропуск: впишите слово' }).fill('  AM ');
  await sp.screenshot({ path: `${OUT}/06-student-gap-typed.png` });
  await g1.getByRole('button', { name: 'Проверить' }).click();
  await g1.getByText('Верно').waitFor();
  assert(!(await g1.innerText()).includes('Правильный ответ'), 'no key shown for a right answer');
  await sp.screenshot({ path: `${OUT}/07-student-gap-correct.png` });

  let g2 = await show(sp, 'Ask the question');
  assert(!(await g2.innerText()).includes('Правильный ответ'), 'no key before the check');
  await g2.getByRole('textbox', { name: 'Пропуск: впишите слово' }).fill('is');
  await g2.getByRole('textbox').press('Enter');
  await g2.getByText('Неверно').waitFor();
  assert((await g2.innerText()).includes('Правильный ответ: are'), 'key after a wrong check');
  assert((await g2.getByRole('textbox').count()) === 0, 'a checked gap is read-only');
  await sp.screenshot({ path: `${OUT}/08-student-gap-wrong.png` });
  ok('gap: « AM » → «Верно»; «is» → «Неверно» and only then «Правильный ответ: are»; a checked gap is locked');

  // ---- 4. Student: the test right and wrong ----
  const test = await show(sp, 'Choose the best reply');
  const q = (prompt) => test.locator('li').filter({ hasText: prompt });
  await q('— Where are you from?').getByRole('radio', { name: "I'm from Kazakhstan." }).click();
  await q('— Where are you from?').getByText('Верно').waitFor();
  await q("— I'm from Rome. And you?").getByRole('radio', { name: 'Yes, I am.' }).click();
  await q("— I'm from Rome. And you?").getByText('Неверно').waitFor();
  assert((await q("— I'm from Rome. And you?").innerText()).includes('Правильный ответ'), 'test key after a wrong answer');
  await sp.screenshot({ path: `${OUT}/09-student-test.png` });
  ok('test: the right reply → «Верно», a wrong one → «Неверно» with the right option marked');

  // ---- 5. Reload keeps everything ----
  await sp.reload();
  g1 = await show(sp, 'Say where you are from');
  await g1.getByText('Верно').waitFor();
  assert((await g1.getByLabel('Ответ ученика').innerText()).trim() === 'AM', 'saved word after reload');
  g2 = await show(sp, 'Ask the question');
  await g2.getByText('Неверно').waitFor();
  const test2 = await show(sp, 'Choose the best reply');
  await test2.locator('li').filter({ hasText: '— Where are you from?' }).getByText('Верно').waitFor();
  await test2.locator('li').filter({ hasText: "— I'm from Rome. And you?" }).getByText('Неверно').waitFor();
  ok('after a reload the gap and test results are the same');

  // ---- 6. Homework: student fills and sends, teacher accepts ----
  await sp.getByRole('tab', { name: 'Домашняя работа' }).click();
  const hg = article(sp, 'Homework: answer the question');
  await hg.waitFor();
  await hg.getByRole('textbox', { name: 'Пропуск: впишите слово' }).fill('is');
  await hg.getByRole('textbox').blur();
  await hg.getByText('Сохранено').waitFor().catch(() => {});
  const ht = article(sp, 'Homework: choose the correct answer');
  const hq = (prompt) => ht.locator('li').filter({ hasText: prompt });
  await hq('Choose the correct question.').getByRole('radio', { name: 'Where are you from?' }).click();
  await hq('Which one is a country?').getByRole('radio', { name: 'Almaty' }).click(); // wrong on purpose
  await hq('— Are you from Almaty?').getByRole('radio', { name: 'am not' }).click();
  await settle(sp);
  assert(!(await ht.innerText()).includes('Неверно'), 'after-mode: no verdict before sending');
  await sp.screenshot({ path: `${OUT}/10-student-homework.png`, fullPage: true });
  await sp.getByRole('button', { name: 'Отправить на проверку' }).click();
  await sp.getByRole('dialog').getByRole('button', { name: 'Отправить на проверку' }).click();
  await sp.getByText('Работа отправлена и ожидает проверки учителем').waitFor();
  await sp.reload();
  await sp.getByRole('tab', { name: 'Домашняя работа' }).click();
  await sp.getByText('Работа отправлена и ожидает проверки учителем').waitFor();
  await sp.screenshot({ path: `${OUT}/11-student-homework-sent.png`, fullPage: true });

  await tp.reload();
  await tp.getByRole('tab', { name: 'Домашняя работа' }).click();
  await tp.getByText('Работа ожидает вашей проверки').waitFor();
  await tp.getByRole('button', { name: 'Завершить' }).click();
  await tp.getByRole('dialog').getByRole('button', { name: 'Завершить' }).click();
  await tp.getByText('Работа завершена').waitFor();
  await tp.screenshot({ path: `${OUT}/12-teacher-homework-done.png`, fullPage: true });
  await sp.getByText('Учитель отметил задание как выполненное').waitFor();
  ok('homework: the student fills the gap and the test and sends them; the teacher accepts the work');

  // ---- 7. The demo teacher (read only): the new course next to «English Starter A1» ----
  if (process.env.DEMO_TEACHER_PASSWORD) {
    const D = await login('teacher@ortalyq.local', process.env.DEMO_TEACHER_PASSWORD);
    await D.p.goto(`${WEB}/materials`);
    await D.p.getByText(COURSE).first().waitFor();
    await D.p.getByText('English Starter A1').first().waitFor();
    await settle(D.p);
    await D.p.screenshot({ path: `${OUT}/00-demo-teacher-materials.png` });
    ok('teacher@ortalyq.local sees the course next to «English Starter A1»');
  }

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
