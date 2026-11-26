// Typed endpoint functions grouped by domain.
import { API_URL, api, request } from './client';
import type {
  Board,
  ClassBoardView,
  BoardImage,
  BoardItem,
  BoardSummary,
  StudentInviteAccepted, StudentInviteLink, StudentInviteSent,
  Appearance, AppearanceRequest, EffectiveAppearance, ImageSlot, MarathonAppearance,
  Assignment, Attachment, CatalogEntry, Category, ChatMessage, Classroom, Course, Exercise, ExerciseStats, ExerciseView,
  FilterOptions, Gif, Grade, InvitationInfo, InviteResult, MarathonInvitation, HomeworkStatus, HomeworkView, Lesson, LessonAttrs, LessonFilter, ListResponse, MarathonLessonCopied, MarathonLessonFields, MarathonLessonReview, MarathonLessons, MarathonMe, MarathonMode, MarathonPassage, MarathonLessonComment, MarathonLessonResults, MarathonMyResults, MarathonReviewRequest, MarathonStudentsRating, MarathonSaveRequest, MarathonStats, MarathonStatsSettings, MarathonUnlockAfter, MarathonStudent, MarathonStudentProgress, Me, Progress,
  SaveExerciseRequest, Scope, Section, SignupLink, SignupRequested, User,
} from '@/types/api';

export const authApi = {
  me: () => api.get<Me>('/auth/me'),
  login: (email: string, password: string) => api.post<Me>('/auth/login', { email, password }),
  logout: () => api.post<void>('/auth/logout'),
  updateMe: (body: Partial<Pick<Me, 'locale' | 'timezone'>>) => api.patch<Me>('/auth/me', body),
};

/** Private teacher sign-up. The token goes in the body, never in a URL. */
export const signupApi = {
  requestEmail: (body: { email: string; locale: string; accept_terms: boolean }) => api.post<SignupRequested>('/signup/email', body),
  verify: (token: string) => api.post<SignupLink>('/signup/verify', { token }),
  complete: (token: string, password: string) => api.post<Me>('/signup/complete', { token, password }),
};

/** Password reset by e-mail. The token goes in the body, never in a URL. */
export const resetApi = {
  request: (body: { email: string; locale: string }) => api.post<SignupRequested>('/password-reset/request', body),
  verify: (token: string) => api.post<SignupLink>('/password-reset/verify', { token }),
  complete: (token: string, password: string) => api.post<void>('/password-reset/complete', { token, password }),
};

/** Invitations of a private teacher to a student by e-mail. Tokens go in the body, never in a URL. */
export const studentInvitesApi = {
  invite: (body: { email: string; locale: string }) => api.post<StudentInviteSent>('/teacher/invitations', body),
  verify: (token: string) => api.post<StudentInviteLink>('/student-invitations/verify', { token }),
  accept: (token: string) => api.post<StudentInviteAccepted>('/student-invitations/accept', { token }),
  register: (body: { token: string; full_name: string; password: string; accept_terms: boolean }) =>
    api.post<Me>('/student-invitations/register', body),
};

export const adminApi = {
  users: (q: { role?: string; q?: string; limit?: number; offset?: number }) => api.get<ListResponse<User>>('/admin/users', q),
  createUser: (body: { email: string; full_name: string; role: string; password: string }) => api.post<User>('/admin/users', body),
  updateUser: (id: string, body: { full_name?: string; role?: string; disabled?: boolean; password?: string }) => api.patch<User>(`/admin/users/${id}`, body),
  assignments: (q: { limit?: number; offset?: number }) => api.get<ListResponse<Assignment>>('/admin/assignments', q),
  assign: (teacher_id: string, student_id: string) => api.post<void>('/admin/assignments', { teacher_id, student_id }),
  unassign: (teacher_id: string, student_id: string) => request<void>('DELETE', '/admin/assignments', { body: { teacher_id, student_id } }),
};

export const classroomsApi = {
  list: (q: { q?: string; type?: string; sort?: 'created' | 'name'; order?: 'asc' | 'desc'; limit?: number; offset?: number }) =>
    api.get<ListResponse<Classroom>>('/classrooms', q),
  get: (id: string) => api.get<Classroom>(`/classrooms/${id}`),
  create: (body: { name: string; student_id: string; lesson_type: 'individual'; cover_color?: string }) => api.post<Classroom>('/classrooms', body),
  rename: (id: string, name: string) => api.patch<Classroom>(`/classrooms/${id}`, { name }),
  setLesson: (id: string, lesson_id: string) => api.put<Classroom>(`/classrooms/${id}/current-lesson`, { lesson_id }),
  myStudents: () => api.get<{ items: User[] }>('/teacher/students'),
};

