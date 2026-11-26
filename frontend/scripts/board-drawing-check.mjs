// Drawing together on the lesson board, in two browser sessions. The student starts in «Только просмотр». The
// teacher picks the student and presses «Разрешить рисовать» → the student gets the pen and colours without a reload.
// Both draw at the same moment → both strokes reach both screens and survive reloads of both pages. «Отозвать
// разрешение» → the student is back to view only at once; a stale tab that missed the event (its socket is cut) still
// shows the pen, and the server refuses its stroke. Closing and opening the board again resets the permission.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/board-drawing-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/boards/drawing-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
let expected403 = false; // the refused stroke of the stale tab logs a 403 in the console
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(300));

function watch(p) {
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 4(01|04|09|22)/.test(m.text()) && !(expected403 && /status of 403/.test(m.text()))) errors.push(m.text()); });
  p.on('request', (r) => { const u = new URL(r.url()); if (/^https?:$/.test(u.protocol) && !['localhost', '127.0.0.1'].includes(u.hostname)) external.add(r.url()); });
}
async function login(email, password) {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 1100 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  watch(p);
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль', { exact: true }).fill(password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL((u) => !u.pathname.startsWith('/login'));
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return { ctx, p };
}
/** A drag through points given in fractions of the element's box. */
async function drag(p, el, pts) {
  const box = await el.boundingBox();
  const at = ([fx, fy]) => [box.x + fx * box.width, box.y + fy * box.height];
  await p.mouse.move(...at(pts[0]));
  await p.mouse.down();
  for (const pt of pts.slice(1)) await p.mouse.move(...at(pt), { steps: 12 });
  await p.mouse.up();
}
const tStrokes = (p) => p.locator('[data-testid="board-canvas"] [data-item="stroke"]');
const sStrokes = (p) => p.locator('[data-testid="board-viewer"] [data-item="stroke"]');
const strokeColors = (loc) => loc.evaluateAll((els) => els.map((g) => g.querySelectorAll('path')[1].getAttribute('stroke')).sort());
const RED = '#E5484D';
const INK = '#1F2937';

