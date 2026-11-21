// End-to-end UI scenario in two separate browser sessions (teacher + student).
// Requires running API + web and freshly seeded demo data (see README).
//   CHROME_PATH=/usr/bin/google-chrome node scripts/e2e.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3448';
const OUT = process.env.SHOTS_DIR || '../docs/screenshots';
const PW = { teacher: process.env.TEACHER_PASSWORD || 'teacher-local-123', student: process.env.STUDENT_PASSWORD || 'student-local-123' };
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: true });
const viewport = { width: 1440, height: 900 };
const tCtx = await browser.newContext({ viewport, locale: 'ru-RU', timezoneId: 'Asia/Almaty' });
const sCtx = await browser.newContext({ viewport, locale: 'ru-RU', timezoneId: 'Asia/Almaty' });
const T = await tCtx.newPage();
const S = await sCtx.newPage();
const errors = [];
for (const [name, p] of [['teacher', T], ['student', S]]) {
  p.on('pageerror', (e) => errors.push(`${name}: ${e.message}`));
  p.on('console', (m) => m.type() === 'error' && !/Failed to load resource/.test(m.text()) && errors.push(`${name} console: ${m.text()}`));
}
let step = 0;
const shot = async (p, name) => {
  await p.waitForTimeout(400);
  await p.screenshot({ path: `${OUT}/${String(++step).padStart(2, '0')}-${name}.png` });
};
const log = (s) => console.log(`✓ ${s}`);
const expectVisible = async (loc, what) => {
  await loc.first().waitFor({ state: 'visible', timeout: 15000 }).catch(() => { throw new Error(`not visible: ${what}`); });
};

async function login(p, email, pw) {
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(pw);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes/);
}