export const marathonsApi = {
  languages: () => api.get<{ items: string[] }>('/marathon-languages'),
  create: (body: MarathonSaveRequest) => api.post<Classroom>('/marathons', body),
  update: (id: string, body: MarathonSaveRequest) => api.patch<Classroom>(`/marathons/${id}`, body),
  archive: (id: string) => api.post<void>(`/marathons/${id}/archive`),
  setPassage: (id: string, body: MarathonPassage) => api.patch<Classroom>(`/marathons/${id}/settings`, body),
  me: (id: string) => api.get<MarathonMe>(`/marathons/${id}/me`),
  changeMode: (id: string, body: { mode: MarathonMode; expected_epoch: number; confirm_reset: boolean; confirm_reset_dates?: boolean; interval_days?: number; weekdays?: number[]; open_time?: string }) => api.post<Classroom>(`/marathons/${id}/mode`, body),
  setLessonOpenAt: (id: string, lessonId: string, openAt: string | null) => api.put<MarathonLessons>(`/marathons/${id}/lessons/${lessonId}/open-at`, { open_at: openAt }),
  unlockLesson: (id: string, lessonId: string, studentId?: string) => api.post<MarathonLessons>(`/marathons/${id}/lessons/${lessonId}/unlock`, studentId ? { student_id: studentId } : {}),
  lessons: (id: string) => api.get<MarathonLessons>(`/marathons/${id}/lessons`),
  addLesson: (id: string, lessonId: string) => api.post<MarathonLessons>(`/marathons/${id}/lessons`, { lesson_id: lessonId }),
  createLesson: (id: string, fields: MarathonLessonFields) => api.post<MarathonLessons>(`/marathons/${id}/lessons`, { new: fields }),
  updateLesson: (id: string, lessonId: string, fields: MarathonLessonFields) => api.patch<MarathonLessons>(`/marathons/${id}/lessons/${lessonId}`, fields),
  reorderLessons: (id: string, lessonIds: string[]) => api.put<MarathonLessons>(`/marathons/${id}/lessons/order`, { lesson_ids: lessonIds }),
  removeLesson: (id: string, lessonId: string) => api.del<MarathonLessons>(`/marathons/${id}/lessons/${lessonId}`),
  copyLesson: (id: string, lessonId: string) => api.post<MarathonLessonCopied>(`/marathons/${id}/lessons/${lessonId}/copy`),
  completeLesson: (id: string, lessonId: string) => api.post<MarathonLessons>(`/marathons/${id}/lessons/${lessonId}/complete`),
  submitLesson: (id: string, lessonId: string) => api.post<MarathonLessons>(`/marathons/${id}/lessons/${lessonId}/submit`),
  reviewDecision: (id: string, lessonId: string, body: MarathonReviewRequest) => api.post<MarathonLessonReview>(`/marathons/${id}/lessons/${lessonId}/review`, body),
  invitations: (id: string) => api.get<{ items: MarathonInvitation[] }>(`/marathons/${id}/invitations`),
  invite: (id: string, emails: string[], locale: string) => api.post<{ items: InviteResult[] }>(`/marathons/${id}/invitations`, { emails, locale }),
  resendInvitation: (id: string, iid: string) => api.post<{ items: MarathonInvitation[] }>(`/marathons/${id}/invitations/${iid}/resend`),
  revokeInvitation: (id: string, iid: string) => api.del<{ items: MarathonInvitation[] }>(`/marathons/${id}/invitations/${iid}`),
  invitationInfo: (token: string) => api.get<InvitationInfo>(`/invitations/${token}`),
  acceptInvitation: (token: string) => api.post<{ marathon_id: string }>(`/invitations/${token}/accept`),
  setStatsSettings: (id: string, body: MarathonStatsSettings) => api.put<Classroom>(`/marathons/${id}/stats-settings`, body),
  // Appearance (stage 10): pictures take effect at once; reactions and the background are saved with the settings.
  uploadImage: (id: string, slot: ImageSlot, file: File, lessonId?: string) => {
    const f = new FormData();
    f.append('file', file);
    return api.upload<{ image_id: string }>(`/marathons/${id}${lessonId ? `/lessons/${lessonId}` : ''}/images/${slot}`, f);
  },
  removeImage: (id: string, slot: ImageSlot, lessonId?: string) => api.del<void>(`/marathons/${id}${lessonId ? `/lessons/${lessonId}` : ''}/images/${slot}`),
  appearance: (id: string) => api.get<MarathonAppearance>(`/marathons/${id}/appearance`),
  setAppearance: (id: string, body: AppearanceRequest) => api.put<MarathonAppearance>(`/marathons/${id}/appearance`, body),
  lessonAppearance: (id: string, lessonId: string) => api.get<Appearance>(`/marathons/${id}/lessons/${lessonId}/appearance`),
  setLessonAppearance: (id: string, lessonId: string, body: AppearanceRequest) => api.put<Appearance>(`/marathons/${id}/lessons/${lessonId}/appearance`, body),
  effectiveAppearance: (id: string, lessonId: string) => api.get<EffectiveAppearance>(`/marathons/${id}/lessons/${lessonId}/appearance/effective`),
  stats: (id: string) => api.get<MarathonStats>(`/marathons/${id}/stats`),
  rating: (id: string) => api.get<MarathonStudentsRating>(`/marathons/${id}/rating`),
  lessonResults: (id: string, lessonId: string) => api.get<MarathonLessonResults>(`/marathons/${id}/lessons/${lessonId}/results`),
  setLessonComment: (id: string, lessonId: string, body: string) => api.put<{ comment: MarathonLessonComment | null }>(`/marathons/${id}/lessons/${lessonId}/stats-comment`, { body }),
  myResults: (id: string) => api.get<MarathonMyResults>(`/marathons/${id}/me/results`),
  setReviewSettings: (id: string, body: { teacher_check: boolean; unlock_after: MarathonUnlockAfter }) => api.put<Classroom>(`/marathons/${id}/review-settings`, body),
  students: (id: string) => api.get<{ items: MarathonStudent[] }>(`/marathons/${id}/students`),
  studentProgress: (id: string, sid: string) => api.get<MarathonStudentProgress>(`/marathons/${id}/students/${sid}/progress`),
  reviewLesson: (id: string, sid: string, lid: string) => api.get<MarathonLessonReview>(`/marathons/${id}/students/${sid}/lessons/${lid}`),
  addStudents: (id: string, ids: string[]) => api.post<{ items: MarathonStudent[] }>(`/marathons/${id}/students`, { student_ids: ids }),
  setStudentsActive: (id: string, ids: string[], active: boolean) => api.put<{ items: MarathonStudent[] }>(`/marathons/${id}/students/active`, { student_ids: ids, active }),
  removeStudent: (id: string, sid: string) => api.del<{ items: MarathonStudent[] }>(`/marathons/${id}/students/${sid}`),
};