try {
  const users = await makeUsers(stack, RUN, { students: ['Данияр'] });
  const [S] = users.students;
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const cls = await api.must('POST', '/classrooms', { name: `Английский — Данияр ${RUN}`, student_id: S.id }, 201);
  const board = await api.must('POST', '/boards', { title: 'English A1 — Together' }, 201);
  await api.must('PUT', `/classrooms/${cls.id}/board`, { board_id: board.id }, 200);
  const classUrl = `${WEB}/classes/${cls.id}`;

  // ---- 1. View only by default ----
  const T = await login(users.teacher.email, users.teacher.password);
  const tp = T.p;
  await tp.goto(classUrl);
  await tp.getByRole('region', { name: 'Рисование учеников' }).waitFor();
  const St = await login(S.email, S.password);
  const sp = St.p;
  await sp.goto(classUrl);
  await sp.getByTestId('class-board-student').waitFor();
  assert((await sp.getByTestId('class-board-student').innerText()).includes('Только просмотр'), 'view only by default');
  assert(await sp.getByTestId('student-board-toolbar').count() === 0, 'no pen by default');
  ok('the student starts in «Только просмотр» (no pen)');

  // ---- 2. «Разрешить рисовать» ----
  const panel = tp.getByRole('region', { name: 'Рисование учеников' });
  await panel.getByLabel('Ученик').selectOption({ label: S.name });
  await panel.getByRole('button', { name: 'Разрешить рисовать' }).click();
  await panel.getByTestId('board-drawer').filter({ hasText: S.name }).waitFor();
  await sp.getByTestId('student-board-toolbar').waitFor();
  assert((await sp.getByTestId('class-board-student').innerText()).includes('Можно рисовать'), 'student badge');
  const stToolbar = sp.getByTestId('student-board-toolbar');
  assert(await stToolbar.getByRole('radio').count() === 6 && await stToolbar.getByRole('button', { name: 'Карандаш' }).count() === 1
    && await stToolbar.getByRole('button').count() === 1, 'the student has only the pen and the colours');
  ok('«Разрешить рисовать» for the chosen student: the pen and the colours appear on the student\'s screen without a reload');

  // ---- 3. Both draw at the same moment ----
  await stToolbar.getByRole('radio', { name: 'Красный' }).click();
  await Promise.all([
    drag(tp, tp.getByTestId('board-canvas'), [[0.1, 0.3], [0.3, 0.2], [0.45, 0.35]]),
    drag(sp, sp.getByTestId('board-viewer'), [[0.55, 0.7], [0.7, 0.55], [0.85, 0.7]]),
  ]);
  await tp.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  const both = async () => {
    for (let i = 0; i < 40; i++) {
      const [tc, sc] = [await strokeColors(tStrokes(tp)), await strokeColors(sStrokes(sp))];
      if (tc.length === 2 && sc.length === 2) return [tc, sc];
      await tp.waitForTimeout(250);
    }
    return [await strokeColors(tStrokes(tp)), await strokeColors(sStrokes(sp))];
  };
  const [tc, sc] = await both();
  assert(JSON.stringify(tc) === JSON.stringify([INK, RED].sort()) && JSON.stringify(sc) === JSON.stringify([INK, RED].sort()), `both strokes on both screens: ${tc} / ${sc}`);
  await tp.getByTestId('board-status').filter({ hasText: 'Все изменения сохранены' }).waitFor();
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/01-teacher-both-strokes.png` });
  await settle(sp);
  await sp.screenshot({ path: `${OUT}/02-student-drawing.png` });
  ok('drawing at the same moment: the teacher\'s black stroke and the student\'s red stroke are on both screens');

  // ---- 4. Reload both pages: both strokes are saved ----
  await Promise.all([tp.reload(), sp.reload()]);
  await tp.getByTestId('board-canvas').waitFor();
  await sp.getByTestId('board-viewer').waitFor();
  await tp.waitForTimeout(500);
  const [tc2, sc2] = [await strokeColors(tStrokes(tp)), await strokeColors(sStrokes(sp))];
  assert(tc2.length === 2 && sc2.length === 2 && tc2.includes(RED) && sc2.includes(INK), `after reload: ${tc2} / ${sc2}`);
  const stored = (await api.must('GET', `/boards/${board.id}`, undefined, 200)).content.items;
  assert(stored.length === 2 && stored.some((x) => x.id.startsWith('s-') && x.color === RED) && stored.some((x) => !x.id.startsWith('s-') && x.color === INK), `stored: ${JSON.stringify(stored.map((x) => [x.id, x.color]))}`);
  ok('after reloading both pages both strokes are there; the server stored the student\'s stroke as its own item');

  // ---- 4b. A teacher tab that misses events: its save is refused (409) and merged with the student's stroke ----
  const t2 = await T.ctx.newPage();
  watch(t2);
  await t2.routeWebSocket(/\/ws$/, () => { /* no events for this tab */ });
  const puts = [];
  t2.on('response', (r) => { if (r.url().endsWith(`/api/boards/${board.id}`) && r.request().method() === 'PUT') puts.push(r.status()); });
  await t2.goto(classUrl);
  await t2.getByTestId('board-canvas').waitFor();
  await t2.waitForTimeout(500);
  await stToolbar.getByRole('radio', { name: 'Зелёный' }).click();
  await drag(sp, sp.getByTestId('board-viewer'), [[0.2, 0.85], [0.4, 0.9]]); // the student draws first
  for (let i = 0; i < 40 && (await api.must('GET', `/boards/${board.id}`, undefined, 200)).content.items.length < 3; i++) await sp.waitForTimeout(250);
  expected403 = true; // a 409 is logged like a 4xx
  await t2.getByRole('radio', { name: 'Синий' }).click();
  await drag(t2, t2.getByTestId('board-canvas'), [[0.6, 0.15], [0.85, 0.25]]); // the stale teacher tab draws
  for (let i = 0; i < 60 && !(puts.includes(409) && puts[puts.length - 1] === 200); i++) await t2.waitForTimeout(250);
  expected403 = false;
  const merged = (await api.must('GET', `/boards/${board.id}`, undefined, 200)).content.items;
  assert(puts.includes(409) && puts[puts.length - 1] === 200, `the stale save was refused and saved again: ${puts}`);
  assert(merged.length === 4 && merged.some((x) => x.color === '#16A34A') && merged.some((x) => x.color === '#2F80ED'), `merged: ${JSON.stringify(merged.map((x) => x.color))}`);
  assert(await t2.getByText('Доску сохранили в другой вкладке').count() === 0, 'no conflict message: merged by itself');
  assert(await tStrokes(t2).count() === 4, 'the stale teacher tab shows the student\'s stroke after merging');
  await t2.close();
  ok(`a teacher tab without events: its save got 409, merged the student's stroke and saved again (${puts.join(' → ')}); the board has all 4 strokes`);

  // ---- 5. A stale tab of the student (it misses every event) ----
  const stale = await St.ctx.newPage();
  watch(stale);
  await stale.routeWebSocket(/\/ws$/, () => { /* never connects to the server: this tab gets no events */ });
  await stale.goto(classUrl);
  await stale.getByTestId('student-board-toolbar').waitFor();

  // ---- 6. «Отозвать разрешение» ----
  await panel.getByRole('button', { name: 'Отозвать разрешение' }).click();
  await panel.getByTestId('board-drawer').waitFor({ state: 'detached' });
  await sp.getByTestId('student-board-toolbar').waitFor({ state: 'detached', timeout: 10000 });
  assert((await sp.getByTestId('class-board-student').innerText()).includes('Только просмотр'), 'back to view only');
  await sp.screenshot({ path: `${OUT}/03-student-after-revoke.png` });
  ok('«Отозвать разрешение»: the student\'s pen disappears at once (view only)');

  assert(await stale.getByTestId('student-board-toolbar').count() === 1, 'the stale tab still shows the pen');
  const staleBefore = await sStrokes(stale).count();
  expected403 = true;
  const refused = stale.waitForResponse((r) => r.url().endsWith(`/classrooms/${cls.id}/board/strokes`) && r.request().method() === 'POST');
  await drag(stale, stale.getByTestId('board-viewer'), [[0.2, 0.8], [0.4, 0.85]]);
  const resp = await refused;
  assert(resp.status() === 403, `stale tab stroke: ${resp.status()}`);
  await stale.getByText('Преподаватель отозвал разрешение рисовать').waitFor();
  await stale.waitForTimeout(500);
  assert(await sStrokes(stale).count() === staleBefore, 'the refused stroke is not kept on the stale tab');
  await stale.screenshot({ path: `${OUT}/04-stale-tab-refused.png` });
  expected403 = false;
  assert((await api.must('GET', `/boards/${board.id}`, undefined, 200)).content.items.length === 4, 'the board did not change');
  ok('a stale tab that missed the event still shows the pen, but the server refuses its stroke (403) and the board stays as it was');

  // ---- 7. Closing and opening again resets the permission ----
  await panel.getByRole('button', { name: 'Разрешить рисовать' }).click();
  await sp.getByTestId('student-board-toolbar').waitFor();
  await tp.getByRole('button', { name: 'Закрыть доску в классе' }).click();
  await sp.getByTestId('class-board-student').waitFor({ state: 'detached', timeout: 10000 });
  await api.must('PUT', `/classrooms/${cls.id}/board`, { board_id: board.id }, 200);
  await sp.getByTestId('class-board-student').waitFor({ timeout: 10000 });
  assert((await sp.getByTestId('class-board-student').innerText()).includes('Только просмотр') && await sp.getByTestId('student-board-toolbar').count() === 0, 'view only after reopening');
  await tp.reload();
  await tp.getByRole('region', { name: 'Рисование учеников' }).waitFor();
  assert(await tp.getByTestId('board-drawer').count() === 0, 'no drawers after reopening');
  expected403 = true;
  const direct = await sp.evaluate(async ([api, cid, bid]) => {
    const r = await fetch(`${api}/api/classrooms/${cid}/board/strokes`, { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ board_id: bid, client_id: crypto.randomUUID(), color: '#E5484D', width: 4, points: [[1, 1], [9, 9]] }) });
    return r.status;
  }, [stack.API, cls.id, board.id]);
  expected403 = false;
  assert(direct === 403, `stroke after reopening: ${direct}`);
  await settle(sp);
  await sp.screenshot({ path: `${OUT}/05-student-after-reopen.png` });
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/06-teacher-after-reopen.png` });
  ok('closing and opening the board again: the student is back to view only, the panel lists nobody, the server refuses a stroke (403)');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`teacher: ${users.teacher.email} / ${users.teacher.password}; student: ${S.email} / ${S.password}`);
  console.log(`class: ${classUrl}`);
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
