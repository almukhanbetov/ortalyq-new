// Marathon after its end date: red banner, server-side lock for students, inactive status, progress kept.
// It CREATES data and MOVES the end date with SQL, so run it only against an isolated stack with its own DB
// (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e node scripts/marathon-finished-check.mjs
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e';
if (DB === 'ortalyq_new' || WEB.includes(':3448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/finished';
mkdirSync(OUT, { recursive: true });

const sql = (q) => execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'ortalyq_new', '-d', DB, '-Atc', q], { cwd: '..' }).toString().trim();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const RUN = Date.now().toString(36); // unique names, so the check can be repeated on the same DB

const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
async function login(email, pw) {
  const ctx = await b.newContext({ viewport: { width: 1862, height: 961 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(pw);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes$/);
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return { ctx, p };
}
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(500));
/** Calls the API from the page, with the page's session cookie (same as the app does). */
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

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');

  // ---- Setup through the API (the same calls the UI makes) ----
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const studentIds = mine.items.map((u) => u.id);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local')?.id;
  assert(S_ID, 'student@ortalyq.local is assigned to the teacher');
  assert(studentIds.length >= 1, 'teacher has assigned students');
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон: проверка завершения ${RUN}`, direction: 'languages', target_language: 'en' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс марафона ${RUN}` }, 201);
  const l1 = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title: 'Present Simple' }, 201);
  const l2 = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title: 'Past Simple' }, 201);
  const sec = await must(T.p, 'POST', `/lessons/${l1.id}/sections`, { title: 'Тест' }, 201);
  const ex = await must(T.p, 'POST', `/lessons/${l1.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Проверка',
    settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 1 },
    questions: [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] }] }, 201);
  ok('marathon, lessons and a test created');

  // Lessons and students through the UI.
  const url = `${WEB}/cabinet/school/marathons/marathon/${m.id}`;
  await T.p.goto(`${url}/lessons`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить урок' }).first().click();
  await T.p.getByRole('dialog', { name: 'Добавить урок' }).getByRole('button', { name: 'Выбрать из личных курсов' }).click();
  const picker = T.p.getByRole('dialog');
  await picker.getByRole('button', { name: new RegExp(`Курс марафона ${RUN}`) }).click();
  for (let i = 0; i < 2; i++) {
    await picker.getByRole('button', { name: 'Добавить в марафон' }).first().click();
    await T.p.waitForTimeout(600);
  }
  assert((await picker.getByText('Добавлен', { exact: true }).count()) === 2, 'both lessons marked as added');
  await picker.getByRole('button', { name: 'Назад к марафону' }).click();
  await settle(T.p);
  assert(await T.p.getByText('Кол-во уроков 2').isVisible(), 'lessons count 2');
  ok('lessons added from personal courses');

  await T.p.goto(`${url}/students`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить ученика' }).click();
  const add = T.p.getByRole('dialog', { name: 'Добавить учеников' });
  await add.waitFor();
  for (const box of await add.locator('label').all()) await box.click();
  await add.getByRole('button', { name: /^Добавить \(\d+\)$/ }).click();
  await settle(T.p);
  const rows = T.p.getByTestId('marathon-students').locator('li');
  assert((await rows.count()) === studentIds.length, 'all assigned students added');
  ok(`${studentIds.length} students added`);

  // End date through the settings (tomorrow, 17:00 in the marathon zone).
  await T.p.getByRole('button', { name: 'Настройки' }).click();
  const st = T.p.getByRole('dialog', { name: 'Настройки марафона' });
  await st.getByRole('tab', { name: 'Прохождение' }).click();
  await st.getByRole('switch', { name: 'Дата завершения' }).click();
  const tomorrow = new Date(Date.now() + 864e5).toISOString().slice(0, 10);
  await st.getByLabel('День').fill(tomorrow);
  await st.getByLabel(/^Время/).selectOption('17:00');
  assert(await st.getByText(/Доступ к урокам закроется ученикам/).isVisible(), 'end date hint');
  await T.p.screenshot({ path: `${OUT}/01-settings-end-date.png` });
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await settle(T.p);
  const ends = T.p.getByTestId('marathon-ends-banner');
  assert(/^Марафон завершится .+ в 17:00 \(UTC[+-]\d+\)\. Доступ к марафону будет закрыт\.$/.test(await ends.innerText()), `ends banner: ${await ends.innerText()}`);
  assert((await T.p.getByTestId('student-status').first().getAttribute('title')) === 'Активен', 'active before end');
  ok('blue banner before the end date; students active');

  // ---- Student before the end date: opens the lesson, answers, completes it ----
  await S.p.goto(url);
  await settle(S.p);
  assert(await S.p.getByTestId('marathon-ends-banner').isVisible(), 'student sees the ends banner');
  await S.p.getByRole('link', { name: 'Открыть урок' }).first().click();
  await S.p.waitForURL(/\/lesson\//);
  await settle(S.p);
  assert(await S.p.getByText('She ___ a teacher.').isVisible() || (await S.p.getByText('Проверка').count()) > 0, 'exercise visible');
  const view = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${ex.id}/attempt`, null, 200);
  const q = view.version.questions[0];
  await must(S.p, 'PUT', `/attempts/${view.attempt.id}/answers/${q.id}`, { option_ids: [q.options[0].id], check: true }, 200);
  await must(S.p, 'POST', `/attempts/${view.attempt.id}/finish`, null, 200);
  await S.p.reload();
  await settle(S.p);
  await S.p.getByRole('button', { name: 'Завершить урок' }).click();
  await S.p.getByText('Урок пройден').first().waitFor();
  await S.p.screenshot({ path: `${OUT}/02-student-lesson-before.png`, fullPage: true });
  ok('student passed lesson 1 before the end date');

  await T.p.reload();
  await settle(T.p);
  assert(await T.p.getByText('Пройдено 1 из 2').first().isVisible(), 'teacher sees progress 1 of 2');

  // "Прогресс ученика" before the end date: real data of the selected student, search over lessons.
  const row = T.p.getByTestId('marathon-students').locator('li').filter({ hasText: 'student@ortalyq.local' });
  await row.getByRole('button', { name: /Прогресс ученика/ }).click();
  await T.p.waitForURL(/\?student=/);
  const prog = T.p.getByRole('dialog', { name: 'Прогресс ученика' });
  await prog.getByTestId('student-progress').waitFor();
  assert(await prog.getByText('student@ortalyq.local').isVisible(), 'email in progress window');
  assert(await prog.getByText('Пройдено 1 из 2').isVisible(), 'passed 1 of 2 in window');
  assert(/^Ученик деактивируется .+ в 17:00 \(UTC[+-]\d+\)\. Причина: Дата завершения марафона\.$/.test(await prog.getByTestId('deactivation-banner').innerText()), 'deactivates banner');
  assert(await prog.locator('li').filter({ hasText: 'Present Simple' }).getByText('Пройден', { exact: true }).isVisible(), 'lesson 1 passed');
  assert(await prog.locator('li').filter({ hasText: 'Past Simple' }).getByText('Не пройден').isVisible(), 'lesson 2 not passed');
  await T.p.screenshot({ path: `${OUT}/03b-student-progress-before-end.png` });
  await prog.getByRole('searchbox').fill('Past');
  assert((await prog.locator('ol > li').count()) === 1, 'search filters lessons');
  await prog.getByRole('searchbox').fill('');
  await T.p.keyboard.press('Escape');
  await T.p.waitForURL((u) => !u.search.includes('student='));
  ok('progress window before the end: status, deactivation date, lessons, search');
  await T.p.screenshot({ path: `${OUT}/03-students-before-end.png` });

  // Row-level digest of everything a student produced: any insert, update or delete changes it.
  const counts = () => sql(`SELECT (SELECT count(*) FROM attempts WHERE classroom_id='${m.id}') || ',' ||
    (SELECT count(*) FROM attempt_answers a JOIN attempts t ON t.id=a.attempt_id WHERE t.classroom_id='${m.id}') || ',' ||
    (SELECT count(*) FROM marathon_lesson_progress WHERE classroom_id='${m.id}') || ',' ||
    (SELECT count(*) FROM marathon_enrollments WHERE classroom_id='${m.id}') || ',' ||
    (SELECT COALESCE(sum(points_earned),0) FROM attempts WHERE classroom_id='${m.id}') || ',' ||
    md5((SELECT COALESCE(string_agg(t::text, '|' ORDER BY t.id), '') FROM attempts t WHERE classroom_id='${m.id}') ||
        (SELECT COALESCE(string_agg(a::text, '|' ORDER BY a::text), '') FROM attempt_answers a JOIN attempts t ON t.id=a.attempt_id WHERE t.classroom_id='${m.id}') ||
        (SELECT COALESCE(string_agg(p::text, '|' ORDER BY p::text), '') FROM marathon_lesson_progress p WHERE classroom_id='${m.id}'))`);
  const before = counts();

  // ---- The end date arrives while the student has the lesson open (clock moved in the isolated DB) ----
  await S.p.goto(`${url}/lesson/${l2.id}`);
  await settle(S.p);
  sql(`UPDATE marathon_settings SET end_at = now() + interval '12 seconds' WHERE classroom_id = '${m.id}'`);
  await S.p.reload();
  await settle(S.p);
  assert(await S.p.getByRole('button', { name: 'Завершить урок' }).isVisible(), 'lesson 2 still open');
  await S.p.getByTestId('marathon-finished-banner').waitFor({ timeout: 30000 });
  await settle(S.p);
  assert(await S.p.getByText('Доступ к урокам закрыт').first().isVisible(), 'lesson closed without reload');
  assert((await S.p.getByRole('button', { name: 'Завершить урок' }).count()) === 0, 'no complete button after end');
  await S.p.screenshot({ path: `${OUT}/04-student-lesson-closed.png` });
  ok('end date reached: student page switches to the closed state by itself');

  // Direct API calls are rejected by the server.
  for (const [method, path, body] of [
    ['POST', `/classrooms/${m.id}/exercises/${ex.id}/attempt`, null],
    ['GET', `/classrooms/${m.id}/lessons/${l1.id}/exercises?scope=lesson`, null],
    ['PUT', `/attempts/${view.attempt.id}/answers/${q.id}`, { option_ids: [q.options[1].id] }],
    ['POST', `/attempts/${view.attempt.id}/finish`, null],
    ['POST', `/marathons/${m.id}/lessons/${l2.id}/complete`, null],
  ]) {
    const r = await call(S.p, method, path, body);
    assert(r.status === 403 && r.body?.error?.code === 'marathon_closed', `${method} ${path} → ${r.status} ${JSON.stringify(r.body)}`);
  }
  ok('server rejects start/answer/finish/complete with 403 marathon_closed');

  await S.p.goto(url);
  await settle(S.p);
  const fin = await S.p.getByTestId('marathon-finished-banner').innerText();
  assert(/^Марафон завершен .+ в \d\d:\d\d \(UTC[+-]\d+\)\. Доступ к марафону закрыт$/.test(fin), `finished banner: ${fin}`);
  assert(await S.p.getByText('Пройдено 1 из 2').isVisible(), 'student still sees own progress');
  assert((await S.p.getByRole('link', { name: 'Открыть урок' }).count()) === 0, 'lessons cannot be opened');
  await S.p.screenshot({ path: `${OUT}/05-student-marathon-finished.png` });
  ok('student marathon page: red banner, progress kept, lessons locked');

  // ---- Teacher-owner after the end ----
  await T.p.goto(`${url}/students`);
  await settle(T.p);
  assert(await T.p.getByTestId('marathon-finished-banner').isVisible(), 'teacher sees the red banner');
  const statuses = await T.p.getByTestId('student-status').evaluateAll((els) => els.map((e) => e.getAttribute('title')));
  assert(statuses.length === studentIds.length && statuses.every((s) => s === 'Неактивен'), `inactive statuses: ${statuses}`);
  assert(await T.p.getByText('Пройдено 1 из 2').first().isVisible(), 'progress kept for the teacher');
  assert(await T.p.getByText('Причина: Дата завершения марафона').first().isVisible(), 'inactive reason');
  await T.p.screenshot({ path: `${OUT}/06-marathon-finished-students.png` });
  await T.p.goto(`${url}/lessons`);
  await settle(T.p);
  assert(await T.p.getByText('Кол-во уроков 2').isVisible(), 'teacher still sees lessons');
  await T.p.screenshot({ path: `${OUT}/07-marathon-finished-lessons.png` });
  await T.p.getByRole('button', { name: 'Настройки' }).click();
  await T.p.getByRole('dialog', { name: 'Настройки марафона' }).getByRole('tab', { name: 'Прохождение' }).click();
  assert(await T.p.getByText('Эта дата уже прошла — марафон будет сразу завершён').isVisible(), 'settings show the past end date');
  await T.p.keyboard.press('Escape');
  ok('owner sees marathon, settings, participants and progress after the end');

  // "Прогресс ученика" after the end, then the read-only answers of lesson 1.
  await T.p.goto(`${url}/students?student=${S_ID}`);
  const prog2 = T.p.getByRole('dialog', { name: 'Прогресс ученика' });
  await prog2.getByTestId('student-progress').waitFor();
  assert(/^Ученик деактивирован .+\. Причина: Дата завершения марафона\.$/.test(await prog2.getByTestId('deactivation-banner').innerText()), 'deactivated banner');
  assert((await prog2.getByTestId('student-status').getAttribute('title')) === 'Неактивен', 'inactive in window');
  assert(await prog2.getByText('Пройдено 1 из 2').isVisible(), 'progress kept in window');
  await T.p.screenshot({ path: `${OUT}/09-student-progress-finished.png` });
  await prog2.locator('li').filter({ hasText: 'Present Simple' }).getByRole('link', { name: 'Открыть урок' }).click();
  await T.p.waitForURL(/\/lesson\/.+\?student=/);
  await T.p.getByTestId('lesson-review').waitFor();
  await settle(T.p);
  assert((await T.p.getByTestId('review-status').innerText()) === 'Пройден', 'review: lesson passed');
  assert((await T.p.getByTestId('review-points').innerText()) === 'Баллы: 1 из 1', 'review: points');
  assert(await T.p.getByText('She ___ a teacher.').isVisible(), 'review: saved question');
  assert((await T.p.getByRole('button', { name: 'Сбросить попытку' }).count()) === 0, 'review has no reset');
  assert((await T.p.getByRole('button', { name: /Начать|Завершить/ }).count()) === 0, 'review has no actions');
  await T.p.screenshot({ path: `${OUT}/10-student-answers-review.png`, fullPage: true });
  await T.p.getByRole('link', { name: /К прогрессу ученика/ }).click();
  await T.p.getByRole('dialog', { name: 'Прогресс ученика' }).waitFor();
  ok('owner reviews saved answers and points after the end; back returns to the progress window');

  // The student can neither use the owner endpoints nor reopen the lesson through the review address.
  for (const path of [`/marathons/${m.id}/students/${S_ID}/progress`, `/marathons/${m.id}/students/${S_ID}/lessons/${l1.id}`]) {
    const r = await call(S.p, 'GET', path);
    assert(r.status === 403, `student ${path} → ${r.status}`);
  }
  await S.p.goto(`${url}/lesson/${l1.id}?student=${S_ID}`);
  await S.p.getByText('Доступ к урокам закрыт').first().waitFor();
  ok('student stays locked out (403 on owner endpoints, closed lesson page)');

  assert(counts() === before, `data changed by finishing: ${before} → ${counts()}`);
  ok(`nothing deleted or reset (attempts, answers, progress, enrollments, points: ${before})`);

  // Localized banners.
  for (const [loc, re] of [['kk', /аяқталды\. Марафонға қолжетімділік жабық$/], ['en', /^The marathon ended on .+ at \d\d:\d\d \(UTC[+-]\d+\)\. Access to the marathon is closed$/]]) {
    await must(T.p, 'PATCH', '/auth/me', { locale: loc }, 200); // the app follows the profile language
    await T.p.evaluate((l) => localStorage.setItem('ortalyq_locale', l), loc);
    await T.p.goto(`${url}/students`);
    await settle(T.p);
    const text = await T.p.getByTestId('marathon-finished-banner').innerText();
    assert(re.test(text), `${loc} banner: ${text}`);
    await T.p.screenshot({ path: `${OUT}/08-finished-students-${loc}.png` });
  }
  await must(T.p, 'PATCH', '/auth/me', { locale: 'ru' }, 200);
  await T.p.evaluate(() => localStorage.setItem('ortalyq_locale', 'ru'));
  ok('kk and en banners');

  assert(!errors.length, `page errors: ${errors.join('; ')}`);
  console.log(`\nAll checks passed. Screenshots: ${OUT}`);
} finally {
  await b.close();
}
