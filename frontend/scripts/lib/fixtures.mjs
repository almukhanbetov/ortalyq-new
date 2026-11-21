// Shared helpers of the browser checks that create their own data.
//
// Every check that uses these helpers WRITES to the database, so it refuses to run without an explicit isolated
// stack (WEB_URL and API_URL, never the demo ports 3448/8448). Users, classes and marathons get a unique run suffix:
// a check never depends on seed data left by other checks, on their order or on what is already in the database.
import { request } from 'playwright-core';

export function isolatedStack() {
  const WEB = (process.env.WEB_URL || '').replace(/\/$/, '');
  const API = (process.env.API_URL || '').replace(/\/$/, '');
  if (!WEB || !API) throw new Error('set WEB_URL and API_URL of an isolated stack (this check creates data)');
  if (/:3448$/.test(WEB) || /:8448$/.test(API) || process.env.E2E_DB === 'ortalyq_new') throw new Error('refusing to run against the demo stack');
  return { WEB, API };
}

export const runId = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 5);

/** An API client signed in as the given user (cookies of its own request context). */
export async function apiAs({ API, WEB }, email, password) {
  const ctx = await request.newContext({ baseURL: API, extraHTTPHeaders: { Origin: WEB } });
  const login = await ctx.post('/api/auth/login', { data: { email, password } });
  if (login.status() !== 200) throw new Error(`login ${email}: ${login.status()}`);
  const call = async (method, path, body) => {
    const r = await ctx.fetch('/api' + path, { method, data: body });
    const text = await r.text();
    let json = null;
    try { json = text ? JSON.parse(text) : null; } catch { json = text; }
    return { status: r.status(), body: json };
  };
  const must = async (method, path, body, want) => {
    const r = await call(method, path, body);
    if (r.status !== want) throw new Error(`${method} ${path}: want ${want}, got ${r.status} ${JSON.stringify(r.body)}`);
    return r.body;
  };
  return { call, must, dispose: () => ctx.dispose() };
}

/**
 * Creates a teacher and students of their own (through the administrator of the isolated stack) and assigns the
 * students to the teacher. Names are given; e-mails and passwords are unique to the run.
 */
export async function makeUsers(stack, run, { teacher = 'Учитель', students = ['Данияр'] } = {}) {
  const admin = await apiAs(stack, process.env.ADMIN_EMAIL || 'admin@ortalyq.local', process.env.ADMIN_PASSWORD || 'admin-local-123');
  const password = `pass-${run}`;
  const make = async (role, name, i) => {
    const email = `${role}${i}-${run}@e2e.test`;
    const u = await admin.must('POST', '/admin/users', { email, full_name: `${name} ${run}`, role, password }, 201);
    return { id: u.id, email, password, name: `${name} ${run}` };
  };
  const t = await make('teacher', teacher, 0);
  const s = [];
  for (const [i, name] of students.entries()) {
    const st = await make('student', name, i + 1);
    await admin.must('POST', '/admin/assignments', { teacher_id: t.id, student_id: st.id }, 204);
    s.push(st);
  }
  await admin.dispose();
  return { teacher: t, students: s };
}

/** Escapes a string for use inside a RegExp (e.g. the web address in a letter). */
export const reEscape = (s) => s.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&');
