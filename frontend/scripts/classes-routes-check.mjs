// Classes routes: /cabinet/school/classes[?type=1|0|2], legacy /classes redirect.
// It prepares its own data (a new teacher with two individual classes, through the administrator), so the counts do
// not depend on the seed, on other checks or on their order. It therefore runs only against an isolated stack:
//   WEB_URL=http://localhost:3452 API_URL=http://localhost:8452 node scripts/classes-routes-check.mjs
import { chromium } from 'playwright-core';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const RUN = runId();
const ROUTE = '/cabinet/school/classes';
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome' });
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const foreign = [];
try {
  // Own data: a teacher with two individual classes (Данияр, Алия) and no marathons.
  const users = await makeUsers(stack, RUN, { students: ['Данияр', 'Алия'] });
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  for (const [i, [lang, st]] of [['Английский', users.students[0]], ['Немецкий', users.students[1]]].entries()) {
    await api.must('POST', '/classrooms', { name: `${lang} — ${st.name}`, lesson_type: 'individual', student_id: st.id, cover_color: i ? '#FF7A45' : '#1AC6FF' }, 201);
  }
  await api.dispose();

  const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('request', (r) => { if (new URL(r.url()).hostname.endsWith('edvibe.com')) foreign.push(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(users.teacher.email);
  await p.getByLabel('Пароль').fill(users.teacher.password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(`${WEB}${ROUTE}`);
  ok(`after login: ${ROUTE}`);

  const main = p.locator('main');
  const url = () => p.url().replace(WEB, '');
  const count = async () => Number((await main.getByText(/Кол-во классов \d+/).innerText()).match(/\d+/)[0]);
  const active = async () => main.getByRole('radio', { checked: true }).filter({ hasText: /Все|Индивидуальные|Групповые|Марафоны/ }).innerText();
  const open = async (path) => { await p.goto(WEB + path); await main.getByRole('radiogroup', { name: 'Тип класса' }).waitFor(); await p.waitForLoadState('networkidle'); await p.waitForTimeout(600); };

  await open(ROUTE);
  assert((await active()) === 'Все' && (await count()) === 2, `all: ${await active()} ${await count()}`);
  await open(`${ROUTE}?type=1`);
  assert((await active()) === 'Индивидуальные' && (await count()) === 2 && url() === `${ROUTE}?type=1`, `type=1: ${await active()} ${url()}`);
  await open(`${ROUTE}?type=2`);
  // Marathons are real classes now: either a counter with cards or the empty state; never the old stub.
  assert((await active()) === 'Марафоны' && (await main.getByText('Марафоны недоступны на этом этапе').count()) === 0
    && ((await main.getByText(/Кол-во классов \d+/).count()) === 1 || (await main.getByText('Создайте класс для проведения обучения').isVisible())), 'type=2');
  await open(`${ROUTE}?type=0`);
  assert((await active()) === 'Групповые' && url() === `${ROUTE}?type=0` && (await main.getByText('Создайте класс для проведения обучения').isVisible()), `type=0: ${await active()} ${url()}`);
  await open(`${ROUTE}?type=group`);
  assert((await active()) === 'Групповые' && url() === `${ROUTE}?type=0`, `legacy type=group → ${url()}`);
  ok('direct: /cabinet/school/classes (Все), ?type=1 (Индивидуальные), ?type=0 (Групповые, old ?type=group rewritten), ?type=2 (Марафоны)');

  await open(`/classes?type=1&q=${encodeURIComponent('данияр')}`);
  assert(url() === `${ROUTE}?type=1&q=${encodeURIComponent('данияр')}`, `redirect url ${url()}`);
  assert((await active()) === 'Индивидуальные' && (await main.getByRole('searchbox').inputValue()) === 'данияр' && (await count()) === 1, 'redirect kept filter + search');
  await open('/classes');
  assert(url() === ROUTE, `plain redirect ${url()}`);
  ok('legacy /classes → /cabinet/school/classes, query (type, q) preserved');

  // UI → URL
  await main.getByRole('radio', { name: 'Марафоны' }).click();
  await p.waitForURL(`${WEB}${ROUTE}?type=2`);
  await main.getByRole('radio', { name: 'Групповые' }).click();
  await p.waitForURL(`${WEB}${ROUTE}?type=0`);
  await main.getByRole('radio', { name: 'Индивидуальные' }).click();
  await p.waitForURL(`${WEB}${ROUTE}?type=1`);
  await main.getByRole('searchbox').fill('алия');
  await p.waitForURL(`${WEB}${ROUTE}?type=1&q=${encodeURIComponent('алия')}`);
  await p.waitForTimeout(500);
  assert((await count()) === 1, 'search via URL');
  await p.reload();
  await main.getByText(/Кол-во классов \d+/).waitFor();
  await p.waitForTimeout(500);
  assert((await main.getByRole('searchbox').inputValue()) === 'алия' && (await active()) === 'Индивидуальные', 'reload keeps state');
  await main.getByRole('radio', { name: 'Все' }).click();
  await p.waitForURL(`${WEB}${ROUTE}?q=${encodeURIComponent('алия')}`);
  await main.getByRole('searchbox').fill('');
  await p.waitForURL(`${WEB}${ROUTE}`);
  ok('chips and search update the URL; reload restores them');

  await open(`${ROUTE}?type=9`);
  assert(url() === ROUTE && (await active()) === 'Все', `unknown type normalised: ${url()}`);
  ok('unknown ?type falls back to Все');

  const navLink = p.locator('#app-sidebar').getByRole('link', { name: 'Классы' });
  assert((await navLink.getAttribute('href')) === ROUTE && (await navLink.getAttribute('aria-current')) === 'page', 'sidebar link + active');
  await main.locator('article').filter({ hasText: 'Английский — Данияр' }).getByRole('link', { name: /Войти в класс/ }).click();
  await p.waitForURL(/\/classes\/[0-9a-f-]{36}$/);
  await p.locator('#classroom-toolbar').waitFor();
  ok(`classroom still at /classes/:id (${url()})`);
  await p.getByRole('link', { name: 'К списку классов' }).click();
  await p.waitForURL(`${WEB}${ROUTE}`);
  ok('classroom "back to classes" leads to the new route');

  assert(foreign.length === 0, `requests to edvibe.com: ${foreign}`);
  ok('no requests or redirects to edvibe.com');
  console.log('\nCLASSES ROUTES CHECK PASSED');
} catch (e) {
  console.error('FAILED:', e.message);
  process.exitCode = 1;
} finally {
  await b.close();
}
