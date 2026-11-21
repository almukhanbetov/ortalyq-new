// Marathon stage 10: «Оформление» in the browser. The owner uploads a marathon cover and a lesson cover (with wrong
// files refused by the client and by the server), sets reactions and a ready background in «Оформление», gives a
// lesson its own appearance with a custom background, switches «Оформление урока» on. The student sees the
// background, gets an icon and a sound only for answers the server has checked (not for an unchecked test), can
// switch the sound off, and gets nothing where reactions are off. Also: owner preview, realtime update, rights,
// phone width, ru/kk/en, cover removal, students' data untouched, no console errors.
// It CREATES data, so run it only against an isolated stack with its own DB (never the demo DB on :3448/:8448):
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 E2E_DB=ortalyq_new_e2e_s10 node scripts/marathon-appearance-check.mjs
// Screenshots go to a new folder per run (SHOTS_DIR), never over the screenshots of earlier stages.
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { deflateSync } from 'node:zlib';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const API = process.env.API_URL || 'http://localhost:8449';
const DB = process.env.E2E_DB || 'ortalyq_new_e2e_s10';
if (DB === 'ortalyq_new' || WEB.includes(':3448') || API.includes(':8448')) throw new Error('refusing to run against the demo stack');
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/marathons/stage10-run-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });

const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const RUN = Date.now().toString(36);

// ---- A tiny PNG encoder (gradient pictures for covers and backgrounds) ----
const crcTable = Array.from({ length: 256 }, (_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
const crc32 = (buf) => { let c = 0xffffffff; for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
const chunk = (type, data) => {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
  const td = Buffer.concat([Buffer.from(type), data]);
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(td));
  return Buffer.concat([len, td, crc]);
};
function png(w, h, [r1, g1, b1], [r2, g2, b2]) {
  const raw = Buffer.alloc((w * 3 + 1) * h);
  for (let y = 0; y < h; y++) {
    raw[y * (w * 3 + 1)] = 0;
    for (let x = 0; x < w; x++) {
      const k = (x + y) / (w + h); const o = y * (w * 3 + 1) + 1 + x * 3;
      raw[o] = r1 + (r2 - r1) * k; raw[o + 1] = g1 + (g2 - g1) * k; raw[o + 2] = b1 + (b2 - b1) * k;
    }
  }
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = 2;
  return Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]);
}
const COVER = { name: 'cover.png', mimeType: 'image/png', buffer: png(240, 240, [26, 198, 255], [146, 84, 222]) };
const LESSON_COVER = { name: 'lesson.png', mimeType: 'image/png', buffer: png(160, 160, [255, 176, 32], [255, 92, 147]) };
const BG = { name: 'bg.png', mimeType: 'image/png', buffer: png(640, 400, [231, 249, 239], [232, 248, 255]) };
const FAKE = { name: 'fake.png', mimeType: 'image/png', buffer: Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>') };
const HUGE = { name: 'huge.png', mimeType: 'image/png', buffer: Buffer.alloc(6 * 1024 * 1024, 1) };
const TEXT = { name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('hello') };

const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
// Counts started oscillators: the sounds are synthesised, so a started oscillator is a played reaction sound.
const countSounds = () => {
  window.__sounds = 0;
  const orig = AudioContext.prototype.createOscillator;
  AudioContext.prototype.createOscillator = function (...a) { window.__sounds++; return orig.apply(this, a); };
};
async function login(email, pw, viewport = { width: 1862, height: 961 }) {
  const ctx = await b.newContext({ viewport, locale: 'ru-RU' });
  await ctx.addInitScript(countSounds);
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 4(01|03|04|09|13|22)/.test(m.text())) errors.push(m.text()); });
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
const sounds = (p) => p.evaluate(() => window.__sounds);
// Student data without the clock and the pictures (a cover is appearance, not the student's data).
const noNow = (v) => JSON.stringify(v, (k, x) => (k === 'server_now' || k === 'cover_image_id' ? undefined : x));
const imgLoaded = (loc) => loc.evaluate((el) => { const i = el.querySelector('img'); return !!i && i.complete && i.naturalWidth > 0; });

