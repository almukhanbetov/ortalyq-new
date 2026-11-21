// Classroom tool rail check (no messages are sent, no answers changed).
// It prepares its own class with an open lesson (a new teacher and student through the administrator), so it does not
// depend on the seed, on e2e.mjs having opened a lesson before, or on the order of checks. Isolated stack only:
//   WEB_URL=http://localhost:3452 API_URL=http://localhost:8452 SHOTS_DIR=… node scripts/classroom-toolbar-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB } = stack;
const RUN = runId();
const OUT = process.env.SHOTS_DIR || '../docs/screenshots/classroom-toolbar';
mkdirSync(OUT, { recursive: true });
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: !process.env.HEADED });
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];

async function enterClass(email, pw, size = { width: 1440, height: 900 }, mobile = false) {
  const ctx = await b.newContext({ viewport: size, locale: 'ru-RU', ...(mobile ? { isMobile: true, hasTouch: true, deviceScaleFactor: 2 } : {}) });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль').fill(pw);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL(/\/classes$/);
  // Scenario from the video: Классы → Войти в класс
  await p.getByRole('article').filter({ hasText: CLASS_NAME }).getByRole('link', { name: 'Войти в класс' }).click();
  await p.waitForURL(/\/classes\/[0-9a-f-]+/);
  await p.locator('#classroom-toolbar').waitFor();
  await p.waitForTimeout(500);
  return { ctx, p };
}

// Own data: an individual class with the current lesson (one section), as a teacher sets it up before a lesson.
const users = await makeUsers(stack, RUN, { students: ['Данияр'] });
const [ST] = users.students;
const CLASS_NAME = `Английский — ${ST.name}`;
{
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const cls = await api.must('POST', '/classrooms', { name: CLASS_NAME, lesson_type: 'individual', student_id: ST.id, cover_color: '#1AC6FF' }, 201);
  const course = await api.must('POST', '/courses', { title: `Курс ${RUN}` }, 201);
  const lesson = await api.must('POST', `/courses/${course.id}/lessons`, { title: 'Present Simple' }, 201);
  await api.must('POST', `/lessons/${lesson.id}/sections`, { title: 'Разминка' }, 201);
  await api.must('PUT', `/classrooms/${cls.id}/current-lesson`, { lesson_id: lesson.id }, 200);
  await api.dispose();
}

