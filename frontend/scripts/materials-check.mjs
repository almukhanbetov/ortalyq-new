// «Материалы» → «Личные материалы» in the browser: empty state → create a material (title, description, cover) →
// it opens and survives a reload → add a lesson → search by material and lesson title (kept in the address, survives a
// reload) → no-results + reset → edit and reload → another teacher and a student get "not found" by a direct link.
// Plus: no console errors, no requests to external domains, no sideways scroll on a phone.
// It creates its own users through the administrator and creates data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/materials-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/materials/check-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });

async function login(email, password, viewport = { width: 1440, height: 900 }) {
  const ctx = await b.newContext({ viewport, locale: 'ru-RU' });
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
  return { ctx, p };
}
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(400));

try {
  const users = await makeUsers(stack, RUN, { students: ['Данияр'] });
  const other = (await makeUsers(stack, RUN + 'x', { teacher: 'Другой учитель', students: [] })).teacher;
  const T = await login(users.teacher.email, users.teacher.password);
  const p = T.p;
  const main = p.locator('main');
  // Material cards; the first card of the grid is «Создать материал».
  const cards = main.locator('ul.grid > li:not(:has([data-testid="create-material-card"]))');
  const count = async () => Number((await main.getByText(/Кол-во материалов \d+/).innerText()).match(/\d+/)[0]);
  const cardTitles = async () => (await cards.locator('h2').allInnerTexts()).map((s) => s.trim());

  // ---- 1. Menu → «Материалы» → «Личные материалы», empty state ----
  await p.locator('#app-sidebar').getByRole('link', { name: 'Материалы' }).click();
  await p.waitForURL(/\/materials$/);
  await main.getByRole('heading', { name: 'Материалы', exact: true }).waitFor();
  assert(await main.getByRole('link', { name: 'Личные материалы' }).getAttribute('aria-current') === 'page', 'personal tab current');
  await main.getByText('У вас пока нет личных материалов').waitFor();
  await p.screenshot({ path: `${OUT}/01-empty.png` });
  ok('menu opens «Материалы» with the «Личные материалы» tab and an empty state');

  // ---- 2. Create a material: it opens, keeps data after a reload ----
  const title = `Present Simple: практикум ${RUN}`;
  assert(await main.locator('ul.grid > li').first().getByTestId('create-material-card').count() === 1, '«Создать материал» is the first card');
  await main.getByTestId('create-material-card').click();
  const dlg = p.getByRole('dialog', { name: 'Создать материал' });
  await dlg.getByLabel('Название').fill(title);
  await dlg.getByLabel('Описание').fill('Тестовый материал: утвердительные и отрицательные предложения.');
  await p.screenshot({ path: `${OUT}/02-create-dialog.png` });
  await dlg.getByRole('button', { name: 'Создать', exact: true }).click();
  await p.waitForURL(/\/materials\/courses\/[0-9a-f-]{36}$/);
  const materialPath = new URL(p.url()).pathname;
  await main.getByRole('heading', { name: title }).waitFor();
  await main.getByRole('button', { name: 'Добавить урок' }).click();
  await p.getByRole('dialog', { name: 'Добавить урок' }).getByLabel('Название урока').fill('Урок 1. Daily routine');
  await p.getByRole('dialog', { name: 'Добавить урок' }).getByRole('button', { name: 'Сохранить' }).click();
  await main.getByRole('link', { name: /Урок 1\. Daily routine/ }).waitFor();
  await p.reload();
  await main.getByRole('heading', { name: title }).waitFor();
  assert(await main.getByText('Тестовый материал: утвердительные').isVisible(), 'description after reload');
  assert(await main.getByRole('link', { name: /Урок 1\. Daily routine/ }).isVisible(), 'lesson after reload');
  await p.screenshot({ path: `${OUT}/03-material-after-reload.png` });
  ok('material created with a lesson, opens and keeps its data after a reload');

  // A second material (through the API, the browser part is covered above) so that search really narrows.
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  await api.must('POST', '/courses', { title: `Kazakh basics ${RUN}`, cover_color: '#FF7A45' }, 201);

  // ---- 3. Search: by title, by lesson title, kept in the address and after a reload ----
  await main.getByRole('link', { name: 'К материалам' }).first().click();
  await p.waitForURL(/\/materials$/);
  await settle(p);
  assert((await count()) === 2 && (await cards.count()) === 2, 'two materials listed');
  await p.screenshot({ path: `${OUT}/04-list.png` });
  const box = main.getByRole('searchbox', { name: 'Поиск по названию' });
  await box.fill('ПРАКТИКУМ');
  await p.waitForURL(/[?&]q=%D0%9F/);
  await settle(p);
  assert((await count()) === 1 && (await cardTitles())[0] === title, `search by title: ${await cardTitles()}`);
  await p.reload();
  await settle(p);
  assert((await box.inputValue()) === 'ПРАКТИКУМ' && (await count()) === 1, 'search survives a reload');
  await p.screenshot({ path: `${OUT}/05-search.png` });
  await box.fill('daily routine');
  await settle(p);
  assert((await count()) === 1 && (await cardTitles())[0] === title, 'search by lesson title');
  await box.fill('нет такого материала');
  await settle(p);
  await main.getByText('Материалы не найдены').waitFor();
  await p.screenshot({ path: `${OUT}/06-no-results.png` });
  await main.getByRole('button', { name: 'Сбросить поиск' }).click();
  await settle(p);
  assert((await count()) === 2 && !new URL(p.url()).search, 'reset clears the search and the address');
  ok('search by material and lesson title, kept in the address, survives a reload; no-results with reset');

  // ---- 4. Open from the card and edit; reload keeps the change; search finds the new title ----
  await cards.filter({ hasText: title }).getByRole('link').click();
  await p.waitForURL(new RegExp(materialPath + '$'));
  const edited = `Present Simple — разговорный практикум ${RUN}`;
  await main.getByRole('button', { name: 'Редактировать' }).click();
  const ed = p.getByRole('dialog', { name: 'Редактировать материал' });
  // The form is filled by an effect right after the dialog opens: wait for it instead of reading the first frame.
  await ed.getByLabel('Название').evaluate((el, v) => new Promise((ok, fail) => {
    const t0 = Date.now(); const tick = () => (el.value === v ? ok() : Date.now() - t0 > 5000 ? fail(new Error('edit dialog prefilled')) : requestAnimationFrame(tick)); tick();
  }), title);
  await ed.getByLabel('Название').fill(edited);
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.waitFor({ state: 'detached' });
  await p.reload();
  await main.getByRole('heading', { name: edited }).waitFor();
  await p.screenshot({ path: `${OUT}/07-edited.png` });
  await p.goto(`${WEB}/materials?q=${encodeURIComponent('разговорный')}`);
  await settle(p);
  assert((await count()) === 1 && (await cardTitles())[0] === edited, 'edited title is searchable');
  ok('material edited from its page; change survives a reload and is found by search');

  // ---- 5. Direct link: another teacher and a student do not get the material ----
  const O = await login(other.email, other.password);
  await O.p.goto(`${WEB}${materialPath}`);
  await O.p.locator('main').getByText('Материал не найден').waitFor();
  assert((await O.p.getByText(edited).count()) === 0, 'other teacher does not see the title');
  await O.p.screenshot({ path: `${OUT}/08-other-teacher-direct-link.png` });
  await O.p.goto(`${WEB}/materials`);
  await O.p.locator('main').getByText('У вас пока нет личных материалов').waitFor();
  const S = await login(users.students[0].email, users.students[0].password);
  await S.p.goto(`${WEB}${materialPath}`);
  await settle(S.p);
  assert((await S.p.getByText(edited).count()) === 0, 'student does not see the material');
  await S.p.screenshot({ path: `${OUT}/09-student-direct-link.png` });
  ok('another teacher gets «Материал не найден» by a direct link and an empty list; a student does not see it');

  // ---- 6. Phone width ----
  const M = await login(users.teacher.email, users.teacher.password, { width: 390, height: 844 });
  for (const path of ['/materials', materialPath]) {
    await M.p.goto(`${WEB}${path}`);
    await settle(M.p);
    const sw = await M.p.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    assert(sw <= 0, `sideways scroll on ${path}: ${sw}px`);
  }
  await M.p.screenshot({ path: `${OUT}/10-phone-material.png` });
  ok('390px: no sideways scroll on the list and the material page');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