/** «Материалы» → «Интерактивные доски». */
export const boardsApi = {
  list: (q: string) => api.get<ListResponse<BoardSummary>>('/boards', { q, limit: 100 }),
  create: (title: string) => api.post<Board>('/boards', { title }),
  get: (id: string) => api.get<Board>(`/boards/${id}`),
  save: (id: string, body: { content: { items: BoardItem[] }; version: number; force?: boolean; title?: string }) => api.put<Board>(`/boards/${id}`, body),
  uploadImage: (id: string, file: File) => {
    const f = new FormData();
    f.append('file', file);
    return api.upload<BoardImage>(`/boards/${id}/images`, f);
  },
  /** A picture of the board; served only to the board's owner (the browser sends the session cookie). */
  imageUrl: (id: string, imageId: string) => `${API_URL}/api/boards/${id}/images/${imageId}`,
};

/** The board open in a class lesson: members read it, the teacher of the class opens and closes it. */
export const classBoardApi = {
  get: (cid: string) => api.get<ClassBoardView>(`/classrooms/${cid}/board`),
  open: (cid: string, boardId: string) => api.put<ClassBoardView>(`/classrooms/${cid}/board`, { board_id: boardId }),
  /** «Создать копию для занятия»: a copy of the board is made and opened; the same requestId twice makes one copy. */
  openCopy: (cid: string, boardId: string, requestId: string) =>
    api.put<ClassBoardView>(`/classrooms/${cid}/board`, { board_id: boardId, copy: true, request_id: requestId }),
  close: (cid: string) => request<void>('DELETE', `/classrooms/${cid}/board`),
  allowDrawing: (cid: string, userId: string) => api.put<ClassBoardView>(`/classrooms/${cid}/board/drawers/${userId}`),
  revokeDrawing: (cid: string, userId: string) => request<ClassBoardView>('DELETE', `/classrooms/${cid}/board/drawers/${userId}`),
  /** A student's pen stroke, added as its own action; the same client_id sent again is stored once. */
  stroke: (cid: string, body: { board_id: string; client_id: string; color: string; width: number; points: [number, number][] }) =>
    api.post<{ item_id: string; version: number; duplicate: boolean }>(`/classrooms/${cid}/board/strokes`, body),
  /** A picture of the open board, served to members of the class only. */
  imageUrl: (cid: string, imageId: string) => `${API_URL}/api/classrooms/${cid}/board/images/${imageId}`,
};

