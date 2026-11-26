// «Создать копию для занятия» in two browser sessions. A separate test board with a picture → «Открыть в классе» with
// the option on → the teacher lands on the copy «… — class — date»; the same request sent again returns the same
// copy (no duplicate). The student sees the copy with its picture. Both draw a stroke in the lesson; the teacher closes
// the board in the class: the student's access ends. In «Материалы» the original has the picture and no strokes, the
// copy has the picture and both strokes; both pictures load.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/board-copy-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { deflateSync } from 'node:zlib';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/boards/copy-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const TITLE = `Тестовая доска с картинкой ${RUN}`;
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
async function drag(p, el, pts) {
  const box = await el.boundingBox();
  const at = ([fx, fy]) => [box.x + fx * box.width, box.y + fy * box.height];
  await p.mouse.move(...at(pts[0]));
  await p.mouse.down();
  for (const pt of pts.slice(1)) await p.mouse.move(...at(pt), { steps: 12 });
  await p.mouse.up();
}
const tItems = (p, type) => p.locator(`[data-testid="board-canvas"] [data-item="${type}"]`);
const sItems = (p, type) => p.locator(`[data-testid="board-viewer"] [data-item="${type}"]`);
/** Waits until the <image> of the editor or viewer has really loaded (its response was 200). */
async function pictureLoads(p, trigger) {
  const resp = p.waitForResponse((r) => /\/images\/[0-9a-f-]{36}$/.test(new URL(r.url()).pathname) && r.request().method() === 'GET');
  await trigger();
  return (await resp).status();
}

