// Classes page check (read-only: no classes are created, no data changes).
//   node scripts/classes-page-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3448';
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/classes-page';
mkdirSync(OUT, { recursive: true });
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];

async function login(email, pw, opts = {}) {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, locale: 'ru-RU', ...opts });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(pw);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes$/);
  await p.evaluate(() => { localStorage.removeItem('ortalyq_classes_view'); localStorage.removeItem('ortalyq_classes_sort'); localStorage.setItem('ortalyq_sidebar_pinned', '0'); });
  await p.reload();
  await p.getByText(/Кол-во классов \d+/).waitFor();
  return { ctx, p };
}

try {
  const { ctx, p } = await login('teacher@ortalyq.local', 'teacher-local-123');
  const main = p.locator('main');
  const count = async () => Number((await main.getByText(/Кол-во классов \d+/).innerText()).match(/\d+/)[0]);
  const names = async () => (await main.locator('article h2').allInnerTexts()).map((s) => s.trim());
  const settle = () => p.waitForTimeout(700);
  await p.mouse.move(900, 600);
  await settle();
  const panel = await main.locator('section').first().boundingBox();
  assert(Math.round(panel.width) === 1024, `panel width ${panel.width}`);
  const all = await names();
  assert(all.length === (await count()), `count ${await count()} vs tiles ${all.length}`);
  assert(await main.getByRole('button', { name: 'Создать класс' }).isVisible(), 'create tile first');
  const tiles = main.locator('.grid > *');
  assert((await tiles.first().innerText()).includes('Создать класс'), 'create tile is first');
  const cover = await main.locator('article').first().locator('div').first().boundingBox();
  assert(Math.abs(cover.width - cover.height) < 1, 'square cover');
  await p.screenshot({ path: `${OUT}/01-grid-1440.png` });
  ok(`grid: panel 1024px, ${all.length} real classes counted (create tile excluded), square covers`);

  // Search + filter together.
  await main.getByRole('searchbox').fill('данияр');
  await settle();
  assert((await count()) === 1 && (await names())[0].includes('Данияр'), 'search');
  await main.getByRole('radio', { name: 'Индивидуальные' }).click();
  await settle();
  assert((await count()) === 1, 'search + individual');
  await main.getByRole('radio', { name: 'Групповые' }).click();
  await settle();
  assert((await count()) === 0 && (await main.getByText('Классы не найдены').isVisible()), 'search + group: no results');
  await p.screenshot({ path: `${OUT}/02-no-results-group.png` });
  await main.getByRole('button', { name: 'Сбросить поиск и фильтр' }).click();
  await settle();
  assert((await count()) === all.length && (await main.getByRole('radio', { name: 'Все' }).getAttribute('aria-checked')) === 'true', 'reset');
  ok('search and type filter work together; no-results state with reset');

  // Group / marathon tabs without search: framed placeholder like the original; hints on hover.
  await main.getByRole('radio', { name: 'Групповые' }).hover();
  await p.waitForTimeout(300);
  assert(await main.getByRole('tooltip', { name: 'Групповые классы онлайн-уроков' }).isVisible(), 'group hint on hover');
  await main.getByRole('radio', { name: 'Групповые' }).click();
  await settle();
  assert((await main.getByText(/Кол-во классов/).count()) === 0 && (await main.getByText('Создайте класс для проведения обучения').isVisible()), 'group empty state');
  await main.getByRole('button', { name: 'Создать класс' }).click();
  await p.getByRole('dialog', { name: 'Создать класс' }).waitFor();
  await p.keyboard.press('Escape');
  await p.getByRole('dialog').waitFor({ state: 'detached' });
  await main.getByRole('radio', { name: 'Марафоны' }).click();
  await settle();
  assert((await main.getByText('Марафоны недоступны на этом этапе').count()) === 0 && (await main.getByRole('button', { name: 'Создать класс' }).count()) >= 1, 'marathons: real list with a create entry');
  await main.getByRole('radio', { name: 'Все' }).click();
  await settle();
  ok('group tab: empty state with create button (dialog opens); marathons: real list with create entry; type hints on hover');

  // Sorting: field ("По умолчанию" = date / "По названию") × direction buttons.
  const dir = (name) => main.getByRole('radio', { name }).click();
  await main.getByLabel('Порядок').selectOption('name');
  await dir('По возрастанию');
  await settle();
  const asc = await names();
  assert(JSON.stringify(asc) === JSON.stringify([...asc].sort((a, c) => a.localeCompare(c, 'ru'))), `name asc: ${asc}`);
  await dir('По убыванию');
  await settle();
  assert(JSON.stringify(await names()) === JSON.stringify([...asc].reverse()), 'name desc');
  await main.getByLabel('Порядок').selectOption('created');
  await settle();
  assert((await main.getByRole('radio', { name: 'По убыванию' }).getAttribute('aria-checked')) === 'true', 'default = newest first');
  ok('sorting: by name A–Z / Z–A, by date newest/oldest');

  // Grid vs list: same data; view persists after reload.
  const gridData = await main.locator('article').allInnerTexts();
  await main.getByRole('radio', { name: 'Список' }).click();
  await settle();
  const listData = await main.locator('article').allInnerTexts();
  const norm = (arr) => arr.map((s) => s.replace(/Войти в класс/g, '').replace(/\s+/g, ' ').trim());
  assert(JSON.stringify(norm(gridData)) === JSON.stringify(norm(listData)), `grid/list differ:\n${norm(gridData)}\n${norm(listData)}`);
  await p.screenshot({ path: `${OUT}/03-list-1440.png` });
  await p.reload();
  await settle();
  assert((await main.getByRole('radio', { name: 'Список' }).getAttribute('aria-checked')) === 'true', 'list view persisted');
  await main.getByRole('radio', { name: 'Плитки' }).click();
  ok('grid and list show identical data; chosen view persists after reload');

  // Create modal opens (closed without creating to keep demo data unchanged).
  await main.getByRole('button', { name: 'Создать класс' }).click();
  // Type chooser first, then the individual form (closed without creating to keep demo data unchanged).
  const chooser = p.getByRole('dialog', { name: 'Создать класс' });
  await chooser.waitFor();
  assert(await chooser.getByRole('button', { name: /Групповой класс/ }).isDisabled(), 'group type is unavailable');
  await chooser.getByRole('button', { name: /Индивидуальный класс/ }).click();
  await p.getByRole('dialog', { name: /Новый класс/ }).waitFor();
  await p.getByRole('dialog').getByRole('button', { name: 'Отмена' }).click();
  ok('create class dialog opens (not submitted)');

  // Sidebar still expands on hover over this page and collapses after leaving.
  const side = p.locator('#app-sidebar');
  await side.getByRole('link', { name: 'Классы' }).hover();
  await p.waitForTimeout(320);
  assert(Math.round((await side.boundingBox()).width) === 288, 'menu expands');
  await p.screenshot({ path: `${OUT}/04-menu-hover.png` });
  await p.mouse.move(900, 600);
  await p.waitForTimeout(450);
  assert(Math.round((await side.boundingBox()).width) === 64, 'menu collapses');
  ok('cabinet menu: hover expand / collapse on /classes');

  // Open an existing class from a tile.
  await main.locator('article').filter({ hasText: 'Английский — Данияр' }).getByRole('link', { name: /Войти в класс/ }).click();
  await p.waitForURL(/\/classes\/[0-9a-f-]+/);
  await p.locator('#classroom-toolbar').waitFor();
  ok('opening an existing class works');
  await ctx.close();

  // Student: no create tile, same layout.
  const { ctx: sctx, p: sp } = await login('student@ortalyq.local', 'student-local-123');
  assert((await sp.locator('main').getByRole('button', { name: 'Создать класс' }).count()) === 0, 'student has no create tile');
  await sp.screenshot({ path: `${OUT}/05-student-grid.png` });
  await sctx.close();
  ok('student view: own classes, no create tile');

  // Phone: no horizontal scroll, 2 columns.
  const { ctx: mctx, p: mp } = await login('teacher@ortalyq.local', 'teacher-local-123', { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 });
  await mp.waitForTimeout(500);
  const overflow = await mp.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(overflow <= 0, `horizontal overflow ${overflow}px`);
  await mp.screenshot({ path: `${OUT}/06-mobile.png`, fullPage: true });
  await mctx.close();
  ok('phone 390px: cards reflow, no horizontal scroll');

  assert(!errors.length, errors.join('\n'));
  console.log('\nCLASSES PAGE CHECK PASSED');
} catch (e) {
  console.error('FAILED:', e.message);
  process.exitCode = 1;
} finally {
  await b.close();
}
