// Marathons, stage 1: create, list cards, marathon page, basic settings, archive.
// It CREATES and ARCHIVES a marathon, so run it against an isolated stack (seeded test DB), not the demo DB:
//   WEB_URL=http://localhost:3449 node scripts/marathon-stage1-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';

const WEB = process.env.WEB_URL || 'http://localhost:3449';
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/marathons/stage1';
const LIST = '/cabinet/school/classes';
mkdirSync(OUT, { recursive: true });
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const foreign = [];

async function login(email, pw, opts = {}) {
  const ctx = await b.newContext({ viewport: { width: 1862, height: 961 }, locale: 'ru-RU', ...opts });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('request', (r) => { const h = new URL(r.url()).hostname; if (h !== 'localhost' && h !== '127.0.0.1') foreign.push(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(pw);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes$/);
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return { ctx, p };
}
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(400));

try {
  const { ctx, p } = await login('teacher@ortalyq.local', 'teacher-local-123');
  const main = p.locator('main');

  // 1. Empty marathons tab → create via the type chooser.
  await p.goto(`${WEB}${LIST}?type=2`);
  await settle(p);
  await main.getByRole('button', { name: 'Создать класс' }).first().click();
  const chooser = p.getByRole('dialog', { name: 'Создать класс' });
  await chooser.waitFor();
  assert(await chooser.getByRole('button', { name: /Групповой класс/ }).isDisabled(), 'group type disabled');
  await p.screenshot({ path: `${OUT}/01-type-chooser.png` });
  await chooser.getByRole('button', { name: /Марафоны/ }).click();
  const form = p.getByRole('dialog', { name: /Марафон/ });
  await form.waitFor();
  const create = form.getByRole('button', { name: 'Создать', exact: true });
  assert(await create.isDisabled(), 'create disabled without a name');
  await form.getByLabel('Название марафона').fill('Проверочный марафон');
  assert(await create.isDisabled(), 'create disabled without a language');
  await form.getByLabel('Направление').selectOption('other');
  assert((await form.getByLabel('Язык').count()) === 0 && !(await create.isDisabled()), '"Другое" hides the language');
  await form.getByLabel('Направление').selectOption('languages');
  await form.getByLabel('Язык').selectOption('en');
  await form.getByRole('button', { name: /Цвет обложки #FF7A45/ }).click();
  await p.screenshot({ path: `${OUT}/02-create-form.png` });
  await create.click();
  // Right after creation «Настройки марафона» opens by itself: no «Основные» (just filled in), «Прохождение» first,
  // «Настроить позже» keeps the defaults (mode «Открытый»).
  const setup = p.getByRole('dialog', { name: 'Настройки марафона' });
  await setup.waitFor();
  await p.waitForURL(/\/cabinet\/school\/marathons\/marathon\/[0-9a-f-]{36}\/students$/);
  const id = p.url().match(/marathon\/([0-9a-f-]{36})/)[1];
  const tabs = await setup.getByRole('tab').allInnerTexts();
  assert(tabs.join('|') === 'Прохождение|Дополнительно|Оформление', `setup tabs: ${tabs}`);
  assert(await setup.getByRole('tab', { name: 'Прохождение' }).getAttribute('aria-selected') === 'true', 'setup starts on «Прохождение»');
  await p.waitForTimeout(300);
  await p.screenshot({ path: `${OUT}/02b-settings-after-create.png` });
  await setup.getByRole('button', { name: 'Настроить позже' }).click();
  await setup.waitFor({ state: 'hidden' });
  await settle(p);
  await p.reload();
  await settle(p);
  assert(await p.getByRole('dialog', { name: 'Настройки марафона' }).count() === 0, 'setup dialog must not reopen after reload');
  assert(await main.getByRole('heading', { level: 1, name: 'Проверочный марафон' }).isVisible(), 'title');
  assert(await main.getByText('Открытый').isVisible() && await main.getByText('Английский').isVisible(), 'mode and language chips');
  assert(await main.getByRole('img', { name: 'Учеников: 0' }).isVisible() && await main.getByRole('img', { name: 'Уроков: 0' }).isVisible(), 'counters');
  assert(await main.getByText('Для проведения онлайн-курсов и марафонов добавьте учеников').isVisible(), 'students empty state');
  assert((await p.locator('#app-sidebar').getByRole('link', { name: 'Классы' }).getAttribute('aria-current')) === 'page', 'Классы active in menu');
  await p.mouse.move(1500, 900);
  await p.waitForTimeout(400);
  await p.screenshot({ path: `${OUT}/03-marathon-students.png` });
  ok(`created via chooser → /marathons/marathon/${id}/students with header, chips, empty state`);

  // 2. Tabs are routes; reload keeps the tab; unknown tab → students.
  await main.getByRole('tab', { name: 'Урок' }).click();
  await p.waitForURL(`${WEB}/cabinet/school/marathons/marathon/${id}/lessons`);
  await p.reload();
  await settle(p);
  assert((await main.getByRole('tab', { name: 'Урок' }).getAttribute('aria-selected')) === 'true', 'lessons tab after reload');
  await p.screenshot({ path: `${OUT}/04-marathon-lessons.png` });
  await p.goto(`${WEB}/cabinet/school/marathons/marathon/${id}/foo`);
  await p.waitForURL(`${WEB}/cabinet/school/marathons/marathon/${id}/students`);
  await p.goto(`${WEB}/cabinet/school/marathons/marathon/${id}`);
  await p.waitForURL(`${WEB}/cabinet/school/marathons/marathon/${id}/students`);
  ok('tabs: students / lessons / statistics in the URL; reload keeps it; bare and unknown tab → students');

  // 3. Cards: marathons tab shows the mode, "Все" shows "Марафон"; the card opens the marathon page.
  await p.goto(`${WEB}${LIST}?type=2`);
  await settle(p);
  const card = main.locator('article').filter({ hasText: 'Проверочный марафон' });
  assert(await card.getByText('Открытый').isVisible(), 'mode chip on marathons tab');
  await p.screenshot({ path: `${OUT}/05-list-marathons.png` });
  await p.goto(`${WEB}${LIST}`);
  await settle(p);
  assert(await main.locator('article').filter({ hasText: 'Проверочный марафон' }).getByText('Марафон', { exact: true }).isVisible(), '"Марафон" chip on Все');
  await p.screenshot({ path: `${OUT}/06-list-all.png` });
  await main.locator('article').filter({ hasText: 'Проверочный марафон' }).getByRole('link').click();
  await p.waitForURL(`${WEB}/cabinet/school/marathons/marathon/${id}/students`);
  await p.goto(`${WEB}/classes/${id}`);
  await p.waitForURL(`${WEB}/cabinet/school/marathons/marathon/${id}/students`);
  ok('cards: mode on ?type=2, "Марафон" on Все; card and old /classes/:id lead to the marathon page');

  // 4. Settings: validation, save, values re-read on reopen.
  await settle(p);
  await main.getByRole('button', { name: 'Настройки' }).click();
  const st = p.getByRole('dialog', { name: 'Настройки марафона' });
  await st.waitFor();
  assert((await st.getByLabel('Название марафона').inputValue()) === 'Проверочный марафон' && (await st.getByLabel('Язык').inputValue()) === 'en', 'settings prefilled');
  await st.getByLabel('Название марафона').fill('   ');
  assert(await st.getByRole('button', { name: 'Сохранить' }).isDisabled(), 'save disabled for blank name');
  await st.getByLabel('Название марафона').fill('Марафон по немецкому');
  await st.getByLabel('Язык').selectOption('de');
  await p.screenshot({ path: `${OUT}/07-settings.png` });
  await st.getByRole('button', { name: 'Сохранить' }).click();
  await st.waitFor({ state: 'detached' });
  await settle(p);
  assert(await main.getByRole('heading', { level: 1, name: 'Марафон по немецкому' }).isVisible() && await main.getByText('Немецкий').isVisible(), 'saved values shown');
  await p.reload();
  await settle(p);
  await main.getByRole('button', { name: 'Настройки' }).click();
  assert((await p.getByRole('dialog', { name: 'Настройки марафона' }).getByLabel('Язык').inputValue()) === 'de', 'saved after reload');
  await p.keyboard.press('Escape');
  ok('settings: blank name blocked, name and language saved and survive reload');

  // 5. Student: no access to a foreign marathon.
  const { ctx: sctx, p: sp } = await login('student@ortalyq.local', 'student-local-123');
  await sp.goto(`${WEB}/cabinet/school/marathons/marathon/${id}/students`);
  await sp.getByText('Марафон не найден').waitFor();
  await sp.goto(`${WEB}${LIST}?type=2`);
  await settle(sp);
  // Only this foreign marathon is checked: since stage 2–4 the student may be enrolled in other marathons of the DB.
  assert((await sp.locator(`main a[href*="${id}"]`).count()) === 0, 'student does not see the foreign marathon');
  await sctx.close();
  ok('student: foreign marathon → «Марафон не найден», not in the student list');

  // 6. Phone layout.
  const { ctx: mctx, p: mp } = await login('teacher@ortalyq.local', 'teacher-local-123', { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 });
  await mp.goto(`${WEB}/cabinet/school/marathons/marathon/${id}/students`);
  await settle(mp);
  const overflow = await mp.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(overflow <= 0, `horizontal overflow ${overflow}px`);
  await mp.screenshot({ path: `${OUT}/08-mobile.png`, fullPage: true });
  await mctx.close();
  ok('phone 390px: no horizontal scroll');

  // 7. Delete = archive: confirmation, back to the list, page gone.
  await main.getByRole('button', { name: 'Настройки' }).click();
  await p.getByRole('dialog', { name: 'Настройки марафона' }).getByRole('button', { name: 'Удалить' }).click();
  const confirm = p.getByRole('dialog', { name: 'Удалить марафон?' });
  await confirm.waitFor();
  await p.screenshot({ path: `${OUT}/09-delete-confirm.png` });
  await confirm.getByRole('button', { name: 'Удалить' }).click();
  await p.waitForURL(`${WEB}${LIST}?type=2`);
  await settle(p);
  // By id: repeated runs on the same DB may leave other marathons with the same name.
  assert((await main.locator(`a[href*="${id}"]`).count()) === 0, 'archived marathon hidden');
  await p.goto(`${WEB}/cabinet/school/marathons/marathon/${id}/students`);
  await p.getByText('Марафон не найден').waitFor();
  await ctx.close();
  ok('delete: confirmation → archived, hidden from the list, page shows «Марафон не найден»');

  assert(!errors.length, `page errors:\n${errors.join('\n')}`);
  assert(!foreign.length, `external requests: ${foreign.join(', ')}`);
  console.log('\nMARATHON STAGE 1 CHECK PASSED');
} catch (e) {
  console.error('FAILED:', e.message);
  process.exitCode = 1;
} finally {
  await b.close();
}