try {
  const users = await makeUsers(stack, RUN, { students: ['Данияр'] });
  const [S] = users.students;
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const cls = await api.must('POST', '/classrooms', { name: `Английский — Данияр ${RUN}`, student_id: S.id }, 201);
  const orig = await api.must('POST', '/boards', { title: TITLE }, 201);

  // ---- 1. The test board with a picture ----
  const tp = await login(users.teacher.email, users.teacher.password);
  await tp.goto(`${WEB}/materials/boards/${orig.id}`);
  await tp.getByTestId('board-image-input').setInputFiles({ name: 'sun.png', mimeType: 'image/png', buffer: png(160, 100, [245, 158, 11]) });
  await tItems(tp, 'image').waitFor();
  await tp.getByRole('button', { name: 'Сохранить' }).click();
  await tp.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  const origSaved = await api.must('GET', `/boards/${orig.id}`, undefined, 200);
  assert(origSaved.content.items.length === 1 && origSaved.content.items[0].type === 'image', 'original: one picture');
  ok(`the test board «${TITLE}» has a picture`);

  // ---- 2. The student waits in the lesson ----
  const sp = await login(S.email, S.password);
  await sp.goto(`${WEB}/classes/${cls.id}`);
  await sp.locator('#classroom-toolbar').getByRole('button', { name: /^Доска/ }).waitFor();

  // ---- 3. «Открыть в классе» with «Создать копию для занятия» ----
  let sent = null;
  tp.on('request', (r) => { if (r.url().endsWith(`/classrooms/${cls.id}/board`) && r.method() === 'PUT') sent = r.postDataJSON(); });
  await tp.getByRole('button', { name: 'Открыть в классе' }).click();
  const dlg = tp.getByRole('dialog', { name: 'Открыть в классе' });
  await dlg.getByText(cls.name).click();
  await dlg.getByRole('switch', { name: 'Создать копию для занятия' }).click();
  await tp.screenshot({ path: `${OUT}/01-open-with-copy.png` });
  await dlg.getByRole('button', { name: 'Открыть', exact: true }).click();
  await tp.waitForURL((u) => /\/materials\/boards\/[0-9a-f-]{36}$/.test(u.pathname) && !u.pathname.endsWith(orig.id));
  const copyId = new URL(tp.url()).pathname.split('/').pop();
  const day = new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', timeZone: 'Asia/Almaty' }).format(new Date());
  const copyTitle = `${TITLE} — ${cls.name} — ${day}`;
  await tp.getByRole('heading', { name: copyTitle }).waitFor();
  await tp.getByText(`Доска открыта в классе «${cls.name}»`).waitFor();
  assert(sent && sent.copy === true && sent.request_id && sent.board_id === orig.id, `request: ${JSON.stringify(sent)}`);
  assert(await tItems(tp, 'image').count() === 1, 'the copy has the picture');
  ok(`with the option on, the copy «${copyTitle}» is made, opened in the class, and the teacher lands on it`);

  // The same request again (a resend): the same copy, no second one.
  const again = await api.must('PUT', `/classrooms/${cls.id}/board`, sent, 200);
  const boards = await api.must('GET', '/boards', undefined, 200);
  assert(again.copy.id === copyId && again.board.id === copyId && boards.total === 2, `repeat: ${again.copy?.id} / ${boards.total}`);
  ok('the same request sent again returns the same copy: the teacher has 2 boards (original + copy)');

  // ---- 4. The student sees the copy with its picture ----
  await sp.getByTestId('class-board-student').waitFor({ timeout: 10000 });
  await sp.getByTestId('class-board-student').getByText(`Доска «${copyTitle}»`).waitFor();
  await sItems(sp, 'image').waitFor();
  const studentPic = await pictureLoads(sp, () => sp.reload());
  await sItems(sp, 'image').waitFor();
  assert(studentPic === 200, `student picture: ${studentPic}`);
  ok('the student sees the copy with its picture (loaded through the class: 200)');

  // ---- 5. A stroke from the teacher and from the student ----
  await tp.getByRole('link', { name: 'Перейти в класс' }).click();
  await tp.waitForURL((u) => u.pathname === `/classes/${cls.id}`);
  const panel = tp.getByRole('region', { name: 'Рисование учеников' });
  await panel.getByRole('button', { name: 'Разрешить рисовать' }).click();
  await sp.getByTestId('student-board-toolbar').waitFor();
  await drag(tp, tp.getByTestId('board-canvas'), [[0.1, 0.8], [0.3, 0.7], [0.45, 0.85]]);
  await sp.getByTestId('student-board-toolbar').getByRole('radio', { name: 'Красный' }).click();
  await drag(sp, sp.getByTestId('board-viewer'), [[0.6, 0.8], [0.75, 0.7], [0.9, 0.85]]);
  let copyNow;
  for (let i = 0; i < 40; i++) {
    copyNow = await api.must('GET', `/boards/${copyId}`, undefined, 200);
    if (copyNow.content.items.filter((x) => x.type === 'stroke').length === 2) break;
    await tp.waitForTimeout(250);
  }
  assert(copyNow.content.items.filter((x) => x.type === 'stroke').length === 2, `copy strokes: ${JSON.stringify(copyNow.content.items.map((x) => x.type))}`);
  await tp.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/02-teacher-lesson-on-copy.png` });
  await settle(sp);
  await sp.screenshot({ path: `${OUT}/03-student-lesson-on-copy.png` });
  ok('a stroke of the teacher and a stroke of the student are saved in the copy');

  // ---- 6. Closing the board in the class ----
  const copyPic = copyNow.content.items.find((x) => x.type === 'image').image_id;
  await tp.getByRole('button', { name: 'Закрыть доску в классе' }).click();
  await sp.getByTestId('class-board-student').waitFor({ state: 'detached', timeout: 10000 });
  const apiS = await apiAs(stack, S.email, S.password);
  const after = await apiS.must('GET', `/classrooms/${cls.id}/board`, undefined, 200);
  const afterPic = await apiS.call('GET', `/classrooms/${cls.id}/board/images/${copyPic}`);
  const direct = await apiS.call('GET', `/boards/${copyId}`);
  assert(after.board === null && afterPic.status === 404 && direct.status === 403, `student after closing: ${JSON.stringify(after)} ${afterPic.status} ${direct.status}`);
  await sp.screenshot({ path: `${OUT}/04-student-after-close.png` });
  ok('after closing, the student has no board in the lesson; the copy and its picture are closed to the student (404 / 403)');

  // ---- 7. «Материалы»: the original and the copy ----
  await tp.goto(`${WEB}/materials/boards`);
  await tp.getByRole('listitem').filter({ hasText: copyTitle }).waitFor();
  await tp.getByRole('listitem').filter({ hasText: TITLE }).first().waitFor();
  await tp.screenshot({ path: `${OUT}/05-materials-both-boards.png` });
  const origPic = await pictureLoads(tp, () => tp.goto(`${WEB}/materials/boards/${orig.id}`));
  await tItems(tp, 'image').waitFor();
  assert(origPic === 200 && await tItems(tp, 'stroke').count() === 0, `original: picture ${origPic}, strokes ${await tItems(tp, 'stroke').count()}`);
  assert(await tp.getByText(/Доска открыта в классе/).count() === 0, 'the original is not open anywhere');
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/06-original.png` });
  const cPic = await pictureLoads(tp, () => tp.goto(`${WEB}/materials/boards/${copyId}`));
  await tItems(tp, 'image').waitFor();
  const strokeColors = await tItems(tp, 'stroke').evaluateAll((els) => els.map((g) => g.querySelectorAll('path')[1].getAttribute('stroke')).sort());
  assert(cPic === 200 && strokeColors.length === 2 && strokeColors.includes('#E5484D') && strokeColors.includes('#1F2937'), `copy: picture ${cPic}, strokes ${strokeColors}`);
  assert(await tp.getByText(/Доска открыта в классе/).count() === 0, 'the copy is not open anywhere');
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/07-copy.png` });
  const origFinal = await api.must('GET', `/boards/${orig.id}`, undefined, 200);
  assert(origFinal.version === origSaved.version && origFinal.content.items.length === 1, 'the original never changed');
  assert(origFinal.content.items[0].image_id !== copyNow.content.items.find((x) => x.type === 'image').image_id, 'the copy has its own picture');
  ok('in «Материалы» the original has the picture and no strokes (unchanged); the copy has the picture and both strokes; both pictures load');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`teacher: ${users.teacher.email} / ${users.teacher.password}; student: ${S.email} / ${S.password}`);
  console.log(`original: ${WEB}/materials/boards/${orig.id}; copy: ${WEB}/materials/boards/${copyId}`);
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