try {
  const T = await login('teacher@ortalyq.local', 'teacher-local-123');
  const S = await login('student@ortalyq.local', 'student-local-123');
  const mine = await must(T.p, 'GET', '/teacher/students', null, 200);
  const S_ID = mine.items.find((u) => u.email === 'student@ortalyq.local').id;
  const m = await must(T.p, 'POST', '/marathons', { name: `Оформление ${RUN}`, direction: 'other' }, 201);
  const course = await must(T.p, 'POST', '/courses', { title: `Курс оформления ${RUN}` }, 201);
  const L = [];
  for (const [i, title] of ['Урок 1', 'Урок 2'].entries()) {
    const l = await must(T.p, 'POST', `/courses/${course.id}/lessons`, { title }, 201);
    const sec = await must(T.p, 'POST', `/lessons/${l.id}/sections`, { title: 'Раздел' }, 201);
    await must(T.p, 'POST', `/lessons/${l.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Тест',
      settings: { check_mode: 'instant', scoring_enabled: true, points_per_correct: 1 },
      questions: [{ prompt: 'She ___ a teacher.', multiple: false, options: [{ label: 'is', is_correct: true }, { label: 'are', is_correct: false }] },
        { prompt: 'They ___ friends.', multiple: false, options: [{ label: 'are', is_correct: true }, { label: 'is', is_correct: false }] },
        { prompt: 'I ___ here.', multiple: false, options: [{ label: 'am', is_correct: true }, { label: 'is', is_correct: false }] }] }, 201);
    if (i === 0) {
      await must(T.p, 'POST', `/lessons/${l.id}/exercises`, { scope: 'lesson', section_id: sec.id, kind: 'test', title: 'Проверка в конце',
        settings: { check_mode: 'after', scoring_enabled: true, points_per_correct: 1 },
        questions: [{ prompt: 'We ___ ready.', multiple: false, options: [{ label: 'are', is_correct: true }, { label: 'is', is_correct: false }] }] }, 201);
    }
    await must(T.p, 'POST', `/marathons/${m.id}/lessons`, { lesson_id: l.id }, 200);
    L.push({ id: l.id, title });
  }
  await must(T.p, 'POST', `/marathons/${m.id}/students`, { student_ids: [S_ID] }, 200);
  const base = `${WEB}/cabinet/school/marathons/marathon/${m.id}`;
  const studentData = async () => noNow(await Promise.all([
    call(S.p, 'GET', `/marathons/${m.id}/me`), call(T.p, 'GET', `/marathons/${m.id}/students`),
    call(S.p, 'GET', `/classrooms/${m.id}/lessons/${L[0].id}/exercises?scope=lesson`),
  ]));
  const before = await studentData();

  // ---- Owner: marathon cover in «Основные» ----
  await T.p.goto(`${base}/students`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Настройки' }).click();
  const dlg = T.p.getByRole('dialog');
  const coverField = dlg.getByTestId('marathon-cover-image');
  const coverInput = coverField.locator('input[type=file]');
  await coverInput.setInputFiles(TEXT);
  await coverField.getByRole('alert').filter({ hasText: 'PNG, JPEG или WebP' }).waitFor();
  await coverInput.setInputFiles(HUGE);
  await coverField.getByRole('alert').filter({ hasText: 'больше 5 МБ' }).waitFor();
  await coverInput.setInputFiles(FAKE); // passes the browser check, refused by the server by its content
  await coverField.getByRole('alert').filter({ hasText: 'PNG, JPEG или WebP' }).waitFor();
  assert((await must(T.p, 'GET', `/classrooms/${m.id}`, null, 200)).cover_image_id == null, 'refused file became a cover');
  ok('Обложка: текстовый файл и файл больше 5 МБ отклонены в браузере, SVG под видом PNG — сервером по содержимому');
  await coverInput.setInputFiles(COVER);
  await coverField.getByRole('button', { name: 'Удалить картинку' }).waitFor();
  await settle(T.p);
  await T.p.screenshot({ path: `${OUT}/01-settings-cover-uploaded.png` });
  const cls = await must(T.p, 'GET', `/classrooms/${m.id}`, null, 200);
  assert(cls.cover_image_id, 'cover not stored');
  const header = T.p.locator(`header [data-cover-image="${cls.cover_image_id}"]`);
  await header.waitFor();
  assert(await imgLoaded(header), 'header cover not loaded');
  ok('Обложка загружена и сразу видна в окне и в шапке марафона');

  // ---- «Оформление»: reactions and a ready background ----
  await dlg.getByRole('tab', { name: 'Оформление' }).click();
  await dlg.getByTestId('marathon-look').waitFor();
  await dlg.getByRole('switch', { name: 'Реакции на ответы' }).click();
  await dlg.getByTestId('marathon-correct-icons').locator('[data-icon=fire]').click();
  await dlg.getByTestId('marathon-correct-sound').getByRole('combobox').selectOption('3');
  const before3 = await sounds(T.p);
  await dlg.getByRole('button', { name: 'Прослушать звук 3' }).click();
  await T.p.waitForTimeout(300);
  assert(await sounds(T.p) > before3, '«Прослушать» did not play');
  await dlg.getByRole('switch', { name: 'Фон в классе' }).click();
  await dlg.getByTestId('marathon-bg-options').getByTitle('Волны').click();
  await dlg.getByText('Свои иконки и звуки').scrollIntoViewIfNeeded();
  await T.p.screenshot({ path: `${OUT}/02-settings-appearance.png` });
  // An icon kind without icons cannot be saved.
  const inc = dlg.getByTestId('marathon-incorrect-icons');
  await inc.locator('[data-icon=sad]').click();
  await inc.locator('[data-icon=grimace]').click();
  await inc.getByRole('alert').filter({ hasText: 'хотя бы одну иконку' }).waitFor();
  assert(await dlg.getByRole('button', { name: 'Сохранить' }).isDisabled(), 'save must wait for an icon');
  await inc.locator('[data-icon=thinking]').click();
  await dlg.getByRole('button', { name: 'Сохранить' }).click();
  await dlg.waitFor({ state: 'hidden' });
  const ap = await must(T.p, 'GET', `/marathons/${m.id}/appearance`, null, 200);
  assert(ap.appearance.reactions_enabled && ap.appearance.background === 'waves' && ap.appearance.reactions.correct_sound.sound === 3
    && ap.appearance.reactions.correct_icon.icons.join() === 'party_face,party_popper,fire' && ap.appearance.reactions.incorrect_icon.icons.join() === 'thinking',
  `saved appearance: ${JSON.stringify(ap)}`);
  ok('«Оформление»: реакции (иконки, «Звук 3» с прослушиванием), фон «Волны» сохранены; без иконок «Сохранить» недоступна');
  // Re-opening shows the saved values.
  await T.p.getByRole('button', { name: 'Настройки' }).click();
  await dlg.getByRole('tab', { name: 'Оформление' }).click();
  assert(await dlg.getByTestId('marathon-correct-icons').locator('[data-icon=fire]').getAttribute('aria-pressed') === 'true', 'icons not restored');
  assert(await dlg.getByTestId('marathon-bg-options').getByRole('radio', { name: 'Волны' }).isChecked(), 'background not restored');
  await dlg.getByRole('button', { name: 'Отмена' }).click();
  ok('Повторное открытие показывает сохранённые настройки');

  // ---- Lesson 2: cover and its own appearance with a custom background ----
  await T.p.goto(`${base}/lessons`);
  await settle(T.p);
  const row2 = T.p.getByRole('listitem').filter({ hasText: 'Урок 2' });
  await T.p.getByRole('button', { name: 'Действия с уроком «Урок 2»' }).click();
  await T.p.getByRole('menuitem', { name: 'Настройки урока' }).click();
  const ldlg = T.p.getByRole('dialog');
  await ldlg.getByTestId('lesson-cover-image').locator('input[type=file]').setInputFiles(LESSON_COVER);
  await ldlg.getByTestId('lesson-cover-image').getByRole('button', { name: 'Удалить картинку' }).waitFor();
  await ldlg.getByRole('tab', { name: 'Оформление' }).click();
  await ldlg.getByText('выключено «Оформление урока»').waitFor();
  await ldlg.getByRole('switch', { name: 'Своё оформление урока' }).click();
  await ldlg.getByRole('switch', { name: 'Фон в классе' }).click();
  await ldlg.getByTestId('lesson-bg-image').locator('input[type=file]').setInputFiles(BG);
  await ldlg.getByTestId('lesson-bg-options').getByRole('radio', { name: 'Свой фон' }).waitFor();
  await T.p.waitForFunction(() => document.querySelector('[data-testid=lesson-bg-options] input[value=custom]')?.checked);
  await settle(T.p);
  await T.p.screenshot({ path: `${OUT}/03-lesson-own-appearance.png` });
  await ldlg.getByRole('button', { name: 'Сохранить' }).click();
  await ldlg.waitFor({ state: 'hidden' });
  const la = await must(T.p, 'GET', `/marathons/${m.id}/lessons/${L[1].id}/appearance`, null, 200);
  assert(la.enabled && !la.reactions_enabled && la.background === 'custom' && la.background_image_id, `lesson appearance: ${JSON.stringify(la)}`);
  const lessons = await must(T.p, 'GET', `/marathons/${m.id}/lessons`, null, 200);
  assert(lessons.items[1].cover_image_id && !lessons.items[0].cover_image_id, 'lesson cover');
  assert(await imgLoaded(row2.locator(`[data-cover-image="${lessons.items[1].cover_image_id}"]`)), 'lesson cover not shown in the list');
  await T.p.screenshot({ path: `${OUT}/04-lessons-with-cover.png` });
  ok('Урок 2: картинка обложки в списке, своё оформление (реакции выключены, свой фон загружен и выбран)');
  assert(await studentData() === before, 'teacher appearance changes touched student data');
  ok('Данные ученика (уроки, статусы, ответы) после настройки оформления не изменились');

  // ---- Student: marathon card and lesson 1 with the marathon appearance ----
  await S.p.goto(`${WEB}/cabinet/school/classes?type=2`);
  await settle(S.p);
  assert(await imgLoaded(S.p.locator(`[data-cover-image="${cls.cover_image_id}"]`).first()), 'student card cover');
  ok('Ученик видит обложку марафона в «Мои классы»');
  await S.p.goto(`${base}/lesson/${L[0].id}`);
  await settle(S.p);
  assert(await S.p.getByTestId('lesson-background').getAttribute('data-preset') === 'waves', 'student background');
  await S.p.getByTestId('reaction-sound-toggle').waitFor();
  const test = S.p.getByRole('article', { name: 'Тест' });
  // An answer of the end-checked test is not checked by the server: no reaction.
  const s0 = await sounds(S.p);
  await S.p.getByRole('article', { name: 'Проверка в конце' }).getByRole('radio', { name: 'are' }).click();
  await settle(S.p);
  assert(await S.p.getByTestId('answer-reaction').count() === 0 && await sounds(S.p) === s0, 'reaction for an unchecked answer');
  ok('Ответ в тесте с проверкой в конце (сервер не проверил) — без реакции');
  await test.getByRole('radio', { name: 'is' }).first().click();
  const burst = S.p.getByTestId('answer-reaction');
  await burst.waitFor();
  assert(await burst.getAttribute('data-kind') === 'correct', 'correct reaction');
  const icon = await burst.innerText();
  assert(['🥳', '🎉', '🔥'].includes(icon.trim()), `correct icon ${icon}`);
  await S.p.waitForTimeout(250);
  await S.p.screenshot({ path: `${OUT}/05-student-correct-reaction.png` });
  assert(await sounds(S.p) > s0, 'correct sound not played');
  ok(`Верный ответ, проверенный сервером: иконка ${icon.trim()} и звук; фон «Волны»`);
  await burst.waitFor({ state: 'detached' });
  // «Звук реакций» off: the icon still shows, no sound.
  await S.p.getByTestId('reaction-sound-toggle').click();
  const s1 = await sounds(S.p);
  await test.getByRole('radio', { name: 'is' }).nth(1).click();
  await burst.waitFor();
  assert(await burst.getAttribute('data-kind') === 'incorrect' && (await burst.innerText()).trim() === '🤔', 'incorrect reaction');
  await S.p.waitForTimeout(250);
  await S.p.screenshot({ path: `${OUT}/06-student-incorrect-reaction-muted.png` });
  assert(await sounds(S.p) === s1, 'sound played while switched off');
  ok('Неверный ответ: иконка 🤔; после «Звук реакций» → выключен звук не играет');
  await S.p.reload();
  await settle(S.p);
  assert(await S.p.getByTestId('reaction-sound-toggle').getAttribute('aria-pressed') === 'false', 'sound switch not remembered');
  await S.p.getByTestId('reaction-sound-toggle').click();
  // Re-checked answers after a reload do not react again.
  assert(await S.p.getByTestId('answer-reaction').count() === 0, 'reaction replayed on reload');
  ok('Выбор звука запоминается в браузере; после перезагрузки старые ответы не вызывают реакцию');

  // ---- Lesson 2 before and after «Оформление урока» ----
  await S.p.goto(`${base}/lesson/${L[1].id}`);
  await settle(S.p);
  assert(await S.p.getByTestId('lesson-background').getAttribute('data-preset') === 'waves', 'lesson appearance must wait for the marathon switch');
  ok('Своё оформление урока не действует, пока в марафоне выключено «Оформление урока»');
  await must(T.p, 'PUT', `/marathons/${m.id}/appearance`, { lesson_appearance: true, reactions_enabled: true, reactions: ap.appearance.reactions,
    background_enabled: true, background: 'waves' }, 200);
  // Realtime: the open lesson page re-reads the appearance without a reload.
  await S.p.waitForFunction(() => document.querySelector('[data-testid=lesson-background]')?.getAttribute('data-preset') === 'custom', null, { timeout: 10000 });
  assert(await S.p.getByTestId('reaction-sound-toggle').count() === 0, 'lesson without reactions shows the sound switch');
  const s2 = await sounds(S.p);
  await S.p.getByRole('article', { name: 'Тест' }).getByRole('radio', { name: 'is' }).first().click();
  await S.p.getByRole('article', { name: 'Тест' }).getByText('Верно').first().waitFor();
  await S.p.waitForTimeout(600);
  assert(await S.p.getByTestId('answer-reaction').count() === 0 && await sounds(S.p) === s2, 'disabled reactions played');
  await S.p.screenshot({ path: `${OUT}/07-student-lesson-own-background.png` });
  ok('«Оформление урока» включено: страница урока обновилась без перезагрузки — свой фон урока, реакции урока выключены и не срабатывают');
  await S.p.goto(`${base}/lesson/${L[0].id}`);
  await settle(S.p);
  assert(await S.p.getByTestId('lesson-background').getAttribute('data-preset') === 'waves', 'lesson 1 keeps the marathon appearance');
  ok('Урок 1 без своего оформления по-прежнему использует оформление марафона');

  // ---- Rights ----
  for (const [method, path, body] of [['GET', `/marathons/${m.id}/appearance`], ['PUT', `/marathons/${m.id}/appearance`, { lesson_appearance: false }],
    ['DELETE', `/marathons/${m.id}/images/cover`], ['PUT', `/marathons/${m.id}/lessons/${L[1].id}/appearance`, { enabled: false }]]) {
    assert((await call(S.p, method, path, body)).status === 403, `student ${method} ${path}`);
  }
  const up = await S.p.evaluate(async ({ API, id }) => {
    const f = new FormData(); f.append('file', new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' }), 'x.png');
    return (await fetch(`${API}/api/marathons/${id}/images/cover`, { method: 'POST', credentials: 'include', body: f })).status;
  }, { API, id: m.id });
  assert(up === 403, `student upload ${up}`);
  await S.p.goto(`${base}/students`);
  await settle(S.p);
  assert(await S.p.getByRole('button', { name: 'Настройки' }).count() === 0, 'student sees settings');
  ok('Права: ученик получает 403 на настройки оформления и загрузку, кнопки «Настройки» у него нет');

  // ---- Owner preview uses the appearance students get ----
  await T.p.goto(`${base}/preview/${L[0].id}`);
  await settle(T.p);
  assert(await T.p.getByTestId('lesson-background').getAttribute('data-preset') === 'waves', 'preview background');
  await T.p.getByRole('article', { name: 'Тест' }).getByRole('radio', { name: 'is' }).first().click();
  await T.p.getByTestId('answer-reaction').waitFor();
  await T.p.waitForTimeout(250);
  await T.p.screenshot({ path: `${OUT}/08-preview-reaction.png` });
  ok('Предпросмотр учителя: тот же фон и реакция на проверенный ответ');

  // ---- Phone width ----
  const SP = await login('student@ortalyq.local', 'student-local-123', { width: 390, height: 844 });
  await SP.p.goto(`${base}/lesson/${L[0].id}`);
  await settle(SP.p);
  assert(await SP.p.evaluate(() => document.documentElement.scrollWidth <= 390), 'student page scrolls sideways');
  await SP.p.getByRole('article', { name: 'Тест' }).getByRole('radio', { name: 'am' }).click();
  await SP.p.getByTestId('answer-reaction').waitFor();
  await SP.p.waitForTimeout(250);
  await SP.p.screenshot({ path: `${OUT}/09-phone-student-reaction.png` });
  const TP = await login('teacher@ortalyq.local', 'teacher-local-123', { width: 390, height: 844 });
  await TP.p.goto(`${base}/students`);
  await settle(TP.p);
  await TP.p.getByRole('button', { name: 'Настройки' }).click();
  await TP.p.getByRole('dialog').getByRole('tab', { name: 'Оформление' }).click();
  await TP.p.getByRole('dialog').getByTestId('marathon-bg-options').scrollIntoViewIfNeeded();
  assert(await TP.p.evaluate(() => document.documentElement.scrollWidth <= 390), 'settings scroll sideways');
  await TP.p.screenshot({ path: `${OUT}/10-phone-settings-appearance.png` });
  ok('Телефон 390 px: урок с реакцией и вкладка «Оформление» без горизонтальной прокрутки');

  // ---- Kazakh and English ----
  for (const [lang, tab, title] of [['kk', 'Безендіру', 'Жауаптарға реакциялар'], ['en', 'Appearance', 'Reactions to answers']]) {
    await must(T.p, 'PATCH', '/auth/me', { locale: lang }, 200); // the app follows the profile language
    await T.p.evaluate((l) => localStorage.setItem('ortalyq_locale', l), lang);
    await T.p.goto(`${base}/students`);
    await settle(T.p);
    await T.p.getByRole('button', { name: lang === 'kk' ? 'Баптаулар' : 'Settings' }).click();
    await T.p.getByRole('dialog').getByRole('tab', { name: tab }).click();
    await T.p.getByRole('dialog').getByText(title).first().waitFor();
    await T.p.screenshot({ path: `${OUT}/11-settings-appearance-${lang}.png` });
    await T.p.keyboard.press('Escape');
  }
  await must(T.p, 'PATCH', '/auth/me', { locale: 'ru' }, 200);
  await T.p.evaluate(() => localStorage.setItem('ortalyq_locale', 'ru'));
  ok('Вкладка «Оформление» на казахском и английском');

  // ---- Removing the cover brings back the generated one ----
  await T.p.goto(`${base}/students`);
  await settle(T.p);
  await T.p.getByRole('button', { name: 'Настройки' }).click();
  await T.p.getByRole('dialog').getByTestId('marathon-cover-image').getByRole('button', { name: 'Удалить картинку' }).click();
  await T.p.getByRole('dialog').getByTestId('marathon-cover-image').getByRole('button', { name: 'Загрузить картинку' }).waitFor();
  await T.p.keyboard.press('Escape');
  await settle(T.p);
  assert(await T.p.locator('header [data-cover-image]').count() === 0, 'cover still shown');
  assert(await S.p.evaluate(async ({ API, id }) => (await fetch(`${API}/api/attachments/${id}?inline=1`, { credentials: 'include' })).status, { API, id: cls.cover_image_id }) === 404, 'removed cover still served');
  await T.p.screenshot({ path: `${OUT}/12-cover-removed.png` });
  ok('«Удалить картинку»: снова обложка «цвет + буква», файл больше не отдаётся');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  ok('Ошибок в консоли нет');
  console.log(`Скриншоты: ${OUT}`);
} finally {
  await b.close();
}
