// A personal board in a lesson, in two browser sessions at once. The student is already in the class: «Доска» is off.
// The teacher opens the board with «Открыть в классе» → the student's lesson shows it without a reload. The teacher
// draws, writes and adds a picture in the lesson (saved by itself) → the student sees each change without a reload;
// after a reload the student sees the board again. A student of another class gets 404 for the board and its picture.
// «Закрыть доску в классе» → the student's view goes back to the lesson and the API refuses the picture; the board
// stays in the teacher's «Материалы». Plus: no console errors, no external requests.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/class-board-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { deflateSync } from 'node:zlib';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/boards/class-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const TITLE = 'English A1 — Hello!';
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(300));

function png(w, h, [r, g, bl]) {
  const table = Array.from({ length: 256 }, (_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
  const crc = (buf) => { let c = 0xffffffff; for (const x of buf) c = table[(c ^ x) & 0xff] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
  const chunk = (type, data) => {
    const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
    const td = Buffer.concat([Buffer.from(type), data]);
    const c = Buffer.alloc(4); c.writeUInt32BE(crc(td));
    return Buffer.concat([len, td, c]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = 2;
  const row = Buffer.concat([Buffer.from([0]), Buffer.alloc(w * 3).map((_, i) => [r, g, bl][i % 3])]);
  return Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(Buffer.concat(Array.from({ length: h }, () => row)))), chunk('IEND', Buffer.alloc(0))]);
}

async function login(email, password) {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 1100 }, locale: 'ru-RU' });
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
  return p;
}
const viewer = (p) => p.getByTestId('class-board-student');
const vItems = (p, type) => p.locator(`[data-testid="board-viewer"] [data-item="${type}"]`);
const tItems = (p, type) => p.locator(`[data-testid="board-canvas"] [data-item="${type}"]`);