try {
  // ---- Login + classes list
  await T.goto(`${WEB}/login`);
  await shot(T, 'login');
  await login(T, 'teacher@ortalyq.local', PW.teacher);
  await expectVisible(T.getByText('Английский — Данияр'), 'class card');
  await shot(T, 'teacher-classes');
  log('teacher logged in, classes visible');

  // ---- Teacher creates materials: test in a section + homework test
  await T.goto(`${WEB}/materials`);
  await expectVisible(T.getByText('English Starter A1'), 'course');
  await shot(T, 'materials-courses');
  await T.getByText('English Starter A1').click();
  await T.getByText('Lesson 1. Hello!').click();
  await expectVisible(T.getByText('Warm-up'), 'section');
  await T.getByRole('button', { name: 'Добавить упражнение' }).first().click();
  await expectVisible(T.getByRole('dialog', { name: 'Добавить упражнение' }), 'catalog');
  await shot(T, 'exercise-catalog');
  await T.getByRole('button', { name: 'Тест', exact: true }).click();
  const ed = T.getByRole('dialog', { name: 'Новый тест' });
  await ed.getByLabel('Заголовок').fill('Проверка английского');
  // Validation: saving without a correct answer is refused on the client.
  await ed.getByLabel('Текст вопроса').fill('Choose the correct answer: She ___ a teacher.');
  await ed.getByLabel('Вариант ответа 1').fill('is');
  await ed.getByLabel('Вариант ответа 2').fill('are');
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await expectVisible(ed.getByText('Отметьте правильные варианты слева'), 'validation');
  await ed.getByRole('radio', { name: 'Отметить вариант 1 как правильный' }).click();
  await shot(T, 'editor-exercise');
  await ed.getByRole('tab', { name: 'Настройки' }).click();
  await ed.getByRole('switch', { name: 'Учитывать баллы' }).click();
  await shot(T, 'editor-settings');
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await expectVisible(T.getByText('Проверка английского'), 'exercise row');
  log('teacher created lesson test with scoring');

  await T.getByRole('tab', { name: 'Домашняя работа' }).click();
  await T.getByRole('button', { name: 'Добавить упражнение' }).click();
  await T.getByRole('button', { name: 'Тест', exact: true }).click();
  const ed2 = T.getByRole('dialog', { name: 'Новый тест' });
  await ed2.getByLabel('Заголовок').fill('Домашняя работа — to be');
  await ed2.getByLabel('Текст вопроса').fill('They ___ students.');
  await ed2.getByLabel('Вариант ответа 1').fill('are');
  await ed2.getByLabel('Вариант ответа 2').fill('is');
  await ed2.getByRole('radio', { name: 'Отметить вариант 1 как правильный' }).click();
  await ed2.getByRole('tab', { name: 'Настройки' }).click();
  await ed2.getByText('После завершения', { exact: true }).click();
  await ed2.getByRole('button', { name: 'Сохранить' }).click();
  await expectVisible(T.getByText('Домашняя работа — to be'), 'homework row');
  await shot(T, 'lesson-editor-homework');
  log('teacher created homework test');

  // Timed test (30 s) in the second section.
  await T.getByRole('tab', { name: 'Урок' }).click();
  await T.getByRole('button', { name: 'Добавить упражнение' }).nth(1).click();
  await T.getByRole('button', { name: 'Тест', exact: true }).click();
  const ed3 = T.getByRole('dialog', { name: 'Новый тест' });
  await ed3.getByLabel('Заголовок').fill('Таймер: to be');
  await ed3.getByLabel('Текст вопроса').fill('I ___ a student.');
  await ed3.getByLabel('Вариант ответа 1').fill('am');
  await ed3.getByLabel('Вариант ответа 2').fill('is');
  await ed3.getByRole('radio', { name: 'Отметить вариант 1 как правильный' }).click();
  await ed3.getByRole('button', { name: 'Добавить вопрос' }).click();
  await ed3.getByLabel('Текст вопроса').nth(1).fill('Pick all pronouns');
  await ed3.getByRole('checkbox', { name: 'Несколько правильных ответов' }).nth(1).check({ force: true }).catch(async () => {
    await ed3.getByText('Несколько правильных ответов').nth(1).click();
  });
  await ed3.getByLabel('Вариант ответа 1').nth(1).fill('he');
  await ed3.getByLabel('Вариант ответа 2').nth(1).fill('she');
  await ed3.getByRole('button', { name: 'Добавить ответ' }).nth(1).click();
  await ed3.getByLabel('Вариант ответа 3').fill('run');
  await ed3.getByRole('checkbox', { name: 'Отметить вариант 1 как правильный' }).click();
  await ed3.getByRole('checkbox', { name: 'Отметить вариант 2 как правильный' }).click();
  await ed3.getByRole('tab', { name: 'Настройки' }).click();
  await ed3.getByRole('switch', { name: 'Таймер' }).click();
  await ed3.getByLabel('Длительность, минут').fill('0.5');
  await ed3.getByRole('button', { name: 'Сохранить' }).click();
  await expectVisible(T.getByText('Таймер: to be'), 'timed row');
  log('teacher created timed test with a multiple-choice question');

  // ---- Teacher opens the lesson in the class via "Другой урок"
  await T.goto(`${WEB}/classes`);
  await T.getByRole('article').filter({ hasText: 'Английский — Данияр' }).getByRole('link', { name: 'Войти в класс' }).click();
  await expectVisible(T.getByText('Выберите урок, чтобы начать занятие'), 'empty classroom');
  await T.getByRole('button', { name: 'Выбрать урок' }).click();
  const picker = T.getByRole('dialog', { name: 'Другой урок' });
  await expectVisible(picker.getByText('Lesson 1. Hello!'), 'picker lessons');
  await shot(T, 'lesson-picker');
  await picker.getByRole('button', { name: 'Фильтр' }).click();
  await picker.getByLabel('Навыки').selectOption('Reading');
  await expectVisible(picker.getByText('Lesson 2. My family'), 'filtered');
  if (await picker.getByText('Lesson 1. Hello!').count()) throw new Error('filter did not exclude Lesson 1');
  await shot(T, 'lesson-picker-filter');
  await picker.getByRole('button', { name: 'Сбросить фильтр' }).click();
  await picker.locator('li').filter({ hasText: 'Lesson 1. Hello!' }).getByRole('button', { name: 'Открыть урок' }).click();
  await expectVisible(T.getByRole('heading', { name: 'Проверка английского' }), 'lesson exercise');
  await shot(T, 'teacher-classroom-lesson');
  const classUrl = T.url().split('?')[0];
  log('lesson opened in class');

  // ---- Student passes the test (instant check) and result persists after reload
  await login(S, 'student@ortalyq.local', PW.student);
  await shot(S, 'student-classes');
  await S.goto(classUrl);
  await expectVisible(S.getByRole('heading', { name: 'Проверка английского' }), 'student sees exercise');
  const html = await S.content();
  if (html.includes('is_correct')) throw new Error('answer key leaked to student page');
  await S.getByRole('radio', { name: 'is' }).click();
  await expectVisible(S.getByText('Верно'), 'correct mark');
  await S.getByRole('button', { name: 'Завершить' }).click();
  await S.getByRole('dialog').getByRole('button', { name: 'Завершить' }).click();
  await expectVisible(S.getByText('Затраченное время'), 'summary');
  await shot(S, 'student-test-result');
  await S.reload();
  await expectVisible(S.getByText('Затраченное время'), 'summary after reload');
  await expectVisible(S.getByText('1 / 1'), 'points 1/1 after reload');
  log('student passed test; result persisted after reload');

  // Timed test: start screen, countdown, reload does not reset, auto-finish.
  await S.getByRole('button', { name: 'Следующий раздел' }).click();
  await expectVisible(S.getByText('Тест с ограничением времени'), 'timer start screen');
  await shot(S, 'student-timer-start');
  await S.getByRole('button', { name: 'Начать тест' }).click();
  const timer = S.getByRole('timer');
  await expectVisible(timer, 'countdown');
  await S.getByRole('checkbox', { name: 'he', exact: true }).click();
  await S.getByRole('checkbox', { name: 'she', exact: true }).click();
  await S.getByRole('button', { name: 'Проверить' }).click();
  await expectVisible(S.getByText('Верно'), 'multi checked');
  const toSec = (x) => { const [m, sec] = x.trim().split(':').map(Number); return m * 60 + sec; };
  const before = toSec(await timer.innerText());
  await shot(S, 'student-timer-running');
  await S.waitForTimeout(3000);
  await S.reload();
  await expectVisible(timer, 'countdown after reload');
  const after = toSec(await timer.innerText());
  if (!(after <= before - 2)) throw new Error(`timer reset by reload: before=${before} after=${after}`);
  log(`timer survived reload (${before}s -> ${after}s)`);
  await S.getByText('Время истекло — тест завершён автоматически').waitFor({ timeout: 45000 });
  await shot(S, 'student-timer-expired');
  log('timed test finished automatically on expiry');
  await S.getByRole('button', { name: 'Предыдущий раздел' }).click();

  // Teacher sees student's answer read-only (no buttons on options)
  await T.reload();
  await expectVisible(T.getByText('Ответы ученика — только просмотр'), 'teacher read-only view');
  if (await T.getByRole('button', { name: 'is' }).count()) throw new Error('teacher can click student option');
  await shot(T, 'teacher-sees-student-answer');

  // ---- Homework: submit -> revision -> resubmit -> accept
  await S.getByRole('tab', { name: 'Домашняя работа' }).click();
  await expectVisible(S.getByText('Выполните задания и отправьте работу на проверку учителю'), 'blue notice');
  await S.getByRole('radio', { name: 'is' }).click(); // wrong on purpose
  await shot(S, 'student-homework-before-submit');
  await S.getByRole('button', { name: 'Отправить на проверку' }).click();
  await S.getByRole('dialog').getByRole('button', { name: 'Отправить на проверку' }).click();
  await expectVisible(S.getByText('Работа отправлена и ожидает проверки учителем'), 'yellow notice');
  await shot(S, 'student-homework-pending');
  log('student submitted homework');

  await T.getByRole('tab', { name: 'Домашняя работа' }).click();
  await expectVisible(T.getByText('Работа ожидает вашей проверки'), 'teacher pending');
  await shot(T, 'teacher-homework-pending');
  await T.getByRole('button', { name: 'Отправить на доработку' }).click();
  await T.getByRole('dialog').getByRole('button', { name: 'Отправить на доработку' }).click();
  await expectVisible(T.getByText('Работа на доработке у ученика'), 'teacher revision');
  log('teacher returned homework for revision');

  await expectVisible(S.getByText('Учитель вернул работу на доработку'), 'pink notice (realtime)');
  await shot(S, 'student-homework-revision');
  await S.getByRole('radio', { name: 'are' }).click();
  await S.waitForTimeout(600);
  await S.getByRole('button', { name: 'Отправить повторно' }).click();
  await S.getByRole('dialog').getByRole('button', { name: 'Отправить на проверку' }).click();
  await expectVisible(S.getByText('Работа отправлена и ожидает проверки учителем'), 'pending again');
  log('student resubmitted');

  await expectVisible(T.getByText('Работа ожидает вашей проверки'), 'teacher pending again');
  await expectVisible(T.getByText(/№2/), 'latest submission #2');
  await T.getByRole('button', { name: 'Завершить' }).click();
  await T.getByRole('dialog').getByRole('button', { name: 'Завершить' }).click();
  await expectVisible(T.getByText('Работа завершена'), 'teacher done');
  await shot(T, 'teacher-homework-done');
  await expectVisible(S.getByText('Учитель отметил задание как выполненное'), 'green notice');
  await shot(S, 'student-homework-done');
  log('teacher accepted homework; student sees green notice');

  // ---- Section completion + lesson completion with grade
  await T.getByRole('tab', { name: 'Урок' }).click();
  await T.getByRole('button', { name: 'Отметить раздел завершённым' }).first().click();
  await expectVisible(T.getByRole('button', { name: 'Снять отметку о завершении' }), 'section done');
  await T.getByRole('button', { name: 'Завершение урока' }).click();
  await expectVisible(T.getByText('Оцените урок'), 'finish page');
  await T.getByRole('radio', { name: 'Удивительно' }).click();
  await shot(T, 'teacher-finish-select');
  await T.getByRole('button', { name: 'Завершить урок' }).click();
  await expectVisible(T.getByText('Урок завершён'), 'lesson completed');
  await shot(T, 'teacher-finish-completed');
  await S.getByRole('tab', { name: 'Урок' }).click();
  await S.getByRole('button', { name: 'Завершение урока' }).click();
  await expectVisible(S.getByText('Урок завершён'), 'student sees completion');
  if (await S.getByRole('button', { name: 'Отменить завершение' }).count()) throw new Error('student can undo completion');
  await shot(S, 'student-finish');
  log('lesson completed with grade; student sees grade, cannot change it');

  // ---- Chat both ways, edit + delete sync
  await T.getByRole('button', { name: 'Чат', exact: true }).click();
  await S.getByRole('button', { name: 'Чат', exact: true }).click();
  const tIn = T.getByRole('textbox', { name: 'Написать сообщение…' });
  await tIn.fill('Hello! Great job today.');
  await tIn.press('Enter');
  await expectVisible(S.getByText('Hello! Great job today.'), 'student receives');
  const sIn = S.getByRole('textbox', { name: 'Написать сообщение…' });
  await sIn.fill('Thank you, teacher!');
  await sIn.press('Enter');
  await expectVisible(T.getByText('Thank you, teacher!'), 'teacher receives');
  log('chat delivered both ways');
  await T.getByRole('button', { name: 'Действия с сообщением' }).first().click();
  await T.getByRole('menuitem', { name: 'Редактировать' }).click();
  await tIn.fill('Hello! Great job today (edited).');
  await tIn.press('Enter');
  await expectVisible(S.getByText('Hello! Great job today (edited).'), 'edit synced');
  await shot(S, 'student-chat');
  await shot(T, 'teacher-chat');
  await T.getByRole('button', { name: 'Действия с сообщением' }).first().click();
  await T.getByRole('menuitem', { name: 'Удалить' }).click();
  await T.getByRole('dialog').getByRole('button', { name: 'Удалить' }).click();
  await S.getByText('Hello! Great job today (edited).').waitFor({ state: 'detached', timeout: 10000 });
  log('edit and delete synced to student');
  await T.getByRole('button', { name: 'Реакции' }).click();
  await expectVisible(T.getByText(/GIF недоступны|Powered by GIPHY/), 'gif panel');
  await shot(T, 'chat-reactions');
  await T.getByRole('dialog', { name: 'Реакции' }).getByRole('button', { name: 'Закрыть' }).click();

  // Reload keeps chat history without duplicates
  await S.reload();
  await S.getByRole('button', { name: 'Чат', exact: true }).click();
  await expectVisible(S.getByText('Thank you, teacher!'), 'history after reload');
  const dupes = await S.getByText('Thank you, teacher!').count();
  if (dupes !== 1) throw new Error(`duplicate messages after reload: ${dupes}`);
  log('history restored after reload, no duplicates');

  // Attachment upload through the paperclip; the other side can download it.
  const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64');
  await S.locator('input[type=file]').setInputFiles({ name: 'homework-photo.png', mimeType: 'image/png', buffer: png });
  const img = T.getByRole('img', { name: 'homework-photo.png' });
  await expectVisible(img, 'teacher sees uploaded image');
  const dl = await T.evaluate(async () => {
    const a = document.querySelector('a[aria-label^="Скачать"]');
    const r = await fetch(a.href, { credentials: 'include' });
    return { status: r.status, type: r.headers.get('content-type'), disp: r.headers.get('content-disposition') };
  });
  if (dl.status !== 200 || dl.type !== 'image/png' || !dl.disp.startsWith('attachment')) throw new Error(`download: ${JSON.stringify(dl)}`);
  await S.locator('input[type=file]').setInputFiles({ name: 'evil.html', mimeType: 'text/html', buffer: Buffer.from('<script>alert(1)</script>') });
  await expectVisible(S.getByText('Недопустимый тип файла'), 'rejected file type');
  log('attachment uploaded, downloadable by member; disallowed type rejected');

  // Reconnect: student goes offline, misses a message, comes back and resyncs without duplicates.
  await sCtx.setOffline(true);
  await expectVisible(S.getByText(/Переподключение/), 'reconnecting banner');
  await tIn.fill('Message sent while you were offline');
  await tIn.press('Enter');
  await expectVisible(T.getByText('Message sent while you were offline'), 'teacher sent');
  await sCtx.setOffline(false);
  await expectVisible(S.getByText('Message sent while you were offline'), 'student resynced after reconnect');
  await S.getByText(/Переподключение/).waitFor({ state: 'detached', timeout: 15000 });
  if ((await S.getByText('Message sent while you were offline').count()) !== 1) throw new Error('duplicate after reconnect');
  log('websocket reconnect: missed message restored, no duplicates');

  // ---- Mobile adaptation (no Edvibe reference)
  const m = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, storageState: await sCtx.storageState() });
  const M = await m.newPage();
  await M.goto(`${WEB}/classes`);
  await expectVisible(M.getByText('Английский — Данияр'), 'mobile classes');
  await M.screenshot({ path: `${OUT}/mobile-classes.png` });
  await M.goto(classUrl);
  await expectVisible(M.getByRole('heading', { name: 'Проверка английского' }), 'mobile lesson');
  await M.screenshot({ path: `${OUT}/mobile-lesson.png` });
  await M.getByRole('tab', { name: 'Домашняя работа' }).click();
  await M.waitForTimeout(500);
  await M.screenshot({ path: `${OUT}/mobile-homework.png` });
  log('mobile screenshots captured');

  if (errors.length) {
    console.log('Browser errors:\n' + errors.join('\n'));
    process.exitCode = 1;
  } else console.log('\nE2E PASSED');
} catch (e) {
  console.error('E2E FAILED:', e.message);
  await T.screenshot({ path: `${OUT}/fail-teacher.png` }).catch(() => {});
  await S.screenshot({ path: `${OUT}/fail-student.png` }).catch(() => {});
  if (errors.length) console.log(errors.join('\n'));
  process.exitCode = 1;
} finally {
  await browser.close();
}
