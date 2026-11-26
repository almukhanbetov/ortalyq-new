// Browser check of the cabinet menu: hover peek, delayed close, pin, keyboard, mobile drawer.
//   node scripts/sidebar-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3448';
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/menu';
mkdirSync(OUT, { recursive: true });
// HEADED=1 runs a regular windowed Chrome instead of headless.
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: !process.env.HEADED, args: (process.env.CHROME_ARGS || '').split(' ').filter(Boolean) });
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
try {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill('teacher@ortalyq.local');
  await p.getByLabel('Пароль', { exact: true }).fill(process.env.TEACHER_PASSWORD || 'teacher-local-123');
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes/);
  await p.evaluate(() => localStorage.removeItem('ortalyq_sidebar_pinned'));
  await p.reload();
  await p.mouse.move(900, 500);
  const side = p.locator('#app-sidebar');
  const width = async () => Math.round((await side.boundingBox()).width);
  const mainLeft = () => p.locator('main').evaluate((m) => getComputedStyle(m).paddingLeft);
  const labelOpacity = () => side.getByText('Материалы', { exact: true }).evaluate((el) => getComputedStyle(el).opacity);
  await p.waitForTimeout(300);

  // 1. Initial: narrow, icons only.
  assert((await width()) === 64, `initial width ${await width()}`);
  assert((await labelOpacity()) === '0', 'labels hidden initially');
  const baseLeft = await mainLeft();
  await p.screenshot({ path: `${OUT}/01-narrow.png` });
  ok('initial: narrow 64px, icons only');

  // 2-3. Hover opens as overlay; moving between items keeps it open.
  await side.getByRole('link', { name: 'Классы' }).hover();
  await p.waitForTimeout(300);
  assert((await width()) === 288, 'expanded on hover');
  assert((await labelOpacity()) === '1', 'labels visible on hover');
  assert((await mainLeft()) === baseLeft, 'page did not shift on hover');
  for (const name of ['Ученики', 'Материалы', 'Расписание', 'Тарифы', 'Настройки', 'Добро пожаловать']) {
    await side.getByRole('link', { name }).hover();
    await p.waitForTimeout(60);
    assert((await width()) === 288, `stays open while moving to ${name}`);
  }
  const box = await side.boundingBox();
  await p.mouse.move(box.x + 120, box.y + box.height - 40); // empty area of the panel
  await p.waitForTimeout(250);
  assert((await width()) === 288, 'stays open over empty panel area');
  await side.getByRole('link', { name: 'Материалы' }).hover();
  await p.waitForTimeout(250);
  await p.screenshot({ path: `${OUT}/02-hover-overlay.png` });
  ok('hover: expands to 288px over content (page offset unchanged), stays open between items');

  // 4. Leaving: delayed close.
  await p.mouse.move(800, 450);
  await p.waitForTimeout(90);
  assert((await width()) === 288, 'still open ~90ms after leaving (close delay)');
  await p.waitForTimeout(450);
  assert((await width()) === 64, 'collapsed after leaving');
  ok('leave: stays ~180ms, then collapses');

  // Brief exit and re-entry within the delay does not close.
  await side.getByRole('link', { name: 'Классы' }).hover();
  await p.waitForTimeout(250);
  await p.mouse.move(300, 300);
  await p.waitForTimeout(80);
  await side.getByRole('link', { name: 'Ученики' }).hover();
  await p.waitForTimeout(300);
  assert((await width()) === 288, 're-entry within delay keeps panel open');
  ok('quick exit/re-entry does not cause accidental close');

  // Click on a menu item with the mouse, then leave: must collapse (no sticky focus).
  await side.getByRole('link', { name: 'Материалы' }).click();
  await p.getByRole('heading', { name: 'Материалы', exact: true }).waitFor();
  await p.mouse.move(900, 500);
  await p.waitForTimeout(500);
  assert((await width()) === 64, 'collapses after mouse click + leave');
  ok('mouse click then leave: collapses (focus from click does not pin it)');

  // 6. Pin: stays open after leaving, content shifts; persists; unpin restores hover mode.
  await p.getByRole('button', { name: 'Закрепить меню' }).click();
  await p.mouse.move(900, 500);
  await p.waitForTimeout(500);
  assert((await width()) === 288, 'pinned stays open');
  assert((await mainLeft()) !== baseLeft, 'content shifted when pinned');
  await p.screenshot({ path: `${OUT}/03-pinned.png` });
  await p.reload();
  await p.mouse.move(900, 500);
  await p.waitForTimeout(400);
  assert((await width()) === 288, 'pin persisted after reload');
  await p.getByRole('button', { name: 'Открепить меню' }).click();
  await p.mouse.move(900, 500);
  await p.waitForTimeout(500);
  assert((await width()) === 64, 'unpinned collapses');
  await side.getByRole('link', { name: 'Классы' }).hover();
  await p.waitForTimeout(300);
  assert((await width()) === 288, 'hover works again after unpin');
  await p.mouse.move(900, 500);
  await p.waitForTimeout(500);
  ok('pin: stays open, shifts content, persists; unpin returns hover mode');

  // 8. Keyboard: focus inside the narrow panel expands it until focus leaves.
  await p.evaluate(() => (document.activeElement)?.blur());
  for (let i = 0; i < 8; i++) {
    await p.keyboard.press('Tab');
    if (await p.evaluate(() => document.getElementById('app-sidebar').contains(document.activeElement))) break;
    assert((await width()) === 64, 'header focus does not expand panel');
  }
  await p.waitForTimeout(300);
  const inSide = await p.evaluate(() => document.getElementById('app-sidebar').contains(document.activeElement));
  assert(inSide, 'Tab moved focus into sidebar');
  assert((await width()) === 288, 'keyboard focus expands');
  for (let i = 0; i < 4; i++) {
    await p.keyboard.press('Tab');
    await p.waitForTimeout(80);
    assert((await width()) === 288, 'stays open while tabbing inside');
  }
  await p.screenshot({ path: `${OUT}/04-keyboard-focus.png` });
  for (let i = 0; i < 10; i++) {
    if (!(await p.evaluate(() => document.getElementById('app-sidebar').contains(document.activeElement)))) break;
    await p.keyboard.press('Tab');
  }
  await p.waitForTimeout(400);
  assert((await width()) === 64, 'collapses when focus leaves');
  ok('keyboard: expands on focus inside, stays while tabbing, collapses when focus leaves');

  // Every cabinet page (unpinned): direct URL, reload, menu navigation, deep pages.
  const pagesReport = [];
  const cycle = async (label) => {
    await p.mouse.move(900, 500);
    await p.waitForTimeout(450);
    const rest = await width();
    await side.getByRole('link').nth(2).hover();
    await p.waitForTimeout(320);
    const hover = await width();
    const shifted = (await mainLeft()) !== baseLeft;
    await p.mouse.move(900, 500);
    await p.waitForTimeout(90);
    const during = await width();
    await p.waitForTimeout(400);
    const after = await width();
    const pass = rest === 64 && hover === 288 && !shifted && during === 288 && after === 64;
    pagesReport.push(`${pass ? 'OK ' : 'BAD'} ${label.padEnd(28)} ${p.url().replace(WEB, '')}`);
    assert(pass, `${label}: rest=${rest} hover=${hover} shifted=${shifted} during=${during} after=${after}`);
  };
  const menuPages = [['Добро пожаловать', '/welcome'], ['Классы', '/cabinet/school/classes'], ['Ученики', '/students'], ['Материалы', '/materials'],
    ['Расписание', '/schedule'], ['Сайт и заявки', '/site'], ['Тарифы', '/pricing'], ['Настройки', '/settings']];
  for (const [name, url] of menuPages) {
    await p.goto(WEB + url);
    await p.locator('main h1').first().waitFor();
    await cycle(`direct ${name}`);
    await p.reload();
    await p.locator('main h1').first().waitFor();
    await cycle(`reload ${name}`);
  }
  for (const [name] of [...menuPages].reverse()) {
    await side.getByRole('link', { name }).hover();
    await side.getByRole('link', { name }).click();
    await p.waitForTimeout(500);
    await cycle(`menu → ${name}`);
  }
  await side.getByRole('link', { name: 'Материалы' }).click();
  await p.getByText('English Starter A1').click();
  await p.waitForURL(/courses/);
  await cycle('in-page → course');
  await p.getByText('Lesson 1. Hello!').click();
  await p.waitForURL(/lessons/);
  await cycle('in-page → lesson editor');
  await p.reload();
  await p.waitForTimeout(800);
  await cycle('reload lesson editor');
  await p.goBack();
  await p.waitForTimeout(800);
  await cycle('history back → course');
  // Mode is kept across page changes: pin on one page, check on others, then unpin.
  await p.getByRole('button', { name: 'Закрепить меню' }).click();
  for (const url of ['/welcome', '/pricing']) {
    await side.getByRole('link', { name: url === '/welcome' ? 'Добро пожаловать' : 'Тарифы' }).click();
    await p.mouse.move(900, 500);
    await p.waitForTimeout(500);
    assert((await width()) === 288, `pinned mode kept on ${url}`);
  }
  await p.getByRole('button', { name: 'Открепить меню' }).click();
  await p.mouse.move(900, 500);
  await p.waitForTimeout(500);
  await cycle('after unpin on /pricing');
  console.log(pagesReport.join('\n'));
  ok(`all cabinet pages behave the same (${pagesReport.length} checks)`);

  // Admin cabinet uses the same menu component.
  const a = await b.newContext({ viewport: { width: 1440, height: 900 }, locale: 'ru-RU' });
  const ap = await a.newPage();
  await ap.goto(`${WEB}/login`);
  await ap.getByLabel('Email').fill('admin@ortalyq.local');
  await ap.getByLabel('Пароль', { exact: true }).fill(process.env.ADMIN_PASSWORD || 'admin-local-123');
  await ap.getByRole('button', { name: 'Войти' }).click();
  await ap.waitForURL(/\/admin/);
  await ap.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  await ap.reload();
  const aw = async () => Math.round((await ap.locator('#app-sidebar').boundingBox()).width);
  await ap.mouse.move(900, 500); await ap.waitForTimeout(400);
  const ar = await aw();
  await ap.locator('#app-sidebar').getByRole('link', { name: 'Администрирование' }).hover(); await ap.waitForTimeout(320);
  const ah = await aw();
  await ap.mouse.move(900, 500); await ap.waitForTimeout(500);
  assert(ar === 64 && ah === 288 && (await aw()) === 64, `admin page hover: ${ar}/${ah}`);
  await a.close();
  ok('admin cabinet (/admin): same hover/collapse behaviour');

  // 7. Mobile: opened by button, overlay with backdrop.
  const m = await b.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, storageState: await ctx.storageState(), locale: 'ru-RU' });
  const mp = await m.newPage();
  mp.on('pageerror', (e) => errors.push(e.message));
  await mp.goto(`${WEB}/classes`);
  const mside = mp.locator('#app-sidebar');
  await mp.waitForTimeout(300);
  assert((await mside.boundingBox()).x < -200, 'drawer hidden initially');
  await mp.getByRole('button', { name: 'Открыть меню' }).click();
  await mp.waitForTimeout(300);
  assert((await mside.boundingBox()).x === 0, 'drawer opened');
  await mp.screenshot({ path: `${OUT}/05-mobile-open.png` });
  await mp.mouse.click(370, 400);
  await mp.waitForTimeout(300);
  assert((await mside.boundingBox()).x < -200, 'closed by outside tap');
  ok('mobile: opened by button, closes on outside tap');

  assert(!errors.length, errors.join('\n'));
  console.log('\nSIDEBAR CHECK PASSED');
} catch (e) {
  console.error('FAILED:', e.message);
  process.exitCode = 1;
} finally {
  await b.close();
}