export const materialsApi = {
  courses: (f: LessonFilter & { limit?: number; offset?: number }) => api.get<ListResponse<Course>>('/courses', { ...f }),
  course: (id: string, f: LessonFilter = {}) => api.get<Course>(`/courses/${id}`, { ...f }),
  createCourse: (body: { title: string; description?: string; cover_color?: string }) => api.post<Course>('/courses', body),
  updateCourse: (id: string, body: { title?: string; description?: string; cover_color?: string }) => api.patch<Course>(`/courses/${id}`, body),
  reorderCourses: (ids: string[]) => api.put<void>('/courses/order', { ids }),
  createLesson: (courseId: string, title: string) => api.post<Lesson>(`/courses/${courseId}/lessons`, { title }),
  reorderLessons: (courseId: string, ids: string[]) => api.put<void>(`/courses/${courseId}/lessons/order`, { ids }),
  lesson: (id: string) => api.get<Lesson>(`/lessons/${id}`),
  updateLesson: (id: string, body: { title?: string; description?: string; attrs?: LessonAttrs }) => api.patch<Lesson>(`/lessons/${id}`, body),
  createSection: (lessonId: string, title: string) => api.post<Section>(`/lessons/${lessonId}/sections`, { title }),
  updateSection: (id: string, title: string) => api.patch<void>(`/sections/${id}`, { title }),
  reorderSections: (lessonId: string, ids: string[]) => api.put<void>(`/lessons/${lessonId}/sections/order`, { ids }),
  filters: () => api.get<FilterOptions>('/material-filters'),
};

export const exercisesApi = {
  catalog: () => api.get<{ items: CatalogEntry[] }>('/exercise-types'),
  list: (lessonId: string, scope: Scope) => api.get<{ items: Exercise[] }>(`/lessons/${lessonId}/exercises`, { scope }),
  create: (lessonId: string, body: SaveExerciseRequest & { scope: Scope; section_id: string | null; kind: string }) =>
    api.post<Exercise>(`/lessons/${lessonId}/exercises`, body),
  get: (id: string) => api.get<Exercise>(`/exercises/${id}`),
  update: (id: string, body: SaveExerciseRequest) => api.put<Exercise>(`/exercises/${id}`, body),
  setHidden: (id: string, hidden: boolean) => api.patch<void>(`/exercises/${id}/hidden`, { hidden }),
  remove: (id: string) => api.del<void>(`/exercises/${id}`),
  reorder: (lessonId: string, scope: Scope, section_id: string | null, ids: string[]) => api.put<void>(`/lessons/${lessonId}/exercises/order`, { scope, section_id, ids }),
  copyToHomework: (id: string) => api.post<Exercise>(`/exercises/${id}/copy-to-homework`),
  stats: (id: string) => api.get<ExerciseStats>(`/exercises/${id}/stats`),
  categories: () => api.get<{ items: Category[] }>('/score-categories'),
  createCategory: (name: string) => api.post<Category>('/score-categories', { name }),
};

