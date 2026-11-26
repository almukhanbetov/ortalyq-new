// «Материалы» → «Личные материалы» → «Создать материал» in the browser (the form of the reference: tabs «Основные»
// and «Теги»; cover, title, description, language, «Разделы»; «Отмена» and «Создать»).
// One click on the first card opens the form; «Основные» is open; the fields go in the reference order; labels are
// left of the fields on a wide screen and above them on a phone. The cover: a wrong file is refused, a picture is
// previewed; a failed cover upload and a failed save show a message and keep every value; the cover is not uploaded
// twice. Tags are added and removed; values survive switching tabs. The material is created; its card shows the
// cover; the form opened again (after a reload) shows every field and tag. «Разделы» off → a new lesson is one list;
// on → the lesson editor offers sections. «Отмена» creates nothing. A PDF uploaded earlier is still listed and opens;
// the page has no «Загрузить PDF». Plus: no console errors, no external requests.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/material-form-check.mjs
import { chromium, request } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { deflateSync } from 'node:zlib';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB, API } = stack;
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/materials';
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: process.env.HEADED !== '1' });
const settle = (p) => p.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {}).then(() => p.waitForTimeout(400));

/** A real PNG: a diagonal gradient with a light circle (no external images). */
function png(w, h) {
  const crcTable = Array.from({ length: 256 }, (_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
  const crc = (buf) => { let c = 0xffffffff; for (const x of buf) c = crcTable[(c ^ x) & 0xff] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
  const chunk = (type, data) => { const len = Buffer.alloc(4); len.writeUInt32BE(data.length); const td = Buffer.concat([Buffer.from(type), data]); const c = Buffer.alloc(4); c.writeUInt32BE(crc(td)); return Buffer.concat([len, td, c]); };
  const raw = Buffer.alloc((w * 3 + 1) * h);
  for (let y = 0; y < h; y++) {
    raw[y * (w * 3 + 1)] = 0;
    for (let x = 0; x < w; x++) {
      const i = y * (w * 3 + 1) + 1 + x * 3;
      const inCircle = (x - w * 0.7) ** 2 + (y - h * 0.35) ** 2 < (h * 0.22) ** 2;
      raw[i] = inCircle ? 255 : 40 + Math.round((x / w) * 60);
      raw[i + 1] = inCircle ? 214 : 120 + Math.round((y / h) * 80);
      raw[i + 2] = inCircle ? 102 : 230;
    }
  }
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = 2;
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]);
}

/** A small valid PDF (correct cross-reference table). */
function pdf(text) {
  const s = `BT /F1 28 Tf 60 700 Td (${text}) Tj ET`;
  const objs = ['<< /Type /Catalog /Pages 2 0 R >>', '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>',
    `<< /Length ${s.length} >>\nstream\n${s}\nendstream`, '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>'];
  let out = '%PDF-1.4\n';
  const offs = [];
  objs.forEach((o, i) => { offs.push(out.length); out += `${i + 1} 0 obj\n${o}\nendobj\n`; });
  const xref = out.length;
  out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n` + offs.map((o) => `${String(o).padStart(10, '0')} 00000 n \n`).join('');
  out += `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return Buffer.from(out, 'latin1');
}

async function login(email, password, viewport = { width: 1440, height: 1000 }) {
  const ctx = await b.newContext({ viewport, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of (4(01|03|04|09|13|22)|500)/.test(m.text())) errors.push(m.text()); });
  p.on('request', (r) => { const u = new URL(r.url()); if (/^https?:$/.test(u.protocol) && !['localhost', '127.0.0.1'].includes(u.hostname)) external.add(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль', { exact: true }).fill(password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL((u) => !u.pathname.startsWith('/login'));
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return p;
}

/** Labels of the form rows in their order on the page, and whether each label is left of its field. */
const rowLayout = (dlg) => dlg.locator('[role=tabpanel]:not([hidden]) > div').evaluateAll((rows) => rows.map((r) => {
  const label = r.querySelector('label').getBoundingClientRect();
  const field = r.children[1].getBoundingClientRect();
  return { label: r.querySelector('label').textContent.replace('*', '').trim(), left: label.right <= field.left + 1 && label.top < field.bottom, above: label.bottom <= field.top + 1 };
}));

try {
  const users = await makeUsers(stack, RUN, { students: [] });
  const T = users.teacher;
  const api = await apiAs(stack, T.email, T.password);

  // A PDF uploaded earlier (before this form): it must stay listed and viewable.
  const raw = await request.newContext({ baseURL: API, extraHTTPHeaders: { Origin: WEB } });
  assert((await raw.post('/api/auth/login', { data: { email: T.email, password: T.password } })).status() === 200, 'login for upload');
  const PDF_TITLE = `Handout ${RUN}`;
  const up = await raw.post('/api/documents', { multipart: { title: PDF_TITLE, file: { name: 'handout.pdf', mimeType: 'application/pdf', buffer: pdf('Handout for the lesson') } } });
  assert(up.status() === 201, `PDF upload: ${up.status()}`);
  const docId = (await up.json()).id;
  await raw.dispose();

  const p = await login(T.email, T.password);
  const main = p.locator('main');
  const grid = main.locator('ul.grid');
  const coverPosts = [];
  p.on('request', (r) => { if (r.method() === 'POST' && r.url() === `${API}/api/course-covers`) coverPosts.push(r.url()); });

  // ---- 1. One click on the first card opens «Создать материал» on «Основные» ----
  await p.locator('#app-sidebar').getByRole('link', { name: 'Материалы' }).click();
  await p.waitForURL(/\/materials$/);
  const first = grid.locator('> li').first().getByTestId('create-material-card');
  await first.waitFor();
  assert(await first.count() === 1 && /Создать материал/.test(await first.innerText()), 'first card «Создать материал»');
  assert(await main.getByRole('button', { name: /Загрузить PDF/ }).count() === 0, 'no «Загрузить PDF» on the page');
  await first.click();
  const dlg = p.getByRole('dialog', { name: 'Создать материал' });
  await dlg.waitFor();
  assert(await dlg.getByRole('tab', { name: 'Основные' }).getAttribute('aria-selected') === 'true', '«Основные» open by default');
  await dlg.getByRole('tab', { name: 'Теги' }).waitFor();
  const layout = await rowLayout(dlg);
  assert(layout.map((r) => r.label).join(' | ') === 'Обложка | Название | Описание | Язык | Разделы', `field order: ${layout.map((r) => r.label)}`);
  assert(layout.every((r) => r.left), `labels left of fields on a wide screen: ${JSON.stringify(layout)}`);
  const sw = dlg.getByRole('switch', { name: 'Разделы' });
  assert(await sw.getAttribute('aria-checked') === 'false', '«Разделы» off for a new material');
  assert(await dlg.getByRole('button', { name: 'Отмена' }).count() === 1 && await dlg.getByRole('button', { name: 'Создать', exact: true }).isDisabled(), 'footer «Отмена» / «Создать» (disabled without a title)');
  ok('one click on the first card opens «Создать материал» on «Основные»; fields in the reference order; labels on the left; «Разделы» off');

  // ---- 2. «Отмена» creates nothing ----
  await dlg.getByLabel('Название').fill(`Cancelled ${RUN}`);
  await dlg.getByRole('button', { name: 'Отмена' }).click();
  await dlg.waitFor({ state: 'detached' });
  assert((await api.must('GET', '/courses?limit=100', undefined, 200)).total === 0, 'cancel created a material');
  ok('«Отмена» closes the form without creating a material');

  // ---- 3. Cover: a wrong file is refused; a picture is previewed ----
  await first.click();
  await dlg.waitFor();
  assert(await dlg.getByLabel('Название').inputValue() === '', 'a new form starts empty');
  let fc = p.waitForEvent('filechooser');
  await dlg.getByRole('button', { name: 'Загрузить' }).click();
  await (await fc).setFiles({ name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('not a picture') });
  await dlg.getByTestId('material-cover-state').filter({ hasText: '«notes.txt» — не картинка' }).waitFor();
  fc = p.waitForEvent('filechooser');
  await dlg.getByRole('button', { name: 'Загрузить' }).click();
  await (await fc).setFiles({ name: 'cover.png', mimeType: 'image/png', buffer: png(480, 270) });
  const thumb = dlg.locator('[data-cover-image] img');
  await thumb.waitFor();
  assert((await thumb.getAttribute('src')).startsWith('blob:') && await thumb.evaluate((i) => i.complete && i.naturalWidth === 480), 'cover preview from the computer');
  ok('cover: «notes.txt» refused with a message; a PNG is previewed in the thumbnail');

  // ---- 4. Every field and the tags; values survive switching tabs ----
  const TITLE = `Speaking A2 ${RUN}`;
  const DESC = 'Разговорный курс: 12 уроков о путешествиях, покупках и работе.';
  await dlg.getByLabel('Название').fill(TITLE);
  await dlg.getByLabel('Описание').fill(DESC);
  await dlg.getByLabel('Язык').selectOption({ label: 'Английский' });
  await sw.click();
  assert(await sw.getAttribute('aria-checked') === 'true' && /Включено/.test(await dlg.locator('#material-sections-hint').innerText()), '«Разделы» on');
  await sw.click();
  assert(await sw.getAttribute('aria-checked') === 'false' && /Выключено/.test(await dlg.locator('#material-sections-hint').innerText()), '«Разделы» off again');
  await settle(p);
  await p.screenshot({ path: `${OUT}/material-form-main.png` });
  await dlg.getByRole('tab', { name: 'Теги' }).click();
  const tagInput = dlg.getByLabel('Новый тег');
  for (const tg of ['speaking', 'A2']) { await tagInput.fill(tg); await tagInput.press('Enter'); }
  await tagInput.fill('travel'); await dlg.getByRole('button', { name: 'Добавить' }).click();
  await tagInput.fill('temporary'); await tagInput.press('Enter');
  await tagInput.fill('A2'); await tagInput.press('Enter');
  await dlg.getByText('Тег «A2» уже добавлен').waitFor();
  await dlg.getByRole('button', { name: 'Удалить тег «temporary»' }).click();
  const tags = async () => (await dlg.getByTestId('material-tags').locator('li').allInnerTexts()).map((s) => s.trim());
  assert((await tags()).join(',') === 'speaking,A2,travel', `tags: ${await tags()}`);
  await settle(p);
  await p.screenshot({ path: `${OUT}/material-form-tags.png` });
  await dlg.getByRole('tab', { name: 'Основные' }).click();
  assert(await dlg.getByLabel('Название').inputValue() === TITLE && await dlg.getByLabel('Описание').inputValue() === DESC &&
    await dlg.getByLabel('Язык').inputValue() === 'en' && await thumb.count() === 1, 'values kept on «Основные»');
  await dlg.getByRole('tab', { name: 'Теги' }).click();
  assert((await tags()).join(',') === 'speaking,A2,travel', 'tags kept');
  await dlg.getByRole('tab', { name: 'Основные' }).click();
  ok('title, description, language, «Разделы» and tags (add by Enter and button, duplicate refused, remove) survive tab switches');

  // ---- 5. A failed cover upload, then a failed save: messages, data kept, no second cover upload ----
  await p.route(`${API}/api/course-covers`, (r) => r.fulfill({ status: 500, contentType: 'application/json', body: '{"error":{"code":"internal","message":"Сервер недоступен"}}' }), { times: 1 });
  await dlg.getByRole('button', { name: 'Создать', exact: true }).click();
  await dlg.getByTestId('material-form-error').filter({ hasText: 'Не удалось загрузить обложку' }).waitFor();
  assert(await dlg.getByLabel('Название').inputValue() === TITLE && await thumb.count() === 1, 'data kept after a cover error');
  await p.screenshot({ path: `${OUT}/material-form-cover-error.png` });
  await p.route(`${API}/api/courses`, (r) => (r.request().method() === 'POST'
    ? r.fulfill({ status: 500, contentType: 'application/json', body: '{"error":{"code":"internal","message":"Сервер недоступен"}}' }) : r.continue()), { times: 1 });
  await dlg.getByRole('button', { name: 'Создать', exact: true }).click();
  await dlg.getByTestId('material-form-error').filter({ hasText: 'Не удалось сохранить материал' }).waitFor();
  assert(await dlg.getByLabel('Описание').inputValue() === DESC, 'data kept after a save error');
  const postsBefore = coverPosts.length;
  const created = p.waitForResponse((r) => r.url() === `${API}/api/courses` && r.request().method() === 'POST');
  await dlg.getByRole('button', { name: 'Создать', exact: true }).click();
  assert((await created).status() === 201, 'created');
  assert(coverPosts.length === postsBefore && coverPosts.length === 2, `cover uploads: ${coverPosts.length} (the failed one and one more)`);
  await p.waitForURL(/\/materials\/courses\/[0-9a-f-]{36}$/);
  const coursePath = new URL(p.url()).pathname;
  ok('a failed cover upload and a failed save show a message and keep the form; the retry creates the material without uploading the cover again');

  // ---- 6. The course page and the card show the saved cover ----
  await main.getByRole('heading', { name: TITLE }).waitFor();
  await main.locator('[data-cover-image] img').waitFor();
  assert(await main.getByTestId('course-language').innerText() === 'Английский', 'language shown');
  assert((await main.getByTestId('course-tags').locator('li').allInnerTexts()).join(',') === 'speaking,A2,travel', 'tags shown');
  await p.goto(`${WEB}/materials`);
  const card = grid.locator('> li').filter({ hasText: TITLE });
  await card.locator('[data-cover-image] img').waitFor();
  assert(await card.locator('img').evaluate((i) => i.complete && i.naturalWidth === 480), 'card cover loaded');
  await settle(p);
  await p.screenshot({ path: `${OUT}/material-card.png` });
  ok('the card in «Личные материалы» shows the uploaded cover');

  // ---- 7. Reopened after a reload, the form shows every saved value ----
  await p.goto(`${WEB}${coursePath}`);
  await p.reload();
  await main.getByRole('button', { name: 'Редактировать' }).click();
  const ed = p.getByRole('dialog', { name: 'Редактировать материал' });
  await ed.getByLabel('Название').evaluate((el, v) => new Promise((res, rej) => { const t0 = Date.now(); const tick = () => (el.value === v ? res() : Date.now() - t0 > 5000 ? rej(new Error('prefill')) : requestAnimationFrame(tick)); tick(); }), TITLE);
  assert(await ed.getByLabel('Описание').inputValue() === DESC && await ed.getByLabel('Язык').inputValue() === 'en', 'description and language reopened');
  assert(await ed.getByRole('switch', { name: 'Разделы' }).getAttribute('aria-checked') === 'false', '«Разделы» reopened off');
  assert((await ed.locator('[data-cover-image] img').getAttribute('src')).includes('/api/course-covers/'), 'saved cover reopened');
  await ed.getByRole('tab', { name: 'Теги' }).click();
  assert((await ed.getByTestId('material-tags').locator('li').allInnerTexts()).map((s) => s.trim()).join(',') === 'speaking,A2,travel', 'tags reopened');
  await ed.getByRole('button', { name: 'Отмена' }).click();
  ok('after a reload the form shows the cover, title, description, language, «Разделы» and tags');

  // ---- 8. «Разделы»: off → a lesson is one list; on → the lesson editor offers sections ----
  await main.getByRole('button', { name: 'Добавить урок' }).click();
  await p.getByRole('dialog', { name: 'Добавить урок' }).getByLabel('Название урока').fill('Урок 1. At the airport');
  await p.getByRole('dialog', { name: 'Добавить урок' }).getByRole('button', { name: 'Сохранить' }).click();
  await main.getByRole('link', { name: /Урок 1\. At the airport/ }).click();
  await main.getByTestId('lesson-flat').waitFor();
  assert(await main.getByRole('button', { name: 'Добавить раздел' }).count() === 0, 'no «Добавить раздел» with «Разделы» off');
  assert(await main.getByRole('button', { name: 'Добавить упражнение' }).count() === 1, 'one «Добавить упражнение»');
  await settle(p);
  await p.screenshot({ path: `${OUT}/material-sections-off.png` });
  await p.goto(`${WEB}${coursePath}`);
  await main.getByRole('button', { name: 'Редактировать' }).click();
  await ed.getByRole('switch', { name: 'Разделы' }).click();
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.waitFor({ state: 'detached' });
  await main.getByText('уроки с разделами').waitFor();
  await main.getByRole('link', { name: /Урок 1\. At the airport/ }).click();
  await main.getByRole('button', { name: 'Добавить раздел' }).waitFor();
  assert(await main.getByTestId('lesson-flat').count() === 0, 'section editor with «Разделы» on');
  ok('«Разделы» off: the lesson is one list of exercises; switched on: the lesson editor shows sections and «Добавить раздел»');

  // ---- 9. The PDF uploaded earlier is listed and opens ----
  await p.goto(`${WEB}/materials`);
  const docCard = grid.getByTestId('document-card').filter({ hasText: PDF_TITLE });
  await docCard.waitFor();
  const pdfResp = p.waitForResponse((r) => r.url().includes(`/api/documents/${docId}/file`));
  await docCard.getByRole('link').click();
  await main.getByTestId('pdf-viewer').waitFor();
  const r = await pdfResp;
  assert(r.status() === 200 && r.headers()['content-type'] === 'application/pdf', `PDF viewer: ${r.status()}`);
  ok('the PDF uploaded earlier is still a card and opens in the viewer');

  // ---- 10. A phone: labels above the fields ----
  const phone = await login(T.email, T.password, { width: 390, height: 844 });
  await phone.goto(`${WEB}/materials`);
  await phone.locator('main').getByTestId('create-material-card').click();
  const pd = phone.getByRole('dialog', { name: 'Создать материал' });
  await pd.waitFor();
  const narrow = await rowLayout(pd);
  assert(narrow.every((x) => x.above), `labels above fields on a phone: ${JSON.stringify(narrow)}`);
  assert(await phone.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), 'no sideways scroll on a phone');
  await settle(phone);
  await phone.screenshot({ path: `${OUT}/material-form-phone.png` });
  ok('on a phone the labels stand above the fields; no sideways scroll');

  await api.dispose();
  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(', ')}`);
  ok('no console errors, no external requests');
  console.log('MATERIAL FORM CHECK PASSED');
} catch (e) {
  console.error('FAILED:', e.message, (e.stack || '').split('\n').find((l) => l.includes('material-form-check')));
  process.exitCode = 1;
} finally {
  await b.close();
}
