// Marathon stage 7: statistics and rating in the browser. The teacher turns on «Статистика» and «Рейтинг учеников»
// in «Настройки → Дополнительно» (from the disabled «Статистика» tab), sees the table and downloads the CSV; the
// student sees the rating, «Результаты урока» with the statistics comment and «Успехи ученика в марафоне»; pages
// follow answers and settings through realtime; rights are checked by direct API calls.
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e_s7 node scripts/marathon-stats-check.mjs
// Screenshots go to a new folder per run (SHOTS_DIR), never over the screenshots of earlier stages.
import { chromium } from 'playwright-core';
import { mkdirSync, readFileSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e_s7';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/marathons/stage7-run-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });

const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const RUN = Date.now().toString(36);

const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
async function login(email, pw) {
  const ctx = await b.newContext({ viewport: { width: 1862, height: 961 }, locale: 'ru-RU', acceptDownloads: true });
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
const mark = (p) => p.evaluate(() => { window.__noReload = 1; });
const notReloaded = async (p, what) => assert(await p.evaluate(() => window.__noReload === 1), `${what}: the page was reloaded`);

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');
  const S2 = await login('student2@ortalyq.local', 'student-local-123');
  const T2 = await login('teacher2@ortalyq.local', 'teacher-local-123');
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local').id;
  const S2_ID = mine.items.find((u) => u.email === 'student2@ortalyq.local').id;
  const m = await must(T.p, 'POST', '/marathons', { name: `Марафон статистики ${RUN}`, direction: 'other' }, 201);
  const cats = await must(T.p, 'GET', '/score-categories', null, 200);
  const gram = (cats.items ?? cats).find((c) => c.name === 'Грамматика') ?? await must(T.p, 'POST', '/score-categories', { name: 'Грамматика' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс статистики ${RUN}` }, 201);
  const L = [];
  for (const [i, title] of ['Урок 1', 'Урок 2'].entries()) {
    const l = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title }, 201);
    const sec = await must(T.p, 'POST', `/lessons/${l.id}/sections`, { title: 'Section 1' }, 201);
    const questions = [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] }];
    if (i === 0) questions.push({ prompt: 'They ___ friends.', multiple: false, options: [{ label: 'are', is_correct: true }, { label: 'is', is_correct: false }] });
    const ex = await must(T.p, 'POST', `/lessons/${l.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
      settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 5, ...(i === 0 ? { category_id: gram.id } : {}) }, questions }, 201);
    await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: l.id }, 200);
    L.push({ id: l.id, ex: ex.id, title });
  }
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID, S2_ID] }, 200);
  const base = `${WEB}/cabinet/school/marathons/marathon/${m.id}`;
  const answer = async (p, ex, labels) => {
    const v = await must(p, 'POST', `/classrooms/${m.id}/exercises/${ex}/attempt`, null, 200);
    for (const [qi, label] of labels.entries()) {
      const q = v.version.questions[qi];
      await must(p, 'PUT', `/attempts/${v.attempt.id}/answers/${q.id}`, { option_ids: [q.options.find((o) => o.label === label).id], check: true }, 200);
    }
  };

  // ---- Statistics off: the tab explains it; direct API refuses ----
  await T.p.goto(`${base}/statistics`);
  await settle(T.p);
  await T.p.getByTestId('stats-disabled').waitFor();
  await T.p.screenshot({ path: `${OUT}/01-statistics-disabled.png` });
  await must(T.p, 'GET', `/marathons/${m.id}/stats`, null, 403, 'marathon_stats_disabled');
  await must(S.p, 'GET', `/marathons/${m.id}/rating`, null, 403, 'marathon_stats_disabled');
  ok('Вкладка «Статистика» при выключенной статистике: пояснение и «Перейти в настройки»; API — 403 marathon_stats_disabled');

  // ---- Settings → Дополнительно ----
  await T.p.getByRole('button', { name: 'Перейти в настройки' }).click();
  const st = T.p.getByRole('dialog', { name: 'Настройки марафона' });
  await st.getByTestId('stats-fields').waitFor();
  assert(await st.getByRole('switch', { name: 'Рейтинг учеников' }).isDisabled(), 'rating locked while statistics is off');
  await st.getByRole('switch', { name: 'Статистика' }).click();
  await st.getByRole('switch', { name: 'Рейтинг учеников' }).click();
  await st.getByLabel('Отображать').selectOption('3');
  await st.getByLabel('Место ученика').selectOption('show');
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/02-settings-statistics-rating.png` });
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await T.p.getByTestId('marathon-stats').waitFor();
  const cls = await must(T.p, 'GET', `/classrooms/${m.id}`, null, 200);
  assert(cls.marathon.stats_enabled && cls.marathon.rating_enabled && cls.marathon.rating_limit === 3 && cls.marathon.rating_show_place, `saved: ${JSON.stringify(cls.marathon)}`);
  ok('«Настройки → Дополнительно»: рейтинг заблокирован без статистики; включены статистика и рейтинг, «Первых 3», «Показывать»');

  // ---- Answers; the open statistics tab follows without reloading ----
  await mark(T.p);
  await answer(S.p, L[0].ex, ['is', 'is']); // 5 of 10
  await answer(S2.p, L[0].ex, ['is', 'are']); // 10 of 10
  await answer(S2.p, L[1].ex, ['is']); // 5 of 5
  await T.p.waitForFunction((sid) => document.querySelector(`[data-testid="stats-row"][data-student="${sid}"] [data-testid="stats-points"]`)?.textContent === '15 / 15', S2_ID, { timeout: 10000 });
  await notReloaded(T.p, 'statistics after answers');
  const rows = await T.p.getByTestId('stats-row').evaluateAll((trs) => trs.map((tr) => [tr.dataset.student, tr.dataset.place, tr.querySelector('[data-testid="stats-points"]').textContent]));
  assert(JSON.stringify(rows) === JSON.stringify([[S2_ID, '1', '15 / 15'], [S_ID, '2', '5 / 15']]), `rows: ${JSON.stringify(rows)}`);
  await settle(T.p);
  await T.p.screenshot({ path: `${OUT}/03-statistics-table.png` });
  ok('Таблица «Статистика»: места 1 и 2, баллы 15/15 и 5/15, колонки категорий; обновилась после ответов без перезагрузки');

  // ---- CSV export ----
  const [dl] = await Promise.all([T.p.waitForEvent('download'), T.p.getByRole('button', { name: 'Скачать данные' }).click()]);
  assert(dl.suggestedFilename().startsWith(`Марафон статистики ${RUN}, Статистика `) && dl.suggestedFilename().endsWith('.csv'), `file name: ${dl.suggestedFilename()}`);
  const csv = readFileSync(await dl.path(), 'utf8');
  const expected = '﻿Email ученика;Имя ученика;Завершено уроков;Заработано баллов;Грамматика;Универсальная;\r\n'
    + 'student2@ortalyq.local;Алия Ермекова;0;15,00;10,00;5,00;\r\n'
    + 'student@ortalyq.local;Данияр Нурланов;0;5,00;5,00;0,00;\r\n';
  assert(csv === expected, `csv: ${JSON.stringify(csv)}`);
  ok(`CSV «${dl.suggestedFilename()}»: BOM, «;», CRLF, десятичная запятая, колонка на категорию — совпадает с таблицей`);

  // ---- Student: rating on the marathon page, results page ----
  await S.p.goto(base);
  await settle(S.p);
  await S.p.getByTestId('rating').waitFor();
  const ratingRows = await S.p.getByTestId('rating').getByTestId('rating-row').evaluateAll((lis) => lis.map((li) => [li.dataset.place, li.dataset.me ?? '']));
  assert(JSON.stringify(ratingRows) === JSON.stringify([['1', ''], ['2', '1']]), `student rating: ${JSON.stringify(ratingRows)}`);
  assert(!(await S.p.getByTestId('rating').innerText()).includes('@'), 'rating shows no emails');
  await S.p.getByTestId('rating').scrollIntoViewIfNeeded();
  await S.p.screenshot({ path: `${OUT}/04-student-marathon-rating.png` });
  await S.p.getByTestId('show-results').click();
  await S.p.waitForURL(/\/results$/);
  await S.p.getByTestId('my-results').waitFor();
  assert((await S.p.getByTestId('my-lessons').innerText()).includes('0 / 2') && (await S.p.getByTestId('my-points').innerText()).includes('5 / 15'), 'my results tiles');
  await settle(S.p);
  await S.p.screenshot({ path: `${OUT}/05-student-results-page.png` });
  ok('Ученик: «Рейтинг учеников» (своя строка отмечена, без email), «Показать результаты» → «Успехи ученика в марафоне»: 0 / 2 уроков, 5 / 15 баллов');

  // ---- Lesson results with the statistics comment (this marathon only) ----
  await S.p.goto(`${base}/lesson/${L[0].id}`);
  await settle(S.p);
  await S.p.getByTestId('lesson-results').waitFor();
  assert((await S.p.getByTestId('lesson-results').getByTestId('points-total').innerText()) === '5 / 10', 'lesson points');
  await mark(S.p);
  await T.p.goto(`${base}/lessons`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Действия с уроком «Урок 1»' }).click();
  await T.p.getByRole('menuitem', { name: 'Результаты урока' }).click();
  const lr = T.p.getByRole('dialog', { name: 'Результаты урока' });
  await lr.getByTestId('lesson-students').waitFor();
  await lr.getByRole('button', { name: 'Добавить комментарий к статистике' }).click();
  await lr.getByLabel('Комментарий к статистике').fill('Повторите спряжение глагола to be.');
  await T.p.waitForTimeout(300);
  await T.p.screenshot({ path: `${OUT}/06-teacher-lesson-results-comment.png` });
  await lr.getByRole('button', { name: 'Сохранить' }).click();
  await lr.getByTestId('stats-comment').waitFor();
  await S.p.getByTestId('lesson-results').getByTestId('stats-comment').waitFor({ timeout: 10000 });
  await notReloaded(S.p, 'comment');
  await S.p.getByTestId('lesson-results').scrollIntoViewIfNeeded();
  await S.p.screenshot({ path: `${OUT}/07-student-lesson-results.png` });
  ok('«Результаты урока»: учитель добавил комментарий к статистике; ученик увидел его без перезагрузки вместе со своими баллами 5 / 10 и рейтингом урока');
  await T.p.keyboard.press('Escape');

  // ---- Rights through direct API calls ----
  await must(S.p, 'GET', `/marathons/${m.id}/stats`, null, 403);
  await must(S.p, 'PUT', `/marathons/${m.id}/stats-settings`, { stats_enabled: false }, 403);
  await must(S.p, 'PUT', `/marathons/${m.id}/lessons/${L[0].id}/stats-comment`, { body: 'x' }, 403);
  await must(T2.p, 'GET', `/marathons/${m.id}/stats`, null, 404);
  await must(T2.p, 'GET', `/marathons/${m.id}/rating`, null, 404);
  await must(T.p, 'GET', `/marathons/${m.id}/me/results`, null, 403);
  ok('Права: ученик — 403 на статистику, настройки и комментарий; чужой учитель — 404; учитель — 403 на «мои результаты»');

  // ---- Deactivation: out of the rating, still in the teacher's table ----
  await must(T.p, 'PUT', `/marathons/${m.id}/students/active`, { student_ids: [S2_ID], active: false }, 200);
  await S.p.goto(base);
  await settle(S.p);
  const after = await S.p.getByTestId('rating').getByTestId('rating-row').evaluateAll((lis) => lis.map((li) => [li.dataset.place, li.dataset.me ?? '']));
  assert(JSON.stringify(after) === JSON.stringify([['1', '1']]), `rating after deactivation: ${JSON.stringify(after)}`);
  await T.p.goto(`${base}/statistics`);
  await settle(T.p);
  assert(await T.p.locator(`[data-testid="stats-row"][data-student="${S2_ID}"]`).getAttribute('data-place') === '0', 'deactivated student not rated');
  await must(S2.p, 'GET', `/marathons/${m.id}/rating`, null, 403, 'marathon_deactivated');
  await must(T.p, 'PUT', `/marathons/${m.id}/students/active`, { student_ids: [S2_ID], active: true }, 200);
  ok('Деактивация: ученик выпал из рейтинга (в таблице учителя «Не в рейтинге»), сам получает 403; после активации вернулся');

  // ---- Statistics off: the student's blocks disappear without reloading ----
  await S.p.goto(`${base}/lesson/${L[0].id}`);
  await settle(S.p);
  await S.p.getByTestId('lesson-results').waitFor();
  await mark(S.p);
  await must(T.p, 'PUT', `/marathons/${m.id}/stats-settings`, { stats_enabled: false, rating_enabled: false, rating_limit: 3, rating_show_place: true }, 200);
  await S.p.getByTestId('lesson-results').waitFor({ state: 'detached', timeout: 10000 });
  await notReloaded(S.p, 'statistics off');
  await S.p.goto(`${base}/results`);
  await settle(S.p);
  await S.p.getByText('Баллы и рейтинг появятся, когда учитель включит статистику.').waitFor();
  await must(S.p, 'GET', `/marathons/${m.id}/lessons/${L[0].id}/results`, null, 403, 'marathon_stats_disabled');
  ok('Статистика выключена: результаты урока у ученика исчезли без перезагрузки; на странице результатов — пояснение; API — 403');

  // ---- Phone width ----
  await must(T.p, 'PUT', `/marathons/${m.id}/stats-settings`, { stats_enabled: true, rating_enabled: true, rating_limit: null, rating_show_place: false }, 200);
  for (const [who, url, shot] of [[T, `${base}/statistics`, '08-phone-statistics'], [S, `${base}/results`, '09-phone-student-results']]) {
    await who.p.setViewportSize({ width: 390, height: 844 });
    await who.p.goto(url);
    await settle(who.p);
    const sw = await who.p.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    assert(sw <= 0, `horizontal scroll on phone (${url}): ${sw}px`);
    await who.p.screenshot({ path: `${OUT}/${shot}.png` });
  }
  ok('Телефон 390 px: таблица прокручивается внутри своего блока, у страницы нет горизонтальной прокрутки');

  assert(errors.length === 0, `page errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
} finally {
  await b.close();
}
