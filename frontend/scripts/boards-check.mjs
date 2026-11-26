// «Материалы» → «Интерактивные доски» in the browser. Teacher A: the tab next to «Личные материалы» → the
// empty list with «Создать доску» → the board «English A1 — Hello!» → text «Hello! My name is …» and a drawing → undo →
// save → a failed save keeps the changes and «Повторить сохранение» saves them → reload restores the drawing and the text → the
// list shows the board with its change date and finds it by title. Teacher B: the board is not in the list and its
// direct link shows «Доска не найдена» (the API answers 404); «Личные материалы» opens from the tabs.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/boards-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/boards/check-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const TITLE = 'English A1 — Hello!';
const TEXT = 'Hello! My name is …';
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
let injected = false; // the deliberately failed save logs a 500 in the console
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(300));

async function login(email, password) {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => {
    if (m.type() !== 'error' || /status of 4(01|03|04|09|22)/.test(m.text()) || (injected && /status of 500/.test(m.text()))) return;
    errors.push(m.text());
  });
  p.on('request', (r) => { const u = new URL(r.url()); if (/^https?:$/.test(u.protocol) && !['localhost', '127.0.0.1'].includes(u.hostname)) external.add(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль', { exact: true }).fill(password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL((u) => !u.pathname.startsWith('/login'));
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return p;
}

/** Draws a polyline on the canvas through points given in fractions of its size. */
async function draw(p, pts) {
  const box = await p.getByTestId('board-canvas').boundingBox();
  const at = ([fx, fy]) => [box.x + fx * box.width, box.y + fy * box.height];
  await p.mouse.move(...at(pts[0]));
  await p.mouse.down();
  for (const pt of pts.slice(1)) await p.mouse.move(...at(pt), { steps: 8 });
  await p.mouse.up();
}
const strokes = (p) => p.locator('[data-testid="board-canvas"] [data-item="stroke"]').count();
const status = (p) => p.getByTestId('board-status').innerText();

try {
  const A = (await makeUsers(stack, RUN, { students: [] })).teacher;
  const B = (await makeUsers(stack, RUN + 'b', { teacher: 'Другой учитель', students: [] })).teacher;

  // ---- 1. The tab and the empty list ----
  const p = await login(A.email, A.password);
  await p.goto(`${WEB}/materials`);
  const tabs = p.getByRole('navigation', { name: 'Разделы материалов' });
  await tabs.getByRole('link', { name: 'Личные материалы' }).waitFor();
  await tabs.getByRole('link', { name: 'Интерактивные доски' }).click();
  await p.waitForURL((u) => u.pathname === '/materials/boards');
  await p.getByText('У вас пока нет интерактивных досок').waitFor();
  await p.screenshot({ path: `${OUT}/01-boards-empty.png` });
  ok('«Интерактивные доски» next to «Личные материалы»; the empty list offers «Создать доску»');

  // ---- 2. Create and draw ----
  await p.getByRole('button', { name: 'Создать доску' }).click();
  const dlg = p.getByRole('dialog', { name: 'Создать доску' });
  await dlg.getByLabel('Название доски').fill(TITLE);
  await dlg.getByRole('button', { name: 'Сохранить' }).click();
  await p.waitForURL((u) => /^\/materials\/boards\/[0-9a-f-]{36}$/.test(u.pathname));
  const boardPath = new URL(p.url()).pathname;
  const boardId = boardPath.split('/').pop();
  await p.getByRole('heading', { name: TITLE }).waitFor();
  assert((await status(p)).includes('Все изменения сохранены'), 'a new board is saved');

  await p.getByRole('button', { name: 'Текст', exact: true }).click();
  await p.getByRole('radio', { name: 'Синий' }).click();
  const box = await p.getByTestId('board-canvas').boundingBox();
  await p.mouse.click(box.x + box.width * 0.08, box.y + box.height * 0.1);
  await p.getByRole('textbox', { name: 'Текст на доске' }).fill(TEXT);
  await p.keyboard.press('Enter');
  await p.locator('[data-item="text"]', { hasText: TEXT }).waitFor();

  await p.getByRole('button', { name: 'Карандаш' }).click();
  await p.getByRole('radio', { name: 'Красный' }).click();
  // A house: walls and a roof.
  await draw(p, [[0.3, 0.8], [0.3, 0.55], [0.5, 0.55], [0.5, 0.8], [0.3, 0.8]]);
  await draw(p, [[0.28, 0.56], [0.4, 0.4], [0.52, 0.56]]);
  await p.getByRole('radio', { name: 'Зелёный' }).click();
  await draw(p, [[0.6, 0.8], [0.9, 0.8]]);
  assert(await strokes(p) === 3, 'three strokes');
  // Undo removes the last action only.
  await draw(p, [[0.7, 0.3], [0.8, 0.2]]);
  assert(await strokes(p) === 4, 'an extra stroke');
  await p.getByRole('button', { name: 'Отменить' }).click();
  assert(await strokes(p) === 3, 'undo removes the last stroke');
  assert((await status(p)).includes('Есть несохранённые изменения'), 'unsaved status');
  await p.getByRole('button', { name: 'Сохранить' }).click();
  await p.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  await settle(p);
  await p.screenshot({ path: `${OUT}/02-board-editor.png` });
  ok('the board «English A1 — Hello!»: blue text «Hello! My name is …», a red house and a green line; undo removes the last stroke; saved');

  // ---- 3. A failed save keeps the changes; «Повторить» saves them ----
  injected = true;
  await p.route('**/api/boards/*', (r) => (r.request().method() === 'PUT' ? r.fulfill({ status: 500, contentType: 'application/json', body: '{"error":{"code":"internal","message":"Сервер недоступен"}}' }) : r.continue()));
  await p.getByRole('radio', { name: 'Чёрный' }).click();
  await draw(p, [[0.6, 0.3], [0.65, 0.45], [0.7, 0.3]]);
  await p.getByRole('button', { name: 'Сохранить' }).click();
  await p.getByText('Не удалось сохранить доску. Изменения остались в редакторе.').waitFor();
  assert(await strokes(p) === 4 && (await status(p)).includes('Есть несохранённые изменения'), 'changes kept after a failed save');
  await p.screenshot({ path: `${OUT}/03-save-failed.png` });
  await p.unroute('**/api/boards/*');
  await p.getByRole('button', { name: 'Повторить сохранение' }).click();
  await p.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  assert(!(await p.getByText('Не удалось сохранить доску').count()), 'the error is gone');
  injected = false;
  ok('a failed save keeps the drawing in the editor with «Повторить сохранение»; the retry saves it');

  // ---- 4. Reload restores the drawing and the text ----
  await p.reload();
  await p.locator('[data-item="text"]', { hasText: TEXT }).waitFor();
  assert(await strokes(p) === 4, `strokes after reload: ${await strokes(p)}`);
  assert((await p.locator('[data-item="text"]').first().getAttribute('fill')) === '#2F80ED', 'text colour kept');
  await settle(p);
  await p.screenshot({ path: `${OUT}/04-board-reopened.png` });
  ok('after a reload the 4 strokes and the text are back, with their colours');

  // ---- 5. The list: title, change date, search ----
  await p.getByRole('link', { name: 'К доскам' }).click();
  const card = p.getByRole('listitem').filter({ hasText: TITLE });
  await card.waitFor();
  assert(/Изменена \d{1,2} \S+/.test(await card.innerText()), `change date on the card: ${await card.innerText()}`);
  await p.getByRole('searchbox', { name: 'Поиск досок' }).fill('hello');
  await p.waitForURL((u) => u.searchParams.get('q') === 'hello');
  await card.waitFor();
  await p.screenshot({ path: `${OUT}/05-boards-list.png` });
  await p.getByRole('searchbox', { name: 'Поиск досок' }).fill('numbers');
  await p.getByText('Доски не найдены').waitFor();
  await p.getByRole('searchbox', { name: 'Поиск досок' }).fill('');
  await card.getByRole('link', { name: TITLE }).click();
  await p.waitForURL((u) => u.pathname === boardPath);
  ok('the list shows the board with «Изменена …», finds it by «hello», not by «numbers», and opens it again');

  // ---- 6. Teacher B: no board in the list, closed by a direct link ----
  const q = await login(B.email, B.password);
  await q.goto(`${WEB}/materials/boards`);
  await q.getByText('У вас пока нет интерактивных досок').waitFor();
  assert(!(await q.getByText(TITLE).count()), 'B does not see A\'s board');
  await q.goto(`${WEB}${boardPath}`);
  await q.getByText('Доска не найдена').waitFor();
  assert(!(await q.getByText(TEXT).count()) && !(await q.getByTestId('board-canvas').count()), 'no drawing for B');
  await q.screenshot({ path: `${OUT}/06-other-teacher-direct-link.png` });
  const apiB = await apiAs(stack, B.email, B.password);
  const r = await apiB.call('GET', `/boards/${boardId}`);
  const w = await apiB.call('PUT', `/boards/${boardId}`, { content: { items: [] }, version: 1, force: true });
  assert(r.status === 404 && w.status === 404, `B by API: GET ${r.status}, PUT ${w.status}`);
  ok('another teacher: the board is not in the list, the direct link shows «Доска не найдена», the API answers 404 to read and save');

  // ---- 7. «Личные материалы» still opens from the tabs ----
  await q.goto(`${WEB}/materials/boards`);
  await q.getByRole('navigation', { name: 'Разделы материалов' }).getByRole('link', { name: 'Личные материалы' }).click();
  await q.waitForURL((u) => u.pathname === '/materials');
  await q.getByRole('heading', { level: 1 }).waitFor();
  ok('«Личные материалы» opens from the boards tabs');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`teacher A: ${A.email} / ${A.password}; teacher B: ${B.email} / ${B.password}`);
  console.log(`board: ${WEB}${boardPath}`);
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
