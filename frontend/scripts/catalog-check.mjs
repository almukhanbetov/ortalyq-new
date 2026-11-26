// «Материалы» → «Каталог» in the browser, for a new teacher of this run who does not have the course yet:
// the «Каталог» tab next to «Личные материалы» → search «первые» (kept in the address) → the card → the course window
// (description, level A1, three lessons) → «Добавить в мои материалы» → «Открыть в моих материалах» → the copy in
// «Личные материалы» → Lesson 1 with its text blocks, exercises and homework. Adding again (button and API) keeps
// one copy. Plus: no console errors, no external requests.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/catalog-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/catalog/check-${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const COURSE = 'English A1 — Первые разговоры';
const KEY = 'english-a1-first-conversations';
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(300));

try {
  const users = await makeUsers(stack, RUN, { students: [] });
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const mine = async () => (await api.must('GET', '/courses', undefined, 200)).items.filter((c) => c.title === COURSE);
  assert((await mine()).length === 0, 'the new teacher has no copy yet');

  const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 4(01|03|04|09|22)/.test(m.text())) errors.push(m.text()); });
  p.on('request', (r) => { const u = new URL(r.url()); if (/^https?:$/.test(u.protocol) && !['localhost', '127.0.0.1'].includes(u.hostname)) external.add(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(users.teacher.email);
  await p.getByLabel('Пароль', { exact: true }).fill(users.teacher.password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL((u) => !u.pathname.startsWith('/login'));
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));

  // ---- 1. The tab and the search ----
  await p.goto(`${WEB}/materials`);
  const tabs = p.getByRole('navigation', { name: 'Разделы материалов' });
  await tabs.getByRole('link', { name: 'Личные материалы' }).waitFor();
  await tabs.getByRole('link', { name: 'Каталог' }).click();
  await p.waitForURL((u) => u.pathname === '/materials/catalog');
  assert((await tabs.getByRole('link', { name: 'Каталог' }).getAttribute('aria-current')) === 'page', 'catalog tab is current');
  await p.getByRole('searchbox', { name: 'Поиск по каталогу' }).fill('химия');
  await p.getByText('Материалы не найдены').waitFor();
  await p.getByRole('searchbox', { name: 'Поиск по каталогу' }).fill('первые');
  await p.waitForURL((u) => u.searchParams.get('q') === 'первые');
  const card = p.getByRole('listitem').filter({ hasText: COURSE });
  await card.waitFor();
  assert((await p.getByRole('listitem').filter({ has: p.getByRole('heading') }).count()) === 1, 'one course found');
  assert((await card.innerText()).includes('Уровень A1') && !(await card.innerText()).includes('В моих материалах'), 'card: level, not owned');
  await p.reload();
  await card.waitFor();
  assert((await p.getByRole('searchbox', { name: 'Поиск по каталогу' }).inputValue()) === 'первые', 'search kept after reload');
  await settle(p);
  await p.screenshot({ path: `${OUT}/01-catalog-search.png` });
  ok('«Каталог» next to «Личные материалы»; the search finds the course (kept in the address), a wrong word finds nothing');

  // ---- 2. The course window and adding ----
  await card.getByRole('button', { name: COURSE }).click();
  const dlg = p.getByRole('dialog', { name: COURSE });
  await dlg.getByText('Уровень A1').waitFor();
  const text = await dlg.innerText();
  assert(text.includes('Авторский разговорный курс') && text.includes('Lesson 1. Hello!') && text.includes('Lesson 2. Where are you from?')
    && text.includes('Lesson 3. Meet my friend'), 'description and three lessons');
  await settle(p);
  await p.screenshot({ path: `${OUT}/02-catalog-course.png` });
  await dlg.getByRole('button', { name: 'Добавить в мои материалы' }).click();
  const open = dlg.getByRole('link', { name: 'Открыть в моих материалах' });
  await open.waitFor();
  await p.screenshot({ path: `${OUT}/03-catalog-added.png` });
  assert((await mine()).length === 1, 'one copy after adding');
  ok('the course window shows the description, level A1 and three lessons; «Добавить» turns into «Открыть в моих материалах»');

  // ---- 3. Adding again never makes a second copy ----
  const again = await api.call('POST', `/material-catalog/${KEY}/add`);
  assert(again.status === 200 && again.body.created === false, `repeated add: ${again.status} ${JSON.stringify(again.body)}`);
  await p.reload();
  await dlg.getByRole('link', { name: 'Открыть в моих материалах' }).waitFor();
  assert((await dlg.getByRole('button', { name: 'Добавить в мои материалы' }).count()) === 0, 'no add button for an owned course');
  assert((await mine()).length === 1, 'still one copy');
  await p.keyboard.press('Escape');
  await card.getByText('В моих материалах').waitFor();
  ok('adding again (API) returns the same copy; after a reload the catalog shows «Открыть в моих материалах» and «В моих материалах»');

  // ---- 4. «Личные материалы» → the copy → Lesson 1 ----
  await p.goto(`${WEB}/materials`);
  const myCards = p.getByRole('listitem').filter({ hasText: COURSE });
  await myCards.first().waitFor();
  assert((await myCards.count()) === 1, 'one course in «Личные материалы»');
  await settle(p);
  await p.screenshot({ path: `${OUT}/04-personal-materials.png` });
  await myCards.getByRole('link', { name: COURSE }).click();
  await p.getByText('Lesson 1. Hello!').click();
  const rows = p.locator('li.bg-surface');
  for (const s of ['Знакомство с темой', 'Практика', 'Разговор']) await p.getByText(s, { exact: true }).first().waitFor();
  for (const title of ['Dialogue', 'Useful phrases', 'Complete the introduction', 'Say your name', 'Choose the best reply', 'Speaking task']) {
    await rows.filter({ hasText: title }).first().waitFor();
  }
  assert((await rows.filter({ hasText: 'Заметки преподавателя' }).count()) === 3, 'three teacher-only notes');
  for (const n of await rows.filter({ hasText: 'Заметки преподавателя' }).all()) assert((await n.innerText()).includes('Скрыто'), 'notes stay hidden');
  await settle(p);
  await p.screenshot({ path: `${OUT}/05-lesson-1.png`, fullPage: true });
  await p.getByRole('tab', { name: 'Домашняя работа' }).click();
  await rows.filter({ hasText: 'Homework: complete the reply' }).waitFor();
  await rows.filter({ hasText: 'Homework: choose the correct sentence' }).waitFor();
  ok('the copy opens from «Личные материалы»: Lesson 1 has the text blocks, the exercises, hidden notes and the homework');

  // ---- 5. The copy is editable ----
  await p.getByRole('tab', { name: 'Урок' }).click();
  await rows.filter({ hasText: 'Dialogue' }).getByRole('button', { name: /Dialogue/ }).click();
  const ed = p.getByRole('dialog', { name: 'Редактирование текстового блока' });
  await ed.getByLabel('Заголовок').fill('Dialogue (my version)');
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.waitFor({ state: 'detached' });
  await rows.filter({ hasText: 'Dialogue (my version)' }).filter({ hasText: 'v2' }).waitFor();
  await api.must('POST', `/material-catalog/${KEY}/add`, undefined, 200);
  await p.reload();
  await rows.filter({ hasText: 'Dialogue (my version)' }).waitFor();
  assert((await mine()).length === 1, 'still one copy after editing and adding again');
  ok('the copy is editable; adding again keeps the edit and one copy');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`teacher: ${users.teacher.email}`);
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
