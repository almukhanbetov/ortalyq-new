// Plan, section 6.3: the whole marathon chain in the browser, teacher and student in two contexts (ru-RU, 1440×900
// and 390×844). Create → settings right after creation (progressive mode, teacher check, statistics and rating) →
// lessons (from personal courses and a new one) → tests made in the lesson editor → student → timed test (a reload
// keeps the countdown) → «Отправить на проверку» → the teacher accepts (the student's page updates without a reload)
// → the next lesson opens → results, rating, statistics and CSV agree → cancel and complete again → the preview
// leaves student data alone. Plus: no requests to external domains, no console errors, no sideways scroll on a phone,
// ru/kk/en have the same keys; screenshots at 1862×961.
// It prepares its own users (a new teacher and student through the administrator) and creates data, so it runs only
// against an isolated stack:
//   WEB_URL=http://localhost:3452 API_URL=http://localhost:8452 SHOTS_DIR=… node scripts/marathon-e2e.mjs
import { chromium } from 'playwright-core';
import { mkdirSync, readFileSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB, API } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/marathons/e2e-run-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });

async function login(email, password, viewport = { width: 1440, height: 900 }) {
  const ctx = await b.newContext({ viewport, locale: 'ru-RU', acceptDownloads: true });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 4(01|03|04|09|22)/.test(m.text())) errors.push(m.text()); });
  p.on('request', (r) => { const u = new URL(r.url()); if (/^https?:$/.test(u.protocol) && !['localhost', '127.0.0.1'].includes(u.hostname)) external.add(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes$/);
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return { ctx, p };
}
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(400));
const waitState = (p, testid, state) => p.waitForFunction(({ testid, state }) =>
  document.querySelector(`[data-testid="${testid}"]`)?.getAttribute(testid === 'finish-state' ? 'data-state' : 'data-status') === state, { testid, state }, { timeout: 15000 });
const noSideScroll = (p) => p.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
const shot = async (p, name) => {
  const v = p.viewportSize();
  await p.setViewportSize({ width: 1862, height: 961 });
  await p.waitForTimeout(300);
  await p.screenshot({ path: `${OUT}/${name}.png` });
  await p.setViewportSize(v);
};

// Makes a test in the lesson editor: title, questions (first option right), scoring on, optional timer in minutes.
async function editorTest(p, section, title, questions, timerMinutes) {
  await p.locator('section').filter({ hasText: section }).getByRole('button', { name: 'Добавить упражнение' }).first().click();
  await p.getByRole('dialog', { name: 'Добавить упражнение' }).getByRole('button', { name: 'Тест', exact: true }).click();
  const ed = p.getByRole('dialog', { name: 'Новый тест' });
  await ed.getByLabel('Заголовок').fill(title);
  for (const [i, [prompt, right, wrong]] of questions.entries()) {
    if (i > 0) await ed.getByRole('button', { name: 'Добавить вопрос' }).click();
    await ed.getByLabel('Текст вопроса').nth(i).fill(prompt);
    await ed.getByLabel('Вариант ответа 1').nth(i).fill(right);
    await ed.getByLabel('Вариант ответа 2').nth(i).fill(wrong);
    await ed.getByRole('radio', { name: 'Отметить вариант 1 как правильный' }).nth(i).click();
  }
  await ed.getByRole('tab', { name: 'Настройки' }).click();
  await ed.getByRole('switch', { name: 'Учитывать баллы' }).click();
  if (timerMinutes) {
    await ed.getByRole('switch', { name: 'Таймер' }).click();
    await ed.getByLabel('Длительность, минут').fill(String(timerMinutes));
  }
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.waitFor({ state: 'hidden' });
  await p.getByText(title).first().waitFor();
}

