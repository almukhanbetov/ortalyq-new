// Marathon stage 6: «Проверка учителем» in the browser. The teacher switches the check on in the settings
// ("Открытие уроков" appears in «По мере прохождения»); the student sends a lesson for review, answers lock;
// the teacher returns it for revision, accepts it with a rating and a comment, cancels the completion and
// completes it again. Both pages follow every decision through realtime, without reloading; the next lesson
// opens after the acceptance and closes again after the cancellation.
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e node scripts/marathon-review-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/review';
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
// A page that must update by realtime keeps this marker: a navigation or reload would drop it.
const mark = (p) => p.evaluate(() => { window.__noReload = 1; });
const notReloaded = async (p, what) => assert(await p.evaluate(() => window.__noReload === 1), `${what}: the page was reloaded`);
const finishState = (p) => p.getByTestId('finish-state').getAttribute('data-state');
const waitState = async (p, testid, state) => p.waitForFunction(({ testid, state }) =>
  document.querySelector(`[data-testid="${testid}"]`)?.getAttribute(testid === 'finish-state' ? 'data-state' : 'data-status') === state, { testid, state }, { timeout: 10000 });

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const student = mine.items.find((u) => u.email === 'student@ortalyq.local');
  const S_ID = student.id;
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон с проверкой ${RUN}`, direction: 'other' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс проверки ${RUN}` }, 201);
  const L = [];
  for (const [i, title] of ['Урок 1', 'Урок 2'].entries()) {
    const l = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title }, 201);
    const sec = await must(T.p, 'POST', `/lessons/${l.id}/sections`, { title: 'Section 1' }, 201);
    const questions = [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] }];
    if (i === 0) questions.push({ prompt: 'They ___ friends.', multiple: false, options: [{ label: 'are', is_correct: true }, { label: 'is', is_correct: false }] });
    const ex = await must(T.p, 'POST', `/lessons/${l.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
      settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 5 }, questions }, 201);
    await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: l.id }, 200);
    L.push({ id: l.id, ex: ex.id, title });
  }
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID] }, 200);
  await must(T.p, 'POST', `/marathons/${m.id}/mode`, { mode: 'progressive', expected_epoch: 1, confirm_reset: true }, 200);
  const base = `${WEB}/cabinet/school/marathons/marathon/${m.id}`;
  const lessonUrl = (i) => `${base}/lesson/${L[i].id}`;
  const reviewUrl = (i) => `${base}/lesson/${L[i].id}?student=${S_ID}`;

  // ---- Settings: «Проверка учителем» and «Открытие уроков» ----
  await T.p.goto(`${base}/students`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Настройки' }).click();
  const st = T.p.getByRole('dialog', { name: 'Настройки марафона' });
  await st.getByRole('tab', { name: 'Прохождение' }).click();
  await st.getByRole('switch', { name: 'Проверка учителем' }).click();
  const unlock = st.getByLabel('Открытие уроков');
  await unlock.waitFor();
  assert(await unlock.inputValue() === 'teacher_check', 'default unlock_after');
  await st.getByTestId('review-fields').scrollIntoViewIfNeeded();
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/01-settings-teacher-check.png` });
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Сохранено' }).first().waitFor().catch(() => {});
  await settle(T.p);
  const cls = await must(T.p, 'GET', `/classrooms/${m.id}`, null, 200);
  assert(cls.marathon.teacher_check === true && cls.marathon.unlock_after === 'teacher_check' && cls.marathon.progress_epoch === 2, `saved: ${JSON.stringify(cls.marathon)}`);
  ok('Настройки → «Прохождение»: «Проверка учителем» включена, «Открытие уроков: После проверки учителем», статусы не сброшены');

  // ---- Student: answers (one wrong), «Отправить на проверку» with confirmation ----
  const v = await must(S.p, 'POST', `/classrooms/${m.id}/exercises/${L[0].ex}/attempt`, null, 200);
  const [q1, q2] = v.version.questions;
  const opt = (q, label) => q.options.find((o) => o.label === label).id;
  await must(S.p, 'PUT', `/attempts/${v.attempt.id}/answers/${q1.id}`, { option_ids: [opt(q1, 'is')], check: true }, 200);
  await must(S.p, 'PUT', `/attempts/${v.attempt.id}/answers/${q2.id}`, { option_ids: [opt(q2, 'is')], check: true }, 200);
  await S.p.goto(base);
  await settle(S.p);
  // Wait for the rendered list instead of reading the DOM right after networkidle: on a slow CPU the page hydrates and
  // asks for /me only after the network has been idle, and a one-off read then saw an empty list (flaky failure).
  await S.p.getByTestId('student-lessons').locator('li').nth(1).getByTestId('lesson-lock').waitFor();
  const locks = await S.p.getByTestId('student-lessons').locator('li').evaluateAll((lis) => lis.map((li) => li.querySelector('[data-testid="lesson-lock"]')?.textContent ?? ''));
  assert(locks[1] === 'Откроется после проверки предыдущего урока учителем', `lock text: ${locks}`);
  await S.p.goto(lessonUrl(0));
  await settle(S.p);
  assert(await finishState(S.p) === 'none', 'not completed state');
  await S.p.getByTestId('lesson-finish').scrollIntoViewIfNeeded();
  await S.p.screenshot({ path: `${OUT}/02-student-not-completed.png` });
  await S.p.getByRole('button', { name: 'Отправить на проверку' }).click();
  const sd = S.p.getByRole('dialog', { name: 'Отправить урок на проверку' });
  await sd.waitFor();
  await S.p.waitForTimeout(300);
  await S.p.screenshot({ path: `${OUT}/03-student-submit-confirm.png` });
  await sd.getByRole('button', { name: 'Отправить' }).click();
  await waitState(S.p, 'finish-state', 'submitted');
  await settle(S.p);
  await S.p.getByTestId('lesson-finish').scrollIntoViewIfNeeded();
  await S.p.screenshot({ path: `${OUT}/04-student-awaiting-review.png` });
  await must(S.p, 'PUT', `/attempts/${v.attempt.id}/answers/${q2.id}`, { option_ids: [opt(q2, 'are')], check: true }, 409, 'lesson_review_locked');
  await must(S.p, 'POST', `/marathons/${m.id}/lessons/${L[1].id}/submit`, null, 403, 'marathon_lesson_locked');
  ok('Ученик: «Урок не завершён» → подтверждение → «Урок ожидает проверку»; ответ по API — 409 lesson_review_locked; урок 2 закрыт');

  // ---- Teacher: counters, review page ----
  await T.p.goto(`${WEB}/cabinet/school/classes?type=2`);
  await settle(T.p);
  const pending = T.p.getByRole('img', { name: 'Ожидают проверки: 1' });
  await pending.first().waitFor();
  await T.p.goto(`${base}/lessons`);
  await settle(T.p);
  await T.p.getByTestId('lesson-pending').first().waitFor(); // same reason: wait for the list, then read it
  assert((await T.p.getByTestId('lesson-pending').allInnerTexts()).join('|') === 'На проверке: 1', 'lesson pending chip');
  await T.p.goto(`${base}/students?student=${S_ID}`);
  await settle(T.p);
  assert(await T.p.getByTestId('student-progress').getByTestId('lesson-status').first().getAttribute('data-status') === 'submitted', 'progress status');
  await T.p.screenshot({ path: `${OUT}/05-teacher-progress-awaiting.png` });
  await T.p.goto(reviewUrl(0));
  await settle(T.p);
  await waitState(T.p, 'review-panel', 'submitted');
  await T.p.getByTestId('review-panel').scrollIntoViewIfNeeded();
  await T.p.screenshot({ path: `${OUT}/06-teacher-awaiting-review.png` });
  ok('Учитель: значок «Ожидают проверки: 1» на карточке, «На проверке: 1» у урока, песочные часы в «Прогресс ученика», экран «Урок ожидает проверку»');

  // ---- Revision: the student's open page follows without reloading ----
  await mark(S.p);
  await T.p.getByRole('button', { name: 'Отправить на доработку' }).click();
  const rd = T.p.getByRole('dialog', { name: 'Отправить на доработку' });
  await rd.waitFor();
  await rd.getByRole('button', { name: 'Отправить на доработку' }).click();
  await waitState(T.p, 'review-panel', 'revision');
  await waitState(S.p, 'finish-state', 'revision');
  await notReloaded(S.p, 'revision');
  await settle(S.p);
  await S.p.getByTestId('lesson-finish').scrollIntoViewIfNeeded();
  await S.p.screenshot({ path: `${OUT}/07-student-revision.png` });
  await must(S.p, 'PUT', `/attempts/${v.attempt.id}/answers/${q1.id}`, { option_ids: [opt(q1, 'are')], check: true }, 409, 'answer_locked');
  await must(S.p, 'PUT', `/attempts/${v.attempt.id}/answers/${q2.id}`, { option_ids: [opt(q2, 'are')], check: true }, 200);
  ok('«Отправить на доработку»: у ученика без перезагрузки «Урок отправлен на доработку»; верный ответ зафиксирован, неверный исправлен');

  // ---- Second submission; the teacher's open page follows ----
  await mark(T.p);
  await S.p.reload();
  await settle(S.p);
  await S.p.getByRole('button', { name: 'Отправить на проверку' }).click();
  await S.p.getByRole('dialog', { name: 'Отправить урок на проверку' }).getByRole('button', { name: 'Отправить' }).click();
  await waitState(S.p, 'finish-state', 'submitted');
  await waitState(T.p, 'review-panel', 'submitted');
  await notReloaded(T.p, 'second submission');
  ok('Повторная отправка: страница учителя без перезагрузки показывает «Урок ожидает проверку»');

  // A second, soon stale tab of the teacher: its rating window is opened before the lesson is accepted in the first tab.
  const T2 = await T.ctx.newPage();
  T2.on('pageerror', (e) => errors.push(e.message));
  await T2.goto(reviewUrl(0));
  await waitState(T2, 'review-panel', 'submitted');
  await T2.getByTestId('review-panel').getByRole('button', { name: 'Завершить урок' }).click();
  await T2.getByRole('dialog', { name: 'Поставьте оценку' }).getByRole('radio', { name: /Неплохо/ }).click();

  // ---- «Завершить урок» with a rating and a comment ----
  await mark(S.p);
  await T.p.bringToFront();
  await T.p.getByTestId('review-panel').getByRole('button', { name: 'Завершить урок' }).click();
  const rate = T.p.getByRole('dialog', { name: 'Поставьте оценку' });
  await rate.waitFor();
  assert(await rate.getByRole('button', { name: 'Оценить' }).isDisabled(), 'rating must be chosen explicitly');
  await rate.getByRole('radio', { name: /Отлично/ }).click();
  await rate.getByLabel('Комментарий ученику').fill('Отличная работа, все ошибки исправлены.');
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/08-teacher-rating-modal.png` });
  await rate.getByRole('button', { name: 'Оценить' }).click();
  await waitState(T.p, 'review-panel', 'completed');
  await settle(T.p);
  await T.p.getByTestId('review-panel').scrollIntoViewIfNeeded();
  await T.p.screenshot({ path: `${OUT}/09-teacher-completed.png` });
  await waitState(S.p, 'finish-state', 'completed');
  await notReloaded(S.p, 'acceptance');
  await S.p.getByTestId('lesson-rating').waitFor();
  assert(await S.p.getByTestId('lesson-rating').getAttribute('data-rating') === 'amazing', 'student sees the rating');
  assert((await S.p.getByTestId('lesson-comment').innerText()) === 'Отличная работа, все ошибки исправлены.', 'student sees the comment');
  await settle(S.p);
  await S.p.getByTestId('lesson-finish').scrollIntoViewIfNeeded();
  await S.p.screenshot({ path: `${OUT}/10-student-completed.png` });
  const reviewed = await must(T.p, 'GET', `/marathons/${m.id}/students/${S_ID}/lessons/${L[0].id}`, null, 200);
  assert(reviewed.points.earned === 10 && reviewed.points.max === 10, `points: ${JSON.stringify(reviewed.points)}`);
  ok('«Завершить урок» → «Поставьте оценку» (выбор обязателен) → «Отлично» с комментарием; ученик видит оценку и комментарий без перезагрузки; 10/10 баллов');

  // ---- The stale tab accepts again with another rating: idempotent, and the interface says nothing changed ----
  await T2.bringToFront();
  await T2.getByRole('dialog', { name: 'Поставьте оценку' }).getByRole('button', { name: 'Оценить' }).click();
  const unchangedNote = T2.getByTestId('review-unchanged');
  await unchangedNote.waitFor();
  const note = await unchangedNote.innerText();
  assert(note.includes('«Отлично»') && note.includes('«Неплохо»') && note.includes('Отменить завершение'), `unchanged notice: ${note}`);
  await T2.getByRole('status').filter({ hasText: 'ничего не изменилось' }).first().waitFor();
  const kept = await must(T.p, 'GET', `/marathons/${m.id}/students/${S_ID}/lessons/${L[0].id}`, null, 200);
  assert(kept.lesson.rating === 'amazing' && kept.lesson.teacher_comment === 'Отличная работа, все ошибки исправлены.', `rating kept: ${JSON.stringify(kept.lesson)}`);
  await T2.getByTestId('review-panel').scrollIntoViewIfNeeded();
  await T2.screenshot({ path: `${OUT}/09b-teacher-repeat-acceptance-unchanged.png` });
  await T2.close();
  await T.p.bringToFront();
  ok('Повторное принятие из устаревшей вкладки с оценкой «Неплохо»: сервер оставил «Отлично» и комментарий, интерфейс объясняет, что оценка не изменилась и как её поменять');

  // ---- «Следующий урок» opens lesson 2 ----
  await S.p.getByTestId('next-lesson').click();
  await S.p.waitForURL(new RegExp(`/lesson/${L[1].id}$`));
  await settle(S.p);
  assert(await finishState(S.p) === 'none', 'lesson 2 is open');
  ok('«Следующий урок» открывает урок 2');

  // ---- «Отменить завершение»: lesson 2 closes again on the open student page ----
  await mark(S.p);
  await T.p.getByRole('button', { name: 'Отменить завершение' }).click();
  const cd = T.p.getByRole('dialog', { name: 'Отменить завершение' });
  await cd.waitFor();
  await cd.getByText('снова закроется').waitFor();
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/11-teacher-cancel-confirm.png` });
  await cd.getByRole('button', { name: 'Продолжить' }).click();
  await waitState(T.p, 'review-panel', 'submitted');
  await S.p.getByText('Урок пока закрыт').waitFor({ timeout: 10000 });
  await notReloaded(S.p, 'cancel');
  await S.p.screenshot({ path: `${OUT}/12-student-next-closed-after-cancel.png` });
  const afterCancel = await must(T.p, 'GET', `/marathons/${m.id}/students/${S_ID}/lessons/${L[0].id}`, null, 200);
  assert(afterCancel.points.earned === 10 && afterCancel.lesson.rating === null && afterCancel.lesson.previous_rating === 'amazing', `after cancel: ${JSON.stringify(afterCancel.lesson)}`);
  ok('«Отменить завершение» с предупреждением: урок 1 снова «Ожидает проверку», урок 2 у ученика закрылся без перезагрузки; баллы 10/10 сохранены');

  // ---- Second completion: the previous rating is preselected, lesson 2 opens again ----
  await T.p.getByTestId('review-panel').getByRole('button', { name: 'Завершить урок' }).click();
  const rate2 = T.p.getByRole('dialog', { name: 'Поставьте оценку' });
  await rate2.waitFor();
  assert(await rate2.getByRole('radio', { name: /Отлично/ }).getAttribute('aria-checked') === 'true', 'previous rating preselected');
  await rate2.getByRole('button', { name: 'Оценить' }).click();
  await waitState(T.p, 'review-panel', 'completed');
  await waitState(S.p, 'finish-state', 'none');
  await notReloaded(S.p, 'second completion');
  ok('Повторное «Завершить урок»: «Отлично» уже выбрано; урок 2 у ученика снова открылся без перезагрузки');

  // ---- Phone width ----
  for (const [who, url] of [[T, reviewUrl(0)], [S, lessonUrl(0)]]) {
    await who.p.setViewportSize({ width: 390, height: 844 });
    await who.p.goto(url);
    await settle(who.p);
    const sw = await who.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    assert(sw <= 0, `horizontal scroll on phone: ${sw}px`);
  }
  await T.p.getByTestId('review-panel').scrollIntoViewIfNeeded();
  await T.p.screenshot({ path: `${OUT}/13-phone-teacher-review.png` });
  await S.p.getByTestId('lesson-finish').scrollIntoViewIfNeeded();
  await S.p.screenshot({ path: `${OUT}/14-phone-student-completed.png` });
  ok('Телефон 390 px: страница проверки и итог урока без горизонтальной прокрутки');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
