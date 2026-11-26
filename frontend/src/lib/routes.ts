// App routes. The classes list mirrors the original cabinet path on our own domain.
export const CLASSES_ROUTE = '/cabinet/school/classes';

export type ClassTypeFilter = '' | 'individual' | 'group' | 'marathon';

/**
 * `type` query codes of the original: no type = all, 1 = individual, 0 = group, 2 = marathons.
 * `group` is the old readable value we used before; it still opens group classes and is rewritten to `0`.
 */
const TYPE_TO_PARAM: Record<Exclude<ClassTypeFilter, ''>, string> = { individual: '1', group: '0', marathon: '2' };
const PARAM_TO_TYPE: Record<string, ClassTypeFilter> = { '1': 'individual', '0': 'group', '2': 'marathon', group: 'group' };

export function typeFromParam(v: string | null): ClassTypeFilter {
  return (v && PARAM_TO_TYPE[v]) || '';
}

export function typeToParam(t: ClassTypeFilter): string | null {
  return t ? TYPE_TO_PARAM[t] : null;
}

export function classesUrl(params: { type?: ClassTypeFilter; q?: string } = {}) {
  const sp = new URLSearchParams();
  const tp = typeToParam(params.type ?? '');
  if (tp) sp.set('type', tp);
  if (params.q) sp.set('q', params.q);
  const s = sp.toString();
  return s ? `${CLASSES_ROUTE}?${s}` : CLASSES_ROUTE;
}

/** Marathon page tabs; the path mirrors the original cabinet on our own domain. */
export const MARATHONS_ROUTE = '/cabinet/school/marathons/marathon';
export const MARATHON_TABS = ['students', 'lessons', 'statistics'] as const;
export type MarathonTab = (typeof MARATHON_TABS)[number];

export function marathonUrl(id: string, tab: MarathonTab = 'students') {
  return `${MARATHONS_ROUTE}/${id}/${tab}`;
}

/** "Ученики" tab with the progress window of one student open (kept in the address). */
/** The marathon page right after creation: it opens «Настройки марафона» with «Настроить позже». */
export function marathonSetupUrl(id: string) {
  return `${marathonUrl(id)}?setup=1`;
}

export function marathonStudentUrl(id: string, studentId: string) {
  return `${marathonUrl(id, 'students')}?student=${encodeURIComponent(studentId)}`;
}

/** Owner's read-only view of a student's lesson. */
export function marathonReviewUrl(id: string, lessonId: string, studentId: string) {
  return `${marathonLessonUrl(id, lessonId)}?student=${encodeURIComponent(studentId)}`;
}

/** «Успехи ученика в марафоне»: the student's own results page. */
export function marathonResultsUrl(id: string) {
  return `${MARATHONS_ROUTE}/${id}/results`;
}

/** «Режим предпросмотра»: the owner takes a marathon lesson as a student, with separate preview attempts. */
export function marathonPreviewUrl(id: string, lessonId: string) {
  return `${MARATHONS_ROUTE}/${id}/preview/${lessonId}`;
}

/** Student lesson player inside a marathon. */
export function marathonLessonUrl(id: string, lessonId: string) {
  return `${MARATHONS_ROUTE}/${id}/lesson/${lessonId}`;
}

/** «Материалы» → «Личные материалы»: the teacher's own materials (courses). The search is kept in the address (?q=…). */
export const MATERIALS_ROUTE = '/materials';

export function materialsUrl(params: { q?: string } = {}) {
  return params.q ? `${MATERIALS_ROUTE}?q=${encodeURIComponent(params.q)}` : MATERIALS_ROUTE;
}

/** One material with its lessons. */
export function materialUrl(id: string) {
  return `${MATERIALS_ROUTE}/courses/${id}`;
}

/** A PDF document of «Личные материалы» (the viewer). */
export function documentUrl(id: string) {
  return `${MATERIALS_ROUTE}/documents/${id}`;
}

/** Lesson editor in materials. With a marathon id the editor offers "back to the marathon" instead of the course. */
export function lessonEditorUrl(lessonId: string, marathonId?: string) {
  const base = `/materials/lessons/${lessonId}`;
  return marathonId ? `${base}?marathon=${encodeURIComponent(marathonId)}` : base;
}

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** The marathon to return to from the lesson editor, taken from the address; anything else is ignored. */
export function marathonFromParam(v: string | null): string | null {
  return v && UUID_RE.test(v) ? v : null;
}

/** Where a class card leads: marathons have their own page, other classes open the classroom. */
export function classUrl(c: { id: string; lesson_type: string }) {
  return c.lesson_type === 'marathon' ? marathonUrl(c.id) : `/classes/${c.id}`;
}

/** «Материалы» → «Каталог»: ready-made author courses to add to «Личные материалы». The search is kept in the address. */
export const MATERIALS_CATALOG_ROUTE = '/materials/catalog';

export function materialsCatalogUrl(params: { q?: string } = {}) {
  return params.q ? `${MATERIALS_CATALOG_ROUTE}?q=${encodeURIComponent(params.q)}` : MATERIALS_CATALOG_ROUTE;
}

/** «Материалы» → «Интерактивные доски»: the teacher's personal boards. The search is kept in the address. */
export const MATERIALS_BOARDS_ROUTE = '/materials/boards';

export function boardsUrl(params: { q?: string } = {}) {
  return params.q ? `${MATERIALS_BOARDS_ROUTE}?q=${encodeURIComponent(params.q)}` : MATERIALS_BOARDS_ROUTE;
}

/** The board editor. */
export function boardUrl(id: string) {
  return `${MATERIALS_BOARDS_ROUTE}/${id}`;
}