try {
  const users = await makeUsers(stack, RUN, { students: ['Данияр', 'Алия'] });
  const [S1, S2] = users.students;
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const clsA = await api.must('POST', '/classrooms', { name: `Английский — Данияр ${RUN}`, student_id: S1.id }, 201);
  const clsB = await api.must('POST', '/classrooms', { name: `Английский — Алия ${RUN}`, student_id: S2.id }, 201);
  const board = await api.must('POST', '/boards', { title: TITLE }, 201);

  // ---- 1. The student waits in the lesson: no board yet ----
  const sp = await login(S1.email, S1.password);
  const studentImages = [];
  sp.on('response', (r) => { if (r.url().includes(`/classrooms/${clsA.id}/board/images/`)) studentImages.push(r.status()); });
  await sp.goto(`${WEB}/classes/${clsA.id}`);
  const boardTool = sp.locator('#classroom-toolbar').getByRole('button', { name: /^Доска/ });
  await boardTool.waitFor();
  assert((await boardTool.getAttribute('aria-disabled')) === 'true', 'the student\'s «Доска» is off before the teacher opens one');
  ok('the student is in the lesson; «Доска» is off (the teacher has not opened a board)');

  // ---- 2. The teacher: «Открыть в классе» in the editor ----
  const tp = await login(users.teacher.email, users.teacher.password);
  await tp.goto(`${WEB}/materials/boards/${board.id}`);
  await tp.getByRole('button', { name: 'Открыть в классе' }).click();
  const dlg = tp.getByRole('dialog', { name: 'Открыть в классе' });
  await dlg.getByText(clsA.name).click();
  await tp.screenshot({ path: `${OUT}/01-teacher-open-in-class.png` });
  await dlg.getByRole('button', { name: 'Открыть', exact: true }).click();
  await tp.getByText(`Доска открыта в классе «${clsA.name}»`).first().waitFor();
  ok('the teacher picks the class in «Открыть в классе»; the editor shows where the board is open');

  // ---- 3. The student sees the board without a reload ----
  await viewer(sp).waitFor();
  assert((await viewer(sp).innerText()).includes(`Доска «${TITLE}»`) && (await viewer(sp).innerText()).includes('Только просмотр'), 'student header');
  ok('the board appears in the student\'s lesson by itself (view only)');

  // ---- 4. The teacher draws in the lesson; the student sees each change ----
  await tp.getByRole('link', { name: 'Перейти в класс' }).click();
  await tp.waitForURL((u) => u.pathname === `/classes/${clsA.id}`);
  await tp.getByText('Доска открыта в этом занятии').waitFor();
  const canvas = tp.getByTestId('board-canvas');
  const cb = await canvas.boundingBox();
  await tp.mouse.move(cb.x + cb.width * 0.1, cb.y + cb.height * 0.75);
  await tp.mouse.down();
  await tp.mouse.move(cb.x + cb.width * 0.35, cb.y + cb.height * 0.55, { steps: 10 });
  await tp.mouse.up();
  await vItems(sp, 'stroke').first().waitFor({ state: 'attached', timeout: 10000 });
  ok('a pen stroke of the teacher reaches the student without a reload');
  await tp.getByRole('toolbar', { name: 'Инструменты доски' }).getByRole('button', { name: 'Текст', exact: true }).click();
  await tp.mouse.click(cb.x + cb.width * 0.06, cb.y + cb.height * 0.08);
  await tp.getByRole('textbox', { name: 'Текст на доске' }).fill('Hello! My name is …');
  await tp.keyboard.press('Enter');
  await vItems(sp, 'text').filter({ hasText: 'Hello! My name is …' }).waitFor({ timeout: 10000 });
  await tp.getByTestId('board-image-input').setInputFiles({ name: 'sun.png', mimeType: 'image/png', buffer: png(160, 100, [245, 158, 11]) });
  await tItems(tp, 'image').waitFor();
  await vItems(sp, 'image').waitFor({ timeout: 10000 });
  await tp.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  await sp.waitForTimeout(800);
  assert(studentImages.includes(200), `the student loads the picture through the class: ${studentImages}`);
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/02-teacher-in-class.png` });
  await settle(sp);
  await sp.screenshot({ path: `${OUT}/03-student-sees-board.png` });
  ok('the text and the picture reach the student without a reload; the picture comes through the class (200)');

  // ---- 5. The student reloads: the board is still there ----
  studentImages.length = 0;
  await sp.reload();
  await viewer(sp).waitFor();
  await vItems(sp, 'text').filter({ hasText: 'Hello! My name is …' }).waitFor();
  assert(await vItems(sp, 'stroke').count() === 1 && await vItems(sp, 'image').count() === 1, 'after reload: stroke and picture');
  await sp.waitForTimeout(800);
  assert(studentImages.includes(200), 'the picture after reload');
  // No tools for the student, and no writes through the API.
  assert(await sp.getByTestId('board-toolbar').count() === 0 && await sp.getByTestId('board-canvas').count() === 0, 'no editor for the student');
  const apiS1 = await apiAs(stack, S1.email, S1.password);
  const write = await apiS1.call('PUT', `/boards/${board.id}`, { version: 99, force: true, content: { items: [] } });
  const closeTry = await apiS1.call('DELETE', `/classrooms/${clsA.id}/board`);
  assert(write.status === 403 && closeTry.status === 403, `student writes: ${write.status} ${closeTry.status}`);
  await sp.screenshot({ path: `${OUT}/04-student-after-reload.png` });
  ok('after a reload the student sees the board with the stroke, the text and the picture; the student cannot edit or close it (403)');

  // ---- 6. A student of another class: nothing ----
  const pic = (await api.must('GET', `/boards/${board.id}`, undefined, 200)).content.items.find((x) => x.type === 'image');
  const apiS2 = await apiAs(stack, S2.email, S2.password);
  const s2Board = await apiS2.call('GET', `/classrooms/${clsA.id}/board`);
  const s2Pic = await apiS2.call('GET', `/classrooms/${clsA.id}/board/images/${pic.image_id}`);
  const s2Own = await apiS2.must('GET', `/classrooms/${clsB.id}/board`, undefined, 200);
  const s2Direct = await apiS2.call('GET', `/boards/${board.id}/images/${pic.image_id}`);
  assert(s2Board.status === 404 && s2Pic.status === 404 && s2Own.board === null && s2Direct.status === 403, `other class: ${s2Board.status} ${s2Pic.status} ${JSON.stringify(s2Own)} ${s2Direct.status}`);
  const s2p = await login(S2.email, S2.password);
  await s2p.goto(`${WEB}/classes/${clsA.id}`);
  await s2p.waitForTimeout(1500);
  assert(await viewer(s2p).count() === 0, 'another class student does not see the board in the browser');
  await s2p.screenshot({ path: `${OUT}/05-other-class-student.png` });
  ok('a student of another class: 404 for the board and its picture, the own class has no board, the page shows nothing');

  // ---- 7. «Закрыть доску в классе» ----
  await tp.getByRole('button', { name: 'Закрыть доску в классе' }).click();
  await tp.getByText('Доска закрыта в классе').waitFor();
  await viewer(sp).waitFor({ state: 'detached', timeout: 10000 });
  const after = await apiS1.must('GET', `/classrooms/${clsA.id}/board`, undefined, 200);
  const afterPic = await apiS1.call('GET', `/classrooms/${clsA.id}/board/images/${pic.image_id}`);
  assert(after.board === null && afterPic.status === 404, `after closing: ${JSON.stringify(after)} ${afterPic.status}`);
  await sp.reload();
  await sp.locator('#classroom-toolbar').getByRole('button', { name: /^Доска/ }).waitFor();
  assert(await viewer(sp).count() === 0, 'no board after a reload');
  await sp.screenshot({ path: `${OUT}/06-student-after-close.png` });
  ok('«Закрыть доску в классе»: the board leaves the student\'s lesson by itself; the API refuses the board and the picture');

  // ---- 8. The board stays in the teacher's «Материалы» ----
  await tp.goto(`${WEB}/materials/boards`);
  await tp.getByRole('listitem').filter({ hasText: TITLE }).getByRole('link', { name: TITLE }).click();
  await tItems(tp, 'text').filter({ hasText: 'Hello! My name is …' }).waitFor();
  assert(await tItems(tp, 'stroke').count() === 1 && await tItems(tp, 'image').count() === 1, 'saved board in «Материалы»');
  assert(await tp.getByText(/Доска открыта в классе/).count() === 0, 'not open anywhere');
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/07-teacher-materials-after-close.png` });
  ok('the teacher opens the saved board in «Материалы» with the stroke, the text and the picture');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`teacher: ${users.teacher.email} / ${users.teacher.password}`);
  console.log(`student: ${S1.email} / ${S1.password}; other class: ${S2.email} / ${S2.password}`);
  console.log(`class: ${WEB}/classes/${clsA.id}; board: ${WEB}/materials/boards/${board.id}`);
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