try {
  for (const [role, email, pw] of [['teacher', users.teacher.email, users.teacher.password], ['student', ST.email, ST.password]]) {
    const { ctx, p } = await enterClass(email, pw);
    const rail = p.locator('#classroom-toolbar');
    const w = async () => Math.round((await rail.boundingBox()).width);
    const lessonX = async () => Math.round((await p.locator('main h1').first().boundingBox()).x);
    const label = (name) => rail.getByRole('button', { name }).locator('span[aria-hidden]').evaluate((el) => getComputedStyle(el).opacity);
    await p.mouse.move(800, 500);
    await p.waitForTimeout(300);
    const x0 = await lessonX();
    assert((await w()) === 64, `${role}: initial width ${await w()}`);
    assert((await label('Чат')) === '0', `${role}: labels hidden initially`);
    if (role === 'teacher') await p.screenshot({ path: `${OUT}/01-narrow.png` });

    await rail.getByRole('button', { name: 'Чат' }).hover();
    await p.waitForTimeout(320);
    assert((await w()) === 232, `${role}: expands on hover (${await w()})`);
    assert((await label('Чат')) === '1', `${role}: labels visible on hover`);
    assert((await lessonX()) === x0, `${role}: lesson did not shift`);
    for (const name of ['Разделы', 'Видеосвязь — Недоступно на этом этапе', 'Таймер — Недоступно на этом этапе']) {
      await rail.getByRole('button', { name }).hover();
      await p.waitForTimeout(60);
      assert((await w()) === 232, `${role}: stays open over ${name}`);
    }
    if (role === 'teacher') await p.screenshot({ path: `${OUT}/02-hover-expanded.png` });
    ok(`${role}: Классы → Войти в класс → hover: 64→232px over the lesson, lesson x unchanged`);

    // Disabled tool: click does nothing.
    await rail.getByRole('button', { name: 'Доска — Недоступно на этом этапе' }).click({ force: true });
    assert((await p.locator('aside[aria-label="Чат"]').count()) === 0, 'disabled tool opened something');

    // Open a tool (chat) from the expanded rail, then move into the opened tool panel.
    await rail.getByRole('button', { name: 'Чат' }).click();
    const chat = p.locator('aside[aria-label="Чат"]');
    await chat.waitFor();
    assert((await w()) === 232, `${role}: rail stays open while cursor is on it after click`);
    const input = chat.getByRole('textbox', { name: 'Написать сообщение…' });
    const ib = await input.boundingBox();
    await p.mouse.move(ib.x + 40, ib.y + 10, { steps: 8 });
    await p.waitForTimeout(90);
    assert((await w()) === 232, `${role}: still open during close delay`);
    await p.waitForTimeout(400);
    assert((await w()) === 64, `${role}: collapsed after cursor left (${await w()})`);
    assert(await chat.isVisible(), `${role}: chat stays open after leaving the rail`);
    await input.click();
    await input.fill('черновик (не отправляется)');
    assert((await input.inputValue()) === 'черновик (не отправляется)', 'typing in tool panel works');
    await input.fill('');
    assert((await rail.getByRole('button', { name: 'Чат' }).getAttribute('aria-pressed')) === 'true', 'chat tool marked active');
    if (role === 'teacher') await p.screenshot({ path: `${OUT}/03-tool-open-rail-collapsed.png` });
    ok(`${role}: tool opened; moving to its panel collapses the rail after 180ms, chat stays open and usable`);

    // Return to the rail and close the tool through it; then open the "Разделы" tool.
    await rail.getByRole('button', { name: 'Чат' }).hover();
    await p.waitForTimeout(300);
    assert((await w()) === 232, 'reopens on hover');
    await rail.getByRole('button', { name: 'Чат' }).click();
    await chat.waitFor({ state: 'detached' });
    await rail.getByRole('button', { name: 'Разделы' }).click();
    await p.waitForTimeout(200);
    await rail.getByRole('button', { name: 'Разделы' }).click();
    await p.locator('aside[aria-label="Разделы"]').waitFor();
    await p.mouse.move(800, 500);
    await p.waitForTimeout(500);
    assert((await w()) === 64, 'collapsed after toggling sections');
    ok(`${role}: existing tool handlers work (chat toggle, sections toggle)`);

    // Keyboard: Tab into the rail expands it; leaving collapses.
    await p.evaluate(() => document.activeElement?.blur());
    let inRail = false;
    for (let i = 0; i < 12 && !inRail; i++) {
      await p.keyboard.press('Tab');
      inRail = await p.evaluate(() => document.getElementById('classroom-toolbar').contains(document.activeElement));
    }
    await p.waitForTimeout(300);
    assert(inRail && (await w()) === 232, `${role}: keyboard focus expands`);
    for (let i = 0; i < 12; i++) {
      if (!(await p.evaluate(() => document.getElementById('classroom-toolbar').contains(document.activeElement)))) break;
      await p.keyboard.press('Tab');
    }
    await p.waitForTimeout(350);
    assert((await w()) === 64, `${role}: collapses when focus leaves`);
    ok(`${role}: keyboard focus expands/collapses`);
    await ctx.close();
  }

  // Phone: bottom bar, tools open by tap, no hover expansion.
  const { ctx: mctx, p: mp } = await enterClass(ST.email, ST.password, { width: 390, height: 844 }, true);
  const mrail = mp.locator('#classroom-toolbar');
  const mb = await mrail.boundingBox();
  assert(mb.width === 390 && mb.y > 700, 'phone: bottom bar');
  await mrail.getByRole('button', { name: 'Чат' }).tap();
  await mp.locator('aside[aria-label="Чат"]').waitFor();
  await mp.screenshot({ path: `${OUT}/04-mobile-chat-tap.png` });
  await mp.getByRole('button', { name: 'Закрыть чат' }).tap();
  assert((await mrail.boundingBox()).width === 390, 'phone: bar unchanged');
  await mp.waitForTimeout(400); // let the 150ms colour transition finish before the screenshot
  assert((await mrail.getByRole('button', { name: 'Чат' }).getAttribute('aria-pressed')) === 'false', 'phone: chat tool inactive after close');
  await mp.screenshot({ path: `${OUT}/05-mobile-bar.png` });
  await mctx.close();
  ok('phone: tools in bottom bar open by tap');

  assert(!errors.length, errors.join('\n'));
  console.log('\nCLASSROOM TOOLBAR CHECK PASSED');
} catch (e) {
  console.error('FAILED:', e.message);
  process.exitCode = 1;
} finally {
  await b.close();
}