export const attemptsApi = {
  lessonExercises: (cid: string, lid: string, scope: Scope, studentId?: string) =>
    api.get<{ items: ExerciseView[] }>(`/classrooms/${cid}/lessons/${lid}/exercises`, { scope, student_id: studentId }),
  view: (cid: string, eid: string, studentId?: string) => api.get<ExerciseView>(`/classrooms/${cid}/exercises/${eid}`, { student_id: studentId }),
  start: (cid: string, eid: string) => api.post<ExerciseView>(`/classrooms/${cid}/exercises/${eid}/attempt`),
  reset: (cid: string, eid: string, studentId: string) => api.post<ExerciseView>(`/classrooms/${cid}/exercises/${eid}/reset`, { student_id: studentId }),
  /** A test answer sends option ids; a gap answer sends the typed word in `text`. */
  answer: (attemptId: string, questionId: string, optionIds: string[], check: boolean, text = '') =>
    api.put<ExerciseView>(`/attempts/${attemptId}/answers/${questionId}`, { option_ids: optionIds, text, check }),
  finish: (attemptId: string) => api.post<ExerciseView>(`/attempts/${attemptId}/finish`),
};

/** Teacher preview (stage 8): the lesson owner takes the lesson with separate preview attempts. */
export const previewApi = {
  lessonExercises: (lid: string) => api.get<{ items: ExerciseView[] }>(`/preview/lessons/${lid}/exercises`),
  resetLesson: (lid: string) => api.post<{ items: ExerciseView[] }>(`/preview/lessons/${lid}/reset`),
  view: (eid: string) => api.get<ExerciseView>(`/preview/exercises/${eid}`),
  start: (eid: string) => api.post<ExerciseView>(`/preview/exercises/${eid}/attempt`),
  /** A test answer sends option ids; a gap answer sends the typed word in `text`. */
  answer: (attemptId: string, questionId: string, optionIds: string[], check: boolean, text = '') =>
    api.put<ExerciseView>(`/preview/attempts/${attemptId}/answers/${questionId}`, { option_ids: optionIds, text, check }),
  finish: (attemptId: string) => api.post<ExerciseView>(`/preview/attempts/${attemptId}/finish`),
  reset: (attemptId: string) => api.post<ExerciseView>(`/preview/attempts/${attemptId}/reset`),
};

export const homeworkApi = {
  get: (cid: string, lid: string, studentId?: string) => api.get<HomeworkView>(`/classrooms/${cid}/lessons/${lid}/homework`, { student_id: studentId }),
  submit: (cid: string, lid: string) => api.post<HomeworkView>(`/classrooms/${cid}/lessons/${lid}/homework/submit`),
  setStatus: (cid: string, lid: string, body: { student_id: string; to_status: HomeworkStatus; from_status?: HomeworkStatus }) =>
    api.post<HomeworkView>(`/classrooms/${cid}/lessons/${lid}/homework/status`, body),
};

export const gradingApi = {
  progress: (cid: string, lid: string, studentId?: string) => api.get<Progress>(`/classrooms/${cid}/lessons/${lid}/progress`, { student_id: studentId }),
  setSection: (cid: string, sectionId: string, completed: boolean) => api.put<void>(`/classrooms/${cid}/sections/${sectionId}/completion`, { completed }),
  setCompletion: (cid: string, lid: string, completed: boolean, grade?: Grade) => api.put<void>(`/classrooms/${cid}/lessons/${lid}/completion`, { completed, grade }),
  setGrade: (cid: string, lid: string, grade: Grade) => api.put<void>(`/classrooms/${cid}/lessons/${lid}/grade`, { grade }),
};

export const chatApi = {
  list: (cid: string, before?: { at: string; id: string }) =>
    api.get<{ items: ChatMessage[]; has_more: boolean }>(`/classrooms/${cid}/messages`, { before: before?.at, before_id: before?.id, limit: 50 }),
  send: (cid: string, body: { client_id: string; kind: 'text' | 'gif' | 'file'; body?: string; gif_id?: string; attachment_id?: string }) =>
    api.post<ChatMessage>(`/classrooms/${cid}/messages`, body),
  edit: (id: string, body: string) => api.patch<ChatMessage>(`/messages/${id}`, { body }),
  remove: (id: string) => api.del<void>(`/messages/${id}`),
  upload: (cid: string, file: File) => {
    const f = new FormData();
    f.append('file', file);
    return api.upload<Attachment>(`/classrooms/${cid}/attachments`, f);
  },
  gifStatus: () => api.get<{ enabled: boolean }>('/gifs/status'),
  searchGifs: (q: string) => api.get<{ items: Gif[] }>('/gifs/search', { q }),
};
