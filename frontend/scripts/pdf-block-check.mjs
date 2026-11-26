// «PDF-документ» in the browser, teacher and students in their own sessions.
// Teacher: «Материалы» → «Личные материалы» → the course → the lesson → the section → «Добавить упражнение» →
// «PDF-документ»; a wrong file and a too large file show a clear error; a real PDF uploads (name and state shown), is
// previewed, saved and shown in the lesson editor. The teacher opens the lesson in the class; the student sees the PDF
// in the block with «Открыть PDF в новой вкладке» and after a reload. The teacher replaces the file → the student's
// block shows the new file without a reload. A student of another class gets 404 for the file by a direct link and in
// the API. The block is deleted from the editor. Plus: no console errors, no external requests.
// It creates its own users and data, so it runs only against an isolated stack:
//   WEB_URL=http://localhost:3449 API_URL=http://localhost:8449 SHOTS_DIR=… node scripts/pdf-block-check.mjs
import { chromium } from 'playwright-core';
import { mkdirSync } from 'node:fs';
import { apiAs, isolatedStack, makeUsers, runId } from './lib/fixtures.mjs';

const stack = isolatedStack();
const { WEB, API } = stack;
const OUT = process.env.SHOTS_DIR || `../docs/screenshots/pdf-block/${new Date().toISOString().slice(0, 10)}`;
mkdirSync(OUT, { recursive: true });
const RUN = runId();
const ok = (s) => console.log(`✓ ${s}`);
const assert = (c, m) => { if (!c) throw new Error(m); };
const errors = [];
const external = new Set();
const b = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: process.env.HEADED !== '1' });
const settle = (p) => p.waitForLoadState('networkidle').then(() => p.waitForTimeout(400));

/** A valid PDF with one page per text line, big readable text (a correct cross-reference table). */
function pdf(title, lines) {
  const objs = ['<< /Type /Catalog /Pages 2 0 R >>', null, '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>', '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>'];
  const kids = [];
  for (const line of lines) {
    const s = `0.1 0.45 0.85 rg 0 742 612 50 re f 1 1 1 rg BT /F2 22 Tf 40 760 Td (${title}) Tj ET 0 0 0 rg BT /F1 30 Tf 60 600 Td (${line}) Tj ET BT /F1 14 Tf 60 560 Td (ORTALYQ test document) Tj ET`;
    objs.push(`<< /Length ${s.length} >>\nstream\n${s}\nendstream`);
    objs.push(`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents ${objs.length} 0 R /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> >>`);
    kids.push(`${objs.length} 0 R`);
  }
  objs[1] = `<< /Type /Pages /Kids [${kids.join(' ')}] /Count ${kids.length} >>`;
  let out = '%PDF-1.4\n';
  const offs = [];
  objs.forEach((o, i) => { offs.push(out.length); out += `${i + 1} 0 obj\n${o}\nendobj\n`; });
  const xref = out.length;
  out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n` + offs.map((o) => `${String(o).padStart(10, '0')} 00000 n \n`).join('');
  out += `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return Buffer.from(out, 'latin1');
}
const PDF1 = pdf('Worksheet 1 - Present Simple', ['I work in Almaty.', 'She reads every day.']);
const PDF2 = pdf('Worksheet 2 - Past Simple', ['I worked yesterday.']);

async function login(email, password) {
  const ctx = await b.newContext({ viewport: { width: 1440, height: 1100 }, locale: 'ru-RU' });
  const p = await ctx.newPage();
  p.on('pageerror', (e) => errors.push(e.message));
  p.on('console', (m) => { if (m.type() === 'error' && !/status of 4(01|03|04|09|13|22)/.test(m.text())) errors.push(m.text()); });
  p.on('request', (r) => { const u = new URL(r.url()); if (/^https?:$/.test(u.protocol) && !['localhost', '127.0.0.1'].includes(u.hostname)) external.add(r.url()); });
  await p.goto(`${WEB}/login`);
  await p.getByLabel('Email').fill(email);
  await p.getByLabel('Пароль', { exact: true }).fill(password);
  await p.getByRole('button', { name: 'Войти' }).click();
  await p.waitForURL((u) => !u.pathname.startsWith('/login'));
  await p.evaluate(() => localStorage.setItem('ortalyq_sidebar_pinned', '0'));
  return p;
}
/** Statuses of the PDF responses a page loads (the frame and the new tab), by exercise route. */
function pdfResponses(p) {
  const seen = [];
  p.context().on('response', (r) => { if (/\/exercises\/[0-9a-f-]{36}\/file/.test(r.url())) seen.push({ url: r.url(), status: r.status(), type: r.headers()['content-type'] }); });
  return seen;
}