try {
  // ---- Own users: a new teacher with one assigned student ----
  const users = await makeUsers(stack, RUN, { students: ['Данияр'] });
  const [SU] = users.students;
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  // A personal course with a lesson and a section (the teacher's materials before the marathon exists).
  const course = await api.must('POST', '/courses', { title: `Личный курс ${RUN}` }, 201);
  const personal = await api.must('POST', `/courses/${course.id}/lessons`, { title: 'Урок 1. To be' }, 201);
  await api.must('POST', `/lessons/${personal.id}/sections`, { title: 'Разминка' }, 201);

  const T = await login(users.teacher.email, users.teacher.password);
  const S = await login(SU.email, SU.password);
  const main = T.p.locator('main');

  // ---- 1. «Создать класс» → «Марафоны» → form → settings right after creation ----
  await T.p.goto(`${WEB}/cabinet/school/classes?type=2`);
  await settle(T.p);
  await main.getByRole('button', { name: 'Создать класс' }).first().click();
  await T.p.getByRole('dialog', { name: 'Создать класс' }).getByRole('button', { name: /Марафоны/ }).click();
  const form = T.p.getByRole('dialog', { name: /Марафон/ });
  await form.getByLabel('Название марафона').fill(`Марафон ${RUN}`);
  await form.getByLabel('Направление').selectOption('other');
  await form.getByRole('button', { name: 'Создать', exact: true }).click();
  const setup = T.p.getByRole('dialog', { name: 'Настройки марафона' });
  await setup.waitFor();
  await T.p.waitForURL(/\/marathon\/[0-9a-f-]{36}\/students$/);
  const id = T.p.url().match(/marathon\/([0-9a-f-]{36})/)[1];
  const base = `${WEB}/cabinet/school/marathons/marathon/${id}`;
  assert(await setup.getByRole('button', { name: 'Настроить позже' }).isVisible(), '«Настроить позже»');
  // Progressive mode (with the reset confirmation), teacher check, statistics and rating — all in this dialog.
  await setup.getByRole('radio', { name: /^По мере прохождения/ }).click();
  await T.p.getByRole('dialog', { name: 'Смена способа прохождения' }).getByRole('button', { name: 'Продолжить' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Способ прохождения изменён' }).waitFor();
  await setup.getByRole('switch', { name: 'Проверка учителем' }).click();
  await setup.getByRole('tab', { name: 'Дополнительно' }).click();
  await setup.getByRole('switch', { name: 'Статистика' }).click();
  await setup.getByRole('switch', { name: 'Рейтинг учеников' }).click();
  await shot(T.p, '01-settings-after-create');
  await setup.getByRole('button', { name: 'Сохранить' }).click();
  await setup.waitFor({ state: 'hidden' });
  const cls = await api.must('GET', `/classrooms/${id}`, null, 200);
  assert(cls.marathon.mode === 'progressive' && cls.marathon.teacher_check && cls.marathon.unlock_after === 'teacher_check'
    && cls.marathon.stats_enabled && cls.marathon.rating_enabled, `settings: ${JSON.stringify(cls.marathon)}`);
  await T.p.goto(`${WEB}/cabinet/school/classes?type=2`);
  await settle(T.p);
  await main.locator('article').filter({ hasText: `Марафон ${RUN}` }).waitFor();
  ok('1. «Создать класс» → «Марафоны» → форма → «Настройки марафона» сразу после создания («По мере прохождения», проверка учителем, статистика и рейтинг) → марафон на ?type=2');

  // ---- 2. Lessons: from personal courses and a new one; tests made in the editor ----
  await T.p.goto(`${base}/lessons`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить урок' }).first().click();
  await T.p.getByRole('dialog', { name: 'Добавить урок' }).getByRole('button', { name: 'Выбрать из личных курсов' }).click();
  const picker = T.p.getByRole('dialog');
  await picker.getByRole('button', { name: new RegExp(`Личный курс ${RUN}`) }).click();
  await picker.getByRole('button', { name: 'Добавить в марафон' }).first().click();
  await picker.getByText('Добавлен', { exact: true }).waitFor();
  await picker.getByRole('button', { name: 'Назад к марафону' }).click();
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить урок' }).first().click();
  await T.p.getByRole('dialog', { name: 'Добавить урок' }).getByRole('button', { name: 'Создать новый урок' }).click();
  const cr = T.p.getByRole('dialog', { name: 'Создать новый урок' });
  await cr.getByLabel('Название').fill('Урок 2. Present Simple');
  await cr.getByRole('button', { name: 'Создать' }).click();
  await T.p.getByRole('status').filter({ hasText: 'Урок создан' }).waitFor();
  const L = (await api.must('GET', `/marathons/${id}/lessons`, null, 200)).items.map((x) => x.lesson_id);
  assert(L.length === 2, 'two lessons');
  // Lesson 1 («Редактировать» → editor → timed test → «К марафону»).
  const editRow = (title) => T.p.getByRole('listitem').filter({ hasText: title }).getByRole('link', { name: 'Редактировать' });
  await editRow('Урок 1. To be').click();
  await T.p.waitForURL(/\/materials\/lessons\//);
  await settle(T.p);
  await editorTest(T.p, 'Разминка', 'Тест на время', [['She ___ a teacher.', 'is', 'are'], ['They ___ friends.', 'are', 'were']], 2);
  await shot(T.p, '02-editor-timed-test');
  await T.p.getByRole('link', { name: 'К марафону' }).click();
  await T.p.waitForURL(new RegExp(`/marathon/${id}/lessons`));
  await settle(T.p);
  // Lesson 2: a section and a test, also in the editor.
  await editRow('Урок 2. Present Simple').click();
  await T.p.waitForURL(/\/materials\/lessons\//);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить раздел' }).click();
  const sd = T.p.getByRole('dialog', { name: 'Добавить раздел' });
  await sd.getByLabel('Название раздела').fill('Практика');
  await sd.getByRole('button', { name: 'Сохранить' }).click();
  await sd.waitFor({ state: 'hidden' });
  await editorTest(T.p, 'Практика', 'Тест 2', [['I ___ here.', 'am', 'is']]);
  await T.p.getByRole('link', { name: 'К марафону' }).click();
  await T.p.waitForURL(new RegExp(`/marathon/${id}/lessons`));
  await settle(T.p);
  await shot(T.p, '03-lessons');
  ok('2. «Выбрать из личных курсов» и «Создать новый урок» → «Редактировать» → тест в редакторе (таймер, баллы) → «К марафону»');

  // ---- 3. «Добавить ученика» → «Существующего» ----
  await T.p.goto(`${base}/students`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Добавить ученика' }).first().click();
  const add = T.p.getByRole('dialog', { name: 'Добавить учеников' });
  await add.locator('label').filter({ hasText: SU.name }).click();
  await add.getByRole('button', { name: 'Добавить (1)' }).click();
  await add.waitFor({ state: 'hidden' });
  await main.getByText('Пройдено 0 из 2').waitFor();
  await shot(T.p, '04-students');
  ok('3. «Добавить ученика» → «Существующего» → ученик в списке, «Пройдено 0 из 2»');

  // ---- 4. Student: the marathon → lesson → timed test (reload keeps the countdown) → finish → send ----
  await S.p.goto(`${WEB}/cabinet/school/classes?type=2`);
  await settle(S.p);
  await S.p.locator('main article').filter({ hasText: `Марафон ${RUN}` }).getByRole('link').first().click();
  await S.p.waitForURL(new RegExp(`/marathon/${id}`));
  await S.p.getByTestId('student-lessons').locator('li').nth(1).getByTestId('lesson-lock').waitFor();
  await S.p.getByTestId('student-lessons').locator('li').first().getByRole('link', { name: 'Открыть урок' }).click();
  await S.p.waitForURL(new RegExp(`/lesson/${L[0]}$`));
  const card = () => S.p.getByRole('article', { name: 'Тест на время' });
  await card().getByRole('button', { name: 'Начать' }).click();
  await card().getByRole('timer').waitFor();
  await card().getByRole('radio', { name: 'is' }).first().click();
  await card().getByText('Верно').first().waitFor();
  await S.p.waitForTimeout(2500);
  const secs = async () => { const [mm, ss] = (await card().getByRole('timer').innerText()).trim().split(':').map(Number); return mm * 60 + ss; };
  const before = await secs();
  await S.p.reload();
  await card().getByRole('timer').waitFor();
  const after = await secs();
  assert(after <= before && after < 120 && after > 90, `countdown after reload: ${before} → ${after}`);
  await card().getByRole('radio', { name: 'were' }).click();
  await card().getByText('Неверно').waitFor();
  await card().getByRole('button', { name: 'Завершить' }).click();
  await S.p.getByRole('dialog').getByRole('button', { name: 'Завершить' }).click();
  await card().getByText('Затраченное время').waitFor();
  await S.p.getByRole('button', { name: 'Отправить на проверку' }).click();
  await S.p.getByRole('dialog', { name: 'Отправить урок на проверку' }).getByRole('button', { name: 'Отправить' }).click();
  await waitState(S.p, 'finish-state', 'submitted');
  await shot(S.p, '05-student-submitted');
  ok(`4. Ученик: марафон → урок → тест на время (после перезагрузки отсчёт продолжился: ${before} → ${after} с) → «Завершить» → «Отправить на проверку»`);

  // ---- 5. Teacher: «Прогресс ученика» → answers → «Завершить урок»; the student's page updates by itself ----
  await S.p.evaluate(() => { window.__noReload = 1; });
  await T.p.goto(`${base}/students`);
  await settle(T.p);
  await T.p.getByRole('button', { name: `Прогресс ученика: ${SU.name}` }).click();
  const progress = T.p.getByTestId('student-progress');
  await progress.getByTestId('lesson-status').first().waitFor();
  assert(await progress.getByTestId('lesson-status').first().getAttribute('data-status') === 'submitted', 'progress shows the sent lesson');
  await progress.getByRole('link', { name: 'Открыть урок' }).first().click();
  await T.p.waitForURL(new RegExp(`/lesson/${L[0]}\\?student=`));
  await waitState(T.p, 'review-panel', 'submitted');
  await T.p.getByRole('article', { name: 'Тест на время' }).getByText('Неверно').first().waitFor();
  await T.p.getByTestId('review-panel').getByRole('button', { name: 'Завершить урок' }).click();
  const rate = T.p.getByRole('dialog', { name: 'Поставьте оценку' });
  await rate.getByRole('radio', { name: /Хорошо/ }).click();
  await rate.getByRole('button', { name: 'Оценить' }).click();
  await waitState(T.p, 'review-panel', 'completed');
  await waitState(S.p, 'finish-state', 'completed');
  assert(await S.p.evaluate(() => window.__noReload === 1), 'the student page was reloaded');
  await shot(S.p, '06-student-accepted-realtime');
  ok('5. Учитель: «Прогресс ученика» → ответы → «Завершить урок» с оценкой; страница ученика обновилась без перезагрузки');

  // ---- 6. Next lesson open; results, rating, statistics and CSV show the same points ----
  await S.p.getByTestId('next-lesson').click();
  await S.p.waitForURL(new RegExp(`/lesson/${L[1]}$`));
  await waitState(S.p, 'finish-state', 'none');
  const stu = await apiAs(stack, SU.email, SU.password);
  const my = await stu.must('GET', `/marathons/${id}/me/results`, null, 200);
  const lr = await stu.must('GET', `/marathons/${id}/lessons/${L[0]}/results`, null, 200);
  const rt = await stu.must('GET', `/marathons/${id}/rating`, null, 200);
  assert(my.points.earned === 1 && lr.points.earned === 1 && lr.points.max === 2 && rt.items[0].points === 1 && rt.items[0].me, `points: ${my.points.earned} ${lr.points?.earned} ${rt.items[0]?.points}`);
  await S.p.goto(`${base}/results`);
  await S.p.getByText('Заработано баллов').first().waitFor();
  assert((await S.p.locator('main').innerText()).includes('1 / 3'), 'results page shows 1 / 3');
  await shot(S.p, '07-student-results-rating');
  await T.p.goto(`${base}/statistics`);
  await T.p.getByRole('button', { name: 'Скачать данные' }).waitFor();
  const [dl] = await Promise.all([T.p.waitForEvent('download'), T.p.getByRole('button', { name: 'Скачать данные' }).click()]);
  const csv = readFileSync(await dl.path(), 'utf8');
  assert(csv.includes(`${SU.email};${SU.name};1;1,00;`), `csv: ${JSON.stringify(csv)}`);
  await shot(T.p, '08-statistics');
  ok('6. Следующий урок открыт; «Результаты», «Результаты урока», рейтинг, статистика и CSV показывают те же баллы (1)');

  // ---- Cancel the completion and complete again; the student's page follows ----
  await S.p.goto(`${base}/lesson/${L[1]}`);
  await waitState(S.p, 'finish-state', 'none');
  await T.p.goto(`${base}/lesson/${L[0]}?student=${SU.id}`);
  await waitState(T.p, 'review-panel', 'completed');
  await T.p.getByRole('button', { name: 'Отменить завершение' }).click();
  await T.p.getByRole('dialog', { name: 'Отменить завершение' }).getByRole('button', { name: 'Продолжить' }).click();
  await waitState(T.p, 'review-panel', 'submitted');
  await S.p.getByText('Урок пока закрыт').waitFor({ timeout: 15000 });
  let st = (await api.must('GET', `/marathons/${id}/stats`, null, 200)).items[0];
  assert(st.completed_lessons === 0 && st.points.earned === 1, `after cancel: ${JSON.stringify(st)}`);
  await T.p.getByTestId('review-panel').getByRole('button', { name: 'Завершить урок' }).click();
  await T.p.getByRole('dialog', { name: 'Поставьте оценку' }).getByRole('button', { name: 'Оценить' }).click();
  await waitState(T.p, 'review-panel', 'completed');
  await waitState(S.p, 'finish-state', 'none');
  st = (await api.must('GET', `/marathons/${id}/stats`, null, 200)).items[0];
  assert(st.completed_lessons === 1 && st.points.earned === 1, `after completing again: ${JSON.stringify(st)}`);
  ok('«Отменить завершение» → урок 2 у ученика закрылся; повторное «Завершить урок» → открылся; баллы не изменились');

  // ---- The preview keeps student data apart ----
  const snap = async () => JSON.stringify(await Promise.all([api.call('GET', `/marathons/${id}/stats`), stu.call('GET', `/marathons/${id}/rating`),
    stu.call('GET', `/marathons/${id}/me`)]), (k, v) => (k === 'server_now' ? undefined : v));
  const before2 = await snap();
  await T.p.goto(`${base}/preview/${L[1]}`);
  await T.p.getByTestId('preview-banner').waitFor();
  await T.p.getByRole('article', { name: 'Тест 2' }).getByRole('radio', { name: 'is' }).click();
  await T.p.getByRole('article', { name: 'Тест 2' }).getByText('Неверно').waitFor();
  assert(await snap() === before2, 'the preview changed student data');
  ok('Предпросмотр учителя не изменил статистику, рейтинг и уроки ученика');

  // ---- Phone ----
  const SP = await login(SU.email, SU.password, { width: 390, height: 844 });
  for (const u of [base, `${base}/lesson/${L[0]}`, `${base}/results`]) {
    await SP.p.goto(u);
    await settle(SP.p);
    assert(await noSideScroll(SP.p), `sideways scroll: ${u}`);
  }
  await SP.p.screenshot({ path: `${OUT}/09-phone-student.png` });
  const TP = await login(users.teacher.email, users.teacher.password, { width: 390, height: 844 });
  for (const tab of ['students', 'lessons', 'statistics']) {
    await TP.p.goto(`${base}/${tab}`);
    await settle(TP.p);
    assert(await noSideScroll(TP.p), `sideways scroll: ${tab}`);
  }
  await TP.p.screenshot({ path: `${OUT}/10-phone-teacher.png` });
  ok('Телефон 390×844: страницы ученика и учителя без горизонтальной прокрутки');

  // ---- Locales ----
  const keys = (o, p = '') => Object.entries(o).flatMap(([k, v]) => (v && typeof v === 'object' ? keys(v, `${p}${k}.`) : [`${p}${k}`])).sort().join('\n');
  const [ru, kk, en] = ['ru', 'kk', 'en'].map((l) => keys(JSON.parse(readFileSync(new URL(`../src/locales/${l}.json`, import.meta.url), 'utf8'))));
  assert(ru === kk && ru === en, 'ru/kk/en keys differ');
  ok(`Ключи ru/kk/en совпадают (${ru.split('\n').length})`);

  assert(external.size === 0, `requests to external domains: ${[...external].slice(0, 5).join(' ')}`);
  ok('Нет запросов к внешним доменам');
  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  ok('Ошибок страницы и консоли нет');
  await api.dispose();
  await stu.dispose();
  console.log(`\nMARATHON E2E PASSED. Скриншоты: ${OUT}`);
} catch (e) {
  console.error('FAILED:', e.message);
  process.exitCode = 1;
} finally {
  await b.close();
}
