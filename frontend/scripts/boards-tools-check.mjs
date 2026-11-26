// The whiteboard editor tools in the browser. A new board with every object type: pen, text, sticky note, rectangle,
// ellipse, arrow and an uploaded picture. Zoom «+», «−» and «100%»; «Рука» moves the view and leaves objects in place;
// after panning and zooming the pen starts exactly under the pointer. «Выделение» selects, moves, resizes and deletes;
// every change is undone and redone (moving, resizing, editing a sticky, deleting). Save, reload: all objects return.
// An old board (in the format of the first editor: no object ids) opens unchanged, is not marked dirty and saves again.
// Another teacher cannot read the picture. Plus: no console errors, no external requests.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/boards-tools-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { deflateSync } from 'node:zlib';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/boards/tools-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(300));

/** A solid-colour PNG (no image library needed). */
function png(w, h, [r, g, bl]) {
  const crcTable = Array.from({ length: 256 }, (_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
  const crc = (buf) => { let c = 0xffffffff; for (const x of buf) c = crcTable[(c ^ x) & 0xff] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
  const chunk = (type, data) => {
    const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
    const td = Buffer.concat([Buffer.from(type), data]);
    const c = Buffer.alloc(4); c.writeUInt32BE(crc(td));
    return Buffer.concat([len, td, c]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = 2;
  const row = Buffer.concat([Buffer.from([0]), Buffer.alloc(w * 3).map((_, i) => [r, g, bl][i % 3])]);
  const raw = Buffer.concat(Array.from({ length: h }, () => row));
  return Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]);
}

async function login(email, password) {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 1150 }, locale: 'ru-RU' });
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

const canvas = (p) => p.getByTestId('board-canvas');
const items = (p, type) => p.locator(`[data-testid="board-canvas"] [data-item="${type}"]`);
const tool = (p, name) => p.getByRole('toolbar', { name: 'Инструменты доски' }).getByRole('button', { name, exact: true });
const status = (p) => p.getByTestId('board-status').innerText();
const zoomText = (p) => p.getByTestId('board-zoom').innerText();

/** Screen point of a board point (through the SVG's own transform: the truth of what is drawn where). */
const screenOf = (p, x, y) => p.evaluate(([x, y]) => {
  const svg = document.querySelector('[data-testid="board-canvas"]');
  const pt = svg.createSVGPoint(); pt.x = x; pt.y = y;
  const s = pt.matrixTransform(svg.getScreenCTM());
  return [s.x, s.y];
}, [x, y]);
/** Board point under a screen point. */
const boardOf = (p, sx, sy) => p.evaluate(([sx, sy]) => {
  const svg = document.querySelector('[data-testid="board-canvas"]');
  const pt = svg.createSVGPoint(); pt.x = sx; pt.y = sy;
  const s = pt.matrixTransform(svg.getScreenCTM().inverse());
  return [s.x, s.y];
}, [sx, sy]);
/** Drags on the canvas between two points given in fractions of its size. */
async function dragFrac(p, a, c, steps = 10) {
  const box = await canvas(p).boundingBox();
  await p.mouse.move(box.x + a[0] * box.width, box.y + a[1] * box.height);
  await p.mouse.down();
  await p.mouse.move(box.x + c[0] * box.width, box.y + c[1] * box.height, { steps });
  await p.mouse.up();
}
async function dragScreen(p, a, c, steps = 10) {
  await p.mouse.move(a[0], a[1]);
  await p.mouse.down();
  await p.mouse.move(c[0], c[1], { steps });
  await p.mouse.up();
}
const attrs = (loc, names) => loc.evaluate((el, names) => Object.fromEntries(names.map((n) => [n, Number(el.getAttribute(n))])), names);

try {
  const A = (await makeUsers(stack, RUN, { students: [] })).teacher;
  const B = (await makeUsers(stack, RUN + 'b', { teacher: 'Другой учитель', students: [] })).teacher;
  const api = await apiAs(stack, A.email, A.password);
  const board = await api.must('POST', '/boards', { title: 'English A1 — Shapes' }, 201);
  const p = await login(A.email, A.password);
  const imageResponses = [];
  p.on('response', (r) => { if (r.url().includes(`/api/boards/${board.id}/images/`) && r.request().method() === 'GET') imageResponses.push(r.status()); });
  await p.goto(`${WEB}/materials/boards/${board.id}`);
  await p.getByRole('heading', { name: 'English A1 — Shapes' }).waitFor();

  // ---- 1. Every object type ----
  await tool(p, 'Карандаш').click();
  await dragFrac(p, [0.05, 0.85], [0.25, 0.7]);
  await tool(p, 'Текст').click();
  const cb = await canvas(p).boundingBox();
  await p.mouse.click(cb.x + cb.width * 0.05, cb.y + cb.height * 0.05);
  await p.getByRole('textbox', { name: 'Текст на доске' }).fill('Hello!');
  await p.keyboard.press('Enter');
  await items(p, 'text').first().waitFor();
  await tool(p, 'Стикер').click();
  await p.getByRole('radio', { name: 'Оранжевый' }).click();
  await p.mouse.click(cb.x + cb.width * 0.35, cb.y + cb.height * 0.06);
  await p.getByRole('textbox', { name: 'Текст стикера' }).fill('My name is …');
  await p.keyboard.press('Control+Enter');
  await items(p, 'sticky').filter({ hasText: 'My name is …' }).waitFor();
  await p.getByRole('radio', { name: 'Красный' }).click();
  await tool(p, 'Прямоугольник').click();
  await dragFrac(p, [0.06, 0.25], [0.26, 0.5]);
  await p.getByRole('radio', { name: 'Зелёный' }).click();
  await tool(p, 'Эллипс').click();
  await dragFrac(p, [0.32, 0.4], [0.5, 0.62]);
  await p.getByRole('radio', { name: 'Фиолетовый' }).click();
  await tool(p, 'Стрелка').click();
  await dragFrac(p, [0.55, 0.8], [0.72, 0.6]);
  await p.getByTestId('board-image-input').setInputFiles({ name: 'blue.png', mimeType: 'image/png', buffer: png(120, 60, [47, 128, 237]) });
  await items(p, 'image').waitFor();
  await p.waitForTimeout(500);
  for (const t of ['stroke', 'text', 'sticky', 'rect', 'ellipse', 'arrow', 'image']) { const n = await items(p, t).count(); assert(n === 1, `one ${t}: ${n}`); }
  assert(imageResponses.includes(200), `the picture loads: ${imageResponses}`);
  assert((await items(p, 'sticky').locator('rect').getAttribute('fill')) === '#FFF1A8', 'choosing a colour for the next shape keeps the sticky orange');
  ok('pen, text, sticky note «My name is …», rectangle, ellipse, arrow and an uploaded PNG are on the board');

  // ---- 2. Select: move, resize, undo / redo ----
  await tool(p, 'Выделение').click();
  const rect = items(p, 'rect');
  const r0 = await attrs(rect, ['x', 'y', 'width', 'height']);
  const rb = await rect.boundingBox();
  await dragScreen(p, [rb.x + 3, rb.y + rb.height / 2], [rb.x + 3 + 120, rb.y + rb.height / 2 + 60]);
  const r1 = await attrs(rect, ['x', 'y', 'width', 'height']);
  assert(r1.x > r0.x + 50 && r1.y > r0.y + 25 && r1.width === r0.width, `moved: ${JSON.stringify([r0, r1])}`);
  const hb = await p.getByTestId('board-resize-handle').boundingBox();
  await dragScreen(p, [hb.x + hb.width / 2, hb.y + hb.height / 2], [hb.x + hb.width / 2 + 100, hb.y + hb.height / 2 + 50]);
  const r2 = await attrs(rect, ['x', 'y', 'width', 'height']);
  assert(r2.width > r1.width + 50 && r2.height > r1.height + 25 && r2.x === r1.x, `resized: ${JSON.stringify([r1, r2])}`);
  await p.getByRole('button', { name: 'Отменить' }).click();
  assert(JSON.stringify(await attrs(rect, ['x', 'y', 'width', 'height'])) === JSON.stringify(r1), 'undo resize');
  await p.getByRole('button', { name: 'Отменить' }).click();
  assert(JSON.stringify(await attrs(rect, ['x', 'y', 'width', 'height'])) === JSON.stringify(r0), 'undo move');
  await p.getByRole('button', { name: 'Повторить', exact: true }).click();
  await p.getByRole('button', { name: 'Повторить', exact: true }).click();
  assert(JSON.stringify(await attrs(rect, ['x', 'y', 'width', 'height'])) === JSON.stringify(r2), 'redo move and resize');
  // The picture resizes keeping its proportions.
  const img = items(p, 'image');
  const ib = await img.boundingBox();
  await p.mouse.click(ib.x + ib.width / 2, ib.y + ib.height / 2);
  const i0 = await attrs(img, ['width', 'height']);
  const ih = await p.getByTestId('board-resize-handle').boundingBox();
  await dragScreen(p, [ih.x + ih.width / 2, ih.y + ih.height / 2], [ih.x + ih.width / 2 + 80, ih.y + ih.height / 2 + 5]);
  const i1 = await attrs(img, ['width', 'height']);
  assert(i1.width > i0.width + 40 && Math.abs(i1.width / i1.height - i0.width / i0.height) < 0.02, `picture resize: ${JSON.stringify([i0, i1])}`);
  // The arrow's end handle.
  const arrowLine = items(p, 'arrow').locator('line').nth(1);
  const a0 = await attrs(arrowLine, ['x2', 'y2']);
  const ab = await items(p, 'arrow').boundingBox();
  await p.mouse.click(ab.x + ab.width / 2, ab.y + ab.height / 2);
  const ah = await p.getByTestId('board-resize-handle').boundingBox();
  await dragScreen(p, [ah.x + ah.width / 2, ah.y + ah.height / 2], [ah.x + ah.width / 2 + 60, ah.y + ah.height / 2 - 40]);
  const a1 = await attrs(arrowLine, ['x2', 'y2']);
  assert(a1.x2 > a0.x2 + 20 && a1.y2 < a0.y2 - 10, `arrow end: ${JSON.stringify([a0, a1])}`);
  ok('select moves and resizes (rectangle, picture keeps proportions, arrow end); undo and redo walk move and resize');

  // ---- 3. Edit a sticky by double-click; undo / redo the edit ----
  const sticky = items(p, 'sticky');
  const sb = await sticky.boundingBox();
  await p.mouse.dblclick(sb.x + sb.width / 2, sb.y + sb.height / 2);
  await p.getByRole('textbox', { name: 'Текст стикера' }).fill('My name is Aruzhan.');
  await p.keyboard.press('Control+Enter');
  await sticky.filter({ hasText: 'My name is Aruzhan.' }).waitFor();
  await p.getByRole('button', { name: 'Отменить' }).click();
  await sticky.filter({ hasText: 'My name is …' }).waitFor();
  await p.getByRole('button', { name: 'Повторить', exact: true }).click();
  await sticky.filter({ hasText: 'My name is Aruzhan.' }).waitFor();
  // With «Выделение» a colour recolours the selected object (a sticky gets the matching paper); undo restores it.
  await p.getByRole('radio', { name: 'Синий' }).click();
  assert((await sticky.locator('rect').getAttribute('fill')) === '#D6E8FF', 'recoloured sticky');
  await p.getByRole('button', { name: 'Отменить' }).click();
  assert((await sticky.locator('rect').getAttribute('fill')) === '#FFF1A8', 'undo recolour');
  ok('a sticky note is edited by double-click; undo and redo restore each text; a colour recolours the selected sticky');

  // ---- 4. Delete; undo and redo the deletion ----
  const eb = await items(p, 'ellipse').boundingBox();
  await p.mouse.click(eb.x + eb.width / 2, eb.y + eb.height / 2);
  await p.getByRole('button', { name: 'Удалить' }).click();
  assert(await items(p, 'ellipse').count() === 0, 'deleted');
  await p.getByRole('button', { name: 'Отменить' }).click();
  assert(await items(p, 'ellipse').count() === 1, 'undo delete');
  await p.getByRole('button', { name: 'Повторить', exact: true }).click();
  assert(await items(p, 'ellipse').count() === 0, 'redo delete');
  await p.getByRole('button', { name: 'Отменить' }).click();
  assert(await items(p, 'ellipse').count() === 1, 'the ellipse is back');
  // The Delete key removes the selected object too.
  const tb = await items(p, 'text').boundingBox();
  await p.mouse.click(tb.x + 5, tb.y + tb.height / 2);
  await p.keyboard.press('Delete');
  assert(await items(p, 'text').count() === 0, 'Delete key');
  await p.keyboard.press('Control+z');
  assert(await items(p, 'text').count() === 1, 'Ctrl+Z brings it back');
  ok('«Удалить» and the Delete key remove an object; undo brings it back, redo removes it again');

  // ---- 5. Zoom and the hand; the pen draws exactly under the pointer ----
  assert(await zoomText(p) === '100%', 'starts at 100%');
  await p.getByRole('button', { name: 'Увеличить' }).click();
  await p.getByRole('button', { name: 'Увеличить' }).click();
  assert(await zoomText(p) === '150%', `zoom: ${await zoomText(p)}`);
  const ellipse = items(p, 'ellipse');
  const e0 = await attrs(ellipse, ['cx', 'cy', 'rx', 'ry']);
  const es0 = await ellipse.boundingBox();
  await tool(p, 'Рука').click();
  await dragFrac(p, [0.6, 0.6], [0.45, 0.5]);
  const e1 = await attrs(ellipse, ['cx', 'cy', 'rx', 'ry']);
  const es1 = await ellipse.boundingBox();
  assert(JSON.stringify(e0) === JSON.stringify(e1), 'the hand does not move objects on the board');
  assert(Math.abs((es0.x - es1.x) - 0.15 * cb.width) < 3 && Math.abs((es0.y - es1.y) - 0.1 * cb.height) < 3, `the view moved: ${JSON.stringify([es0, es1])}`);
  await tool(p, 'Карандаш').click();
  await p.getByRole('radio', { name: 'Чёрный' }).click();
  const s0 = [cb.x + cb.width * 0.7, cb.y + cb.height * 0.3];
  await dragScreen(p, s0, [s0[0] + 80, s0[1] + 40]);
  const firstPoint = async () => {
    const d = await items(p, 'stroke').last().locator('path').nth(1).getAttribute('d');
    const [, x, y] = /^M([-\d.]+) ([-\d.]+)/.exec(d);
    return screenOf(p, Number(x), Number(y));
  };
  const at150 = await firstPoint();
  assert(Math.hypot(at150[0] - s0[0], at150[1] - s0[1]) < 1.5, `pen under the pointer at 150% after panning: ${at150} vs ${s0}`);
  for (let i = 0; i < 4; i++) await p.getByRole('button', { name: 'Уменьшить' }).click(); // 150 → 125 → 100 → 90 → 75
  const z = await zoomText(p);
  assert(z === '75%', `zoom out: ${z}`);
  const s1 = [cb.x + cb.width * 0.2, cb.y + cb.height * 0.15];
  await dragScreen(p, s1, [s1[0] + 60, s1[1] + 60]);
  const at75 = await firstPoint();
  assert(Math.hypot(at75[0] - s1[0], at75[1] - s1[1]) < 1.5, `pen under the pointer at 75%: ${at75} vs ${s1}`);
  // A new rectangle at this zoom also starts under the pointer.
  await tool(p, 'Прямоугольник').click();
  const s2 = [cb.x + cb.width * 0.8, cb.y + cb.height * 0.75];
  await dragScreen(p, s2, [s2[0] + 70, s2[1] + 50]);
  const nr = await attrs(items(p, 'rect').last(), ['x', 'y']);
  const nrs = await screenOf(p, nr.x, nr.y);
  assert(Math.hypot(nrs[0] - s2[0], nrs[1] - s2[1]) < 1.5, `rectangle under the pointer: ${nrs} vs ${s2}`);
  await p.getByTestId('board-zoom').click();
  assert(await zoomText(p) === '100%', 'reset to 100%');
  await settle(p);
  await p.screenshot({ path: `${OUT}/01-board-all-objects.png` });
  await p.getByTestId('board-toolbar').screenshot({ path: `${OUT}/00-toolbar.png` });
  ok('«+» → 150%, the hand moves the view (objects keep their board coordinates); the pen and a rectangle start under the pointer at 150% after panning and at 75%; «100%» resets');

  // ---- 6. Save and reopen ----
  assert((await status(p)).includes('Есть несохранённые изменения'), 'dirty');
  await p.getByRole('button', { name: 'Сохранить' }).click();
  await p.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  const counts = async () => Object.fromEntries(await Promise.all(['stroke', 'text', 'sticky', 'rect', 'ellipse', 'arrow', 'image'].map(async (t) => [t, await items(p, t).count()])));
  const before = await counts();
  const rectBefore = await attrs(items(p, 'rect').first(), ['x', 'y', 'width', 'height']);
  imageResponses.length = 0;
  await p.reload();
  await items(p, 'sticky').filter({ hasText: 'My name is Aruzhan.' }).waitFor();
  await p.waitForTimeout(500);
  const after = await counts();
  assert(JSON.stringify(before) === JSON.stringify(after), `after reload: ${JSON.stringify([before, after])}`);
  assert(JSON.stringify(await attrs(items(p, 'rect').first(), ['x', 'y', 'width', 'height'])) === JSON.stringify(rectBefore), 'rectangle geometry kept');
  assert(imageResponses.includes(200), 'the picture loads after reload');
  assert((await status(p)).includes('Все изменения сохранены'), 'not dirty after reload');
  await p.screenshot({ path: `${OUT}/02-board-reopened.png` });
  const saved = await api.must('GET', `/boards/${board.id}`, undefined, 200);
  const ids = saved.content.items.map((x) => x.id);
  assert(ids.every(Boolean) && new Set(ids).size === ids.length, 'every saved object has its own id');
  ok(`saved and reopened: ${JSON.stringify(after)}, the rectangle keeps its size, the picture loads`);

  // ---- 7. Another teacher cannot read the picture ----
  const imageItem = saved.content.items.find((x) => x.type === 'image');
  const apiB = await apiAs(stack, B.email, B.password);
  const rb2 = await apiB.call('GET', `/boards/${board.id}/images/${imageItem.image_id}`);
  assert(rb2.status === 404, `another teacher reads the picture: ${rb2.status}`);
  ok('another teacher gets 404 for the picture');

  // ---- 8. An old board (no ids) opens unchanged and saves again ----
  {
    // Exactly what the first version of the editor saved: strokes and a text, no ids, inside 1600 × 1000.
    const apiOld = api;
    const created = await apiOld.must('POST', '/boards', { title: 'Old board' }, 201);
    await apiOld.must('PUT', `/boards/${created.id}`, { version: 1, content: { items: [
      { type: 'stroke', color: '#E5484D', width: 4, points: [[480, 800], [480, 550], [800, 550], [800, 800], [480, 800]] },
      { type: 'stroke', color: '#E5484D', width: 4, points: [[450, 560], [640, 400], [830, 560]] },
      { type: 'stroke', color: '#16A34A', width: 4, points: [[960, 800], [1440, 800]] },
      { type: 'text', color: '#2F80ED', x: 128, y: 100, size: 40, text: 'Hello! My name is …' },
    ] } }, 200);
    const old = await apiOld.must('GET', `/boards/${created.id}`, undefined, 200);
    assert(old.content.items.every((x) => !x.id), 'the old board has no ids');
    const q = await login(A.email, A.password);
    await q.goto(`${WEB}/materials/boards/${old.id}`);
    await q.getByRole('heading', { name: old.title }).waitFor();
    const strokes = old.content.items.filter((x) => x.type === 'stroke').length;
    const texts = old.content.items.filter((x) => x.type === 'text').length;
    await items(q, 'stroke').nth(strokes - 1).waitFor({ state: 'attached' });
    assert(await items(q, 'stroke').count() === strokes && await items(q, 'text').count() === texts, 'old objects shown');
    assert((await status(q)).includes('Все изменения сохранены'), 'an old board is not marked changed on open');
    await q.screenshot({ path: `${OUT}/03-old-board.png` });
    // Select and move an old stroke, then save: it works and the board gets ids.
    await tool(q, 'Выделение').click();
    const sb2 = await items(q, 'stroke').first().locator('path').nth(1).boundingBox();
    const d0 = await items(q, 'stroke').first().locator('path').nth(1).getAttribute('d');
    const startPt = /^M([-\d.]+) ([-\d.]+)/.exec(d0);
    const sp = await screenOf(q, Number(startPt[1]), Number(startPt[2]));
    await dragScreen(q, sp, [sp[0] + 40, sp[1] + 20]);
    assert((await items(q, 'stroke').first().locator('path').nth(1).getAttribute('d')) !== d0, `old stroke moved ${JSON.stringify(sb2)}`);
    await q.getByRole('button', { name: 'Отменить' }).click();
    assert((await items(q, 'stroke').first().locator('path').nth(1).getAttribute('d')) === d0, 'undo the move');
    assert((await status(q)).includes('Все изменения сохранены'), 'undo back to the saved state is not a change');
    await q.getByRole('button', { name: 'Повторить', exact: true }).click();
    await q.getByRole('button', { name: 'Сохранить' }).click();
    await q.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
    const re = await apiOld.must('GET', `/boards/${old.id}`, undefined, 200);
    assert(re.content.items.length === old.content.items.length && re.content.items.every((x) => x.id), 'saved with ids');
    ok(`the old board «${old.title}» opens unchanged (${strokes} strokes, ${texts} text), its stroke moves and undoes, it saves again`);
  }

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`teacher: ${A.email} / ${A.password}; board: ${WEB}/materials/boards/${board.id}`);
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