try {
  const users = await makeUsers(stack, RUN, { students: ['Данияр', 'Алия'] });
  const [S1, S2] = users.students;
  const api = await apiAs(stack, users.teacher.email, users.teacher.password);
  const clsA = await api.must('POST', '/classrooms', { name: `Английский — Данияр ${RUN}`, student_id: S1.id }, 201);
  const clsB = await api.must('POST', '/classrooms', { name: `Английский — Алия ${RUN}`, student_id: S2.id }, 201);
  const COURSE = `Reading A2 ${RUN}`;
  const course = await api.must('POST', '/courses', { title: COURSE }, 201);
  const lesson = await api.must('POST', `/courses/${course.id}/lessons`, { title: 'Урок 1. Present Simple' }, 201);
  await api.must('POST', `/lessons/${lesson.id}/sections`, { title: 'Чтение' }, 201);

  // ---- 1. Teacher: «Материалы» → «Личные материалы» → course → lesson → section → «Добавить упражнение» ----
  const tp = await login(users.teacher.email, users.teacher.password);
  const tPdf = pdfResponses(tp);
  const main = tp.locator('main');
  await tp.locator('#app-sidebar').getByRole('link', { name: 'Материалы' }).click();
  await tp.waitForURL(/\/materials$/);
  assert(await main.getByRole('link', { name: 'Личные материалы' }).getAttribute('aria-current') === 'page', '«Личные материалы» tab');
  await main.locator('ul.grid > li').filter({ hasText: COURSE }).getByRole('link').first().click();
  await main.getByRole('link', { name: /Урок 1\. Present Simple/ }).click();
  await main.getByText('Чтение', { exact: true }).first().waitFor();
  await main.getByRole('button', { name: 'Добавить упражнение' }).first().click();
  const catalog = tp.getByRole('dialog', { name: 'Добавить упражнение' });
  await catalog.getByRole('button', { name: 'PDF-документ', exact: true }).waitFor();
  await settle(tp);
  await tp.screenshot({ path: `${OUT}/01-catalog.png` });
  await catalog.getByRole('button', { name: 'PDF-документ', exact: true }).click();
  const ed = tp.getByRole('dialog', { name: 'Новый PDF-документ' });
  await ed.waitFor();
  ok('path: «Материалы» → «Личные материалы» → course → lesson → «Чтение» → «Добавить упражнение» → «PDF-документ»');

  // ---- 2. Errors: no file, not a PDF (checked by the server), too large (checked at once) ----
  await ed.getByLabel('Заголовок').fill('Рабочий лист');
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.getByRole('alert').filter({ hasText: 'Выберите PDF-файл' }).waitFor();
  const input = ed.getByTestId('pdf-input');
  await input.setInputFiles({ name: 'homework.pdf', mimeType: 'application/pdf', buffer: Buffer.from('just a text file renamed to .pdf') });
  await ed.getByRole('alert').filter({ hasText: 'Файл «homework.pdf» — не PDF. Выберите документ в формате PDF.' }).waitFor();
  await tp.screenshot({ path: `${OUT}/02-error-not-pdf.png` });
  await input.setInputFiles({ name: 'scan.pdf', mimeType: 'application/pdf', buffer: Buffer.alloc(21 * 1024 * 1024, 0x20) });
  await ed.getByRole('alert').filter({ hasText: 'Файл «scan.pdf» больше 20 МБ' }).waitFor();
  const noFile = await api.call('POST', `/lessons/${lesson.id}/files`); // no multipart body
  assert(noFile.status === 422, `upload without a file: ${noFile.status}`);
  ok('errors: no file → «Выберите PDF-файл»; a renamed text → «… — не PDF» (server check); 21 MB → «… больше 20 МБ»');

  // ---- 3. A real PDF: name, upload state, preview, save ----
  await input.setInputFiles({ name: 'Worksheet 1.pdf', mimeType: 'application/pdf', buffer: PDF1 });
  await ed.getByTestId('pdf-file-name').filter({ hasText: 'Worksheet 1.pdf' }).waitFor();
  assert((await ed.getByTestId('pdf-file-state').innerText()).includes('Загружен'), 'uploaded state');
  assert(await ed.getByRole('alert').count() === 0, 'the error is gone');
  await ed.getByTestId('pdf-viewer').waitFor();
  await tp.waitForTimeout(1200);
  await tp.screenshot({ path: `${OUT}/03-editor-uploaded.png` });
  await ed.getByRole('button', { name: 'Сохранить' }).click();
  await ed.waitFor({ state: 'detached' });
  const row = main.locator('li[data-kind="pdf"]').filter({ hasText: 'Рабочий лист' });
  await row.getByText('Worksheet 1.pdf').first().waitFor();
  await row.getByTestId('pdf-viewer').waitFor();
  await settle(tp);
  await tp.waitForTimeout(1500);
  await tp.screenshot({ path: `${OUT}/04-lesson-editor.png`, fullPage: true });
  assert(tPdf.some((r) => r.status === 200 && r.type === 'application/pdf'), `teacher PDF frame: ${JSON.stringify(tPdf)}`);
  ok('the PDF uploads with its name and state, is previewed in the editor, saved and shown in the lesson editor block');

  // ---- 4. The teacher opens the lesson in the class; the student reads the PDF ----
  await tp.goto(`${WEB}/classes/${clsA.id}`);
  await tp.getByRole('button', { name: 'Выбрать урок' }).click();
  await tp.getByRole('dialog', { name: 'Другой урок' }).locator('li').filter({ hasText: 'Урок 1. Present Simple' }).getByRole('button', { name: 'Открыть урок' }).click();
  await tp.getByTestId('pdf-block').waitFor();
  const sp = await login(S1.email, S1.password);
  const sPdf = pdfResponses(sp);
  await sp.goto(`${WEB}/classes/${clsA.id}`);
  const block = sp.getByTestId('pdf-block').filter({ hasText: 'Рабочий лист' });
  await block.waitFor();
  assert(await block.getByRole('link', { name: 'Открыть PDF в новой вкладке' }).isVisible(), 'open-in-new-tab button');
  assert(await block.getByRole('button', { name: /Начать|Проверить|Ответить/ }).count() === 0 && await block.locator('input').count() === 0, 'no answer controls');
  await settle(sp);
  await sp.waitForTimeout(1500);
  await block.scrollIntoViewIfNeeded();
  await sp.screenshot({ path: `${OUT}/05-student-class.png` });
  assert(sPdf.some((r) => r.status === 200 && r.type === 'application/pdf' && r.url.includes(`/classrooms/${clsA.id}/`)), `student frame: ${JSON.stringify(sPdf)}`);
  ok('the student sees the PDF in the block of the open lesson (200, application/pdf), no answer field or grade');

  // «Открыть PDF в новой вкладке» opens the same checked address.
  const [tab] = await Promise.all([sp.context().waitForEvent('page'), block.getByRole('link', { name: 'Открыть PDF в новой вкладке' }).click()]);
  await tab.waitForLoadState().catch(() => {});
  await tab.waitForTimeout(1500);
  assert(tab.url().includes(`/api/classrooms/${clsA.id}/exercises/`), `new tab url: ${tab.url()}`);
  await tab.screenshot({ path: `${OUT}/06-student-new-tab.png` });
  await tab.close();
  ok('«Открыть PDF в новой вкладке» opens the PDF through the class route');

  // After a reload the block and the file are still there.
  sPdf.length = 0;
  await sp.reload();
  await block.waitFor();
  await settle(sp);
  await sp.waitForTimeout(1200);
  assert(sPdf.some((r) => r.status === 200), 'PDF after reload');
  await block.scrollIntoViewIfNeeded();
  await sp.screenshot({ path: `${OUT}/07-student-after-reload.png` });
  ok('after a reload the student sees the PDF again');

  // ---- 5. Replace the file: the student's block follows without a reload ----
  await tp.goto(`${WEB}/materials/lessons/${lesson.id}`);
  const lessonRow = tp.locator('main li[data-kind="pdf"]').filter({ hasText: 'Рабочий лист' });
  await lessonRow.getByRole('button', { name: /Рабочий лист/ }).click();
  const edit = tp.getByRole('dialog', { name: 'Редактирование PDF-документа' });
  await edit.getByTestId('pdf-file-name').filter({ hasText: 'Worksheet 1.pdf' }).waitFor();
  assert((await edit.getByTestId('pdf-file-state').innerText()).includes('Текущий файл'), 'current file state');
  assert(await edit.getByRole('button', { name: 'Заменить файл' }).isVisible(), '«Заменить файл» button');
  await edit.getByTestId('pdf-input').setInputFiles({ name: 'Worksheet 2.pdf', mimeType: 'application/pdf', buffer: PDF2 });
  await edit.getByTestId('pdf-file-name').filter({ hasText: 'Worksheet 2.pdf' }).waitFor();
  await edit.getByText('Ученики увидят новый файл после сохранения.').waitFor();
  await tp.waitForTimeout(1200);
  await tp.screenshot({ path: `${OUT}/08-replace.png` });
  await edit.getByRole('button', { name: 'Сохранить' }).click();
  await edit.waitFor({ state: 'detached' });
  await lessonRow.getByText('Worksheet 2.pdf').first().waitFor();
  await block.getByText('Worksheet 2.pdf').waitFor({ timeout: 15000 });
  await settle(sp);
  await sp.waitForTimeout(1500);
  await block.scrollIntoViewIfNeeded();
  await sp.screenshot({ path: `${OUT}/09-student-replaced.png` });
  const ex = (await api.must('GET', `/lessons/${lesson.id}/exercises?scope=lesson`, undefined, 200)).items.find((x) => x.kind === 'pdf');
  assert(ex.version.number === 2 && ex.version.file.name === 'Worksheet 2.pdf', `version after replace: ${JSON.stringify(ex.version)}`);
  const apiS1 = await apiAs(stack, S1.email, S1.password);
  ok('«Заменить файл» saves v2; the student\'s block shows Worksheet 2.pdf without a reload');

  // ---- 6. A student of another class: refused in the API, by a direct link and on the page ----
  const fileUrl = `/classrooms/${clsA.id}/exercises/${ex.id}/file`;
  const apiS2 = await apiAs(stack, S2.email, S2.password);
  const direct = await apiS2.call('GET', fileUrl);
  const viaOwn = await apiS2.call('GET', `/classrooms/${clsB.id}/exercises/${ex.id}/file`);
  const editorRoute = await apiS2.call('GET', `/exercises/${ex.id}/file`);
  const s1direct = await apiS1.call('GET', fileUrl);
  assert(direct.status === 404 && viaOwn.status === 404 && editorRoute.status === 403 && s1direct.status === 200,
    `access: other class ${direct.status}, own class ${viaOwn.status}, editor route ${editorRoute.status}, student ${s1direct.status}`);
  const s2p = await login(S2.email, S2.password);
  const resp = await s2p.goto(`${API}/api${fileUrl}`);
  assert(resp.status() === 404 && !(resp.headers()['content-type'] || '').includes('pdf'), `direct link in the browser: ${resp.status()}`);
  await s2p.screenshot({ path: `${OUT}/10-other-class-direct-link.png` });
  await s2p.goto(`${WEB}/classes/${clsA.id}`);
  await s2p.waitForTimeout(1500);
  assert(await s2p.getByTestId('pdf-block').count() === 0, 'the other student sees no block');
  ok('a student of another class: 404 by a direct link (browser and API), 404 through the own class, 403 on the editor route');

  // ---- 7. Delete the block from the editor ----
  await lessonRow.getByRole('button', { name: /Рабочий лист/ }).click();
  const edit2 = tp.getByRole('dialog', { name: 'Редактирование PDF-документа' });
  await edit2.getByRole('button', { name: 'Удалить блок' }).click();
  await tp.getByRole('dialog', { name: 'Удалить упражнение?' }).getByRole('button', { name: 'Удалить' }).click();
  await lessonRow.waitFor({ state: 'detached' });
  await block.waitFor({ state: 'detached', timeout: 15000 });
  const gone = await apiS1.call('GET', fileUrl);
  assert(gone.status === 404, `file after delete: ${gone.status}`);
  ok('«Удалить блок» removes it from the lesson; the student\'s block disappears; the file is refused (404)');

  assert(errors.length === 0, `console errors: ${errors.join(' | ')}`);
  assert(external.size === 0, `external requests: ${[...external].join(' ')}`);
  ok('no console errors, no external requests');
  console.log(`teacher: ${users.teacher.email} / ${users.teacher.password}`);
  console.log(`student: ${S1.email} / ${S1.password}; other class: ${S2.email} / ${S2.password}`);
  console.log(`class: ${WEB}/classes/${clsA.id}; lesson: ${WEB}/materials/lessons/${lesson.id}`);
  console.log(`screenshots: ${OUT}`);
} finally {
  await b.close();
}
