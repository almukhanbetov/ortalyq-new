// DTOs mirrored from the Go API. One source of truth for the frontend.

/** private_teacher: registered on their own (sign-up); teacher cabinet without school-assigned students. */
export type Role = 'admin' | 'teacher' | 'private_teacher' | 'student';
export type Locale = 'ru' | 'kk' | 'en';

export interface Me {
  id: string;
  email: string;
  full_name: string;
  role: Role;
  locale: Locale;
  timezone: string;
  avatar_color: string;
}

export interface ListResponse<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface User {
  id: string;
  email: string;
  full_name: string;
  role: Role;
  avatar_color: string;
  disabled_at: string | null;
  created_at: string;
}

export interface Assignment {
  teacher: User;
  student: User;
  created_at: string;
}

export interface Member {
  user_id: string;
  full_name: string;
  role: 'teacher' | 'student';
  avatar_color: string;
}

export interface Classroom {
  id: string;
  name: string;
  lesson_type: 'individual' | 'group' | 'marathon';
  owner_id: string;
  cover_color: string;
  /** Marathon cover picture (stage 10): an attachment of the class; null = generated cover. */
  cover_image_id?: string | null;
  my_role: 'teacher' | 'student';
  current_lesson: { lesson_id: string; lesson_title: string; course_id: string; course_title: string } | null;
  members: Member[];
  homework_badge: number;
  lessons_count: number;
  students_count: number;
  /** Present only for lesson_type = 'marathon'. */
  marathon: MarathonInfo | null;
  created_at: string;
}

export type MarathonDirection = 'languages' | 'other';
export type MarathonMode = 'open' | 'interval' | 'progressive' | 'closed' | 'weekdays';

export interface MarathonInfo {
  direction: MarathonDirection;
  target_language: string | null;
  mode: MarathonMode;
  /** IANA time zone of the marathon (owner's zone at creation); dates are shown in it. */
  timezone: string;
  /** After this moment students can no longer start or continue lessons. */
  end_at: string | null;
  /** Computed by the server with now() of the DB. */
  finished: boolean;
  server_now: string;
  /** Delayed start: before it students cannot open lessons. started is computed by the server. */
  start_at: string | null;
  started: boolean;
  /** Each student loses access this many days after their activation date; null = off. */
  auto_deactivate_days: number | null;
  /** Lesson statuses count in this epoch; a mode change starts a new one (sent back as expected_epoch). */
  progress_epoch: number;
  /** Schedule of "По интервалам" / "По дням недели" (1 = Monday … 7 = Sunday), opening hour "HH:00". */
  interval_days: number | null;
  weekdays: number[];
  open_time: string | null;
  /** "Проверка учителем": lessons are sent to the teacher instead of being completed by the student. */
  teacher_check: boolean;
  /** "Открытие уроков" of the progressive mode with the check on: after acceptance or after sending. */
  unlock_after: MarathonUnlockAfter;
  /** Statistics and rating (stage 7). rating_limit: 3 / 5 / 10 places shown to students; null = all. */
  stats_enabled: boolean;
  rating_enabled: boolean;
  rating_limit: 3 | 5 | 10 | null;
  rating_show_place: boolean;
}

export type MarathonUnlockAfter = 'teacher_check' | 'submission';
/** Review status of a lesson for one student in the current epoch. */
export type MarathonLessonStatus = 'none' | 'submitted' | 'revision' | 'completed';
export type MarathonRating = 'not_bad' | 'nice' | 'amazing';

/** Lesson rule of the passage mode (backend LessonAccessAt). */
export interface MarathonLessonLock {
  open: boolean;
  reason?: 'opens_at' | 'previous_not_completed' | 'previous_not_reviewed' | 'previous_not_submitted' | 'not_unlocked' | 'scheduled' | 'mode_unavailable';
  opens_at?: string;
}

/** The server's access decision for one student (backend access.go). */
export interface MarathonAccess {
  status: 'open' | 'not_started' | 'deactivated';
  reason: MarathonInactiveReason | null;
  opens_at: string | null;
  deactivates_at: string | null;
  deactivation_reason: MarathonInactiveReason | null;
}

/** GET /marathons/:id/me — the student's own view. */
export interface MarathonMe {
  access: MarathonAccess;
  mode: MarathonMode;
  server_now: string;
  /** access "locked": the student may work, but the passage mode keeps this lesson closed (see lock). */
  items: (MarathonLesson & { access: MarathonAccess['status'] | 'locked'; lock?: MarathonLessonLock })[];
  completed_lessons: number;
  total_lessons: number;
}

export interface MarathonPassage {
  start_at: string | null;
  end_at: string | null;
  auto_deactivate_days: number | null;
}

export interface MarathonLesson {
  lesson_id: string;
  title: string;
  description: string;
  course_title: string;
  cover_color: string;
  /** Lesson cover picture in this marathon (stage 10); null = generated cover. */
  cover_image_id: string | null;
  position: number;
  section_count: number;
  /** Teacher: how many students completed the lesson. */
  completed_count: number;
  /** Teacher: how many participants sent the lesson and wait for the review. */
  pending_count: number;
  /** Student: whether the caller completed it. */
  completed: boolean;
  completed_at: string | null;
  /** Review state of the student the list is built for (stage 6). accepted: completed by the teacher. */
  status: MarathonLessonStatus;
  submitted_at: string | null;
  accepted: boolean;
  /** Shown only while the lesson is accepted. */
  rating: MarathonRating | null;
  teacher_comment: string | null;
  /** Teacher only: the review kept after «Отменить завершение» (prefills the rating window). */
  previous_rating?: MarathonRating | null;
  previous_comment?: string | null;
  /** "Дата открытия" of the lesson (open mode). */
  open_at: string | null;
  /** Closed mode: opened for every student (teacher view). */
  unlocked_for_all: boolean;
  /** "Прогресс ученика": the lesson rule for that student. */
  student_lesson?: MarathonLessonLock;
}

export interface MarathonLessons {
  items: MarathonLesson[];
  completed_lessons: number;
  total_lessons: number;
}

/** "Настройки урока": title and description belong to the lesson template, the cover to this marathon. */
export interface MarathonLessonFields {
  title?: string;
  description?: string;
  /** Empty string restores the course colour. */
  cover_color?: string;
}

export interface MarathonLessonCopied extends MarathonLessons {
  lesson_id: string;
}

export interface MarathonStudent {
  id: string;
  full_name: string;
  email: string;
  avatar_color: string;
  activated_at: string;
  status: 'active' | 'inactive';
  inactive_reason: MarathonInactiveReason | null;
  /** When and why the student loses (or lost) access: the end date, or the moment of manual deactivation. */
  deactivates_at: string | null;
  deactivation_reason: MarathonInactiveReason | null;
  completed_lessons: number;
  total_lessons: number;
  /** Lessons the student sent and the teacher has not reviewed yet. */
  pending_lessons: number;
  /** Manual state: set while the teacher has deactivated the student. */
  /** The same access decision the server applies to the student. */
  access_status: MarathonAccess['status'];
  deactivated_at: string | null;
  deactivated_by_name: string | null;
  reactivated_at: string | null;
}

/** end_date: the marathon is over; manual: the teacher deactivated the student (wins over the end date). */
export type MarathonInactiveReason = 'end_date' | 'manual' | 'auto_deactivation';

/** "Прогресс ученика": the student and their status per lesson. */
export interface MarathonStudentProgress extends MarathonLessons {
  student: MarathonStudent;
}

/** Teacher's read-only view of a lesson as the student left it. */
export interface MarathonLessonReview {
  student: MarathonStudent;
  lesson_id: string;
  lesson_title: string;
  course_title: string;
  completed: boolean;
  completed_at: string | null;
  sections: Section[];
  exercises: ExerciseView[];
  points: { earned: number; max: number; exercises: number; finished: number; categories: { category_id: string | null; category_name: string; earned: number; max: number }[] };
  /** The lesson with the student's review status (teacher view). */
  lesson: MarathonLesson;
  teacher_check: boolean;
  submissions_count: number;
  /** Answer to a decision: the lesson already had that status, nothing was saved (repeated decisions are idempotent). */
  unchanged?: boolean;
}

/** POST /marathons/:id/lessons/:lid/review — the teacher's decision. */
export interface MarathonReviewRequest {
  student_id: string;
  to_status: 'revision' | 'completed' | 'submitted';
  from_status?: MarathonLessonStatus;
  rating?: MarathonRating;
  comment?: string;
}

export interface MarathonSaveRequest {
  name: string;
  cover_color: string;
  direction: MarathonDirection;
  target_language: string | null;
}

export interface LessonAttrs {
  language: string;
  age: string;
  level: string;
  lesson_type: string;
  duration_min: number | null;
  skills: string[];
  grammar: string[];
  vocabulary: string[];
  functions: string[];
  tags: string[];
}

export interface Section {
  id: string;
  lesson_id: string;
  title: string;
  position: number;
}

export interface Lesson extends LessonAttrs {
  id: string;
  course_id: string;
  title: string;
  description: string;
  position: number;
  section_count: number;
  sections?: Section[];
  updated_at: string;
}

export interface Course {
  id: string;
  owner_id: string;
  title: string;
  description: string;
  cover_color: string;
  position: number;
  lesson_count: number;
  lessons?: Lesson[];
  created_at: string;
  updated_at: string;
}

export interface FilterOptions {
  language: string[];
  age: string[];
  level: string[];
  lesson_type: string[];
  skills: string[];
  duration: number[];
  grammar: string[];
  vocabulary: string[];
  functions: string[];
  tags: string[];
}

export interface LessonFilter {
  q?: string;
  language?: string;
  age?: string;
  level?: string;
  lesson_type?: string;
  skill?: string;
  max_duration?: string;
  grammar?: string;
  vocabulary?: string;
  function?: string;
  tag?: string;
}

export type CheckMode = 'instant' | 'after';

export interface ExerciseSettings {
  check_mode: CheckMode;
  timer_enabled: boolean;
  timer_seconds: number | null;
  scoring_enabled: boolean;
  points_per_correct: number;
  category_id: string | null;
  category_name?: string;
}

export interface Option {
  id: string;
  label: string;
  is_correct?: boolean;
}

export interface Question {
  id: string;
  prompt: string;
  multiple: boolean;
  options: Option[];
}

export interface ExerciseVersion {
  id: string;
  exercise_id: string;
  number: number;
  title: string;
  settings: ExerciseSettings;
  questions: Question[];
  created_at: string;
}

export type Scope = 'lesson' | 'homework';

export interface Exercise {
  id: string;
  lesson_id: string;
  scope: Scope;
  section_id: string | null;
  kind: string;
  position: number;
  hidden: boolean;
  copied_from_id: string | null;
  version_id: string;
  version: ExerciseVersion;
  updated_at: string;
}

export interface CatalogEntry {
  kind: string;
  group: string;
  available: boolean;
}

export interface Category {
  id: string;
  name: string;
  color: string;
  system: boolean;
}

export interface SaveExerciseRequest {
  title: string;
  settings: ExerciseSettings;
  questions: { prompt: string; multiple: boolean; options: { label: string; is_correct: boolean }[] }[];
  hidden?: boolean;
}

export interface ExerciseStats {
  version_number: number;
  attempts: number;
  finished: number;
  questions: { question_id: string; prompt: string; correct: number; wrong: number; unanswered: number }[];
}

export type AnswerResult = 'correct' | 'wrong' | 'unanswered';

export interface AnswerView {
  question_id: string;
  selected: string[];
  checked: boolean;
  result: AnswerResult | null;
  correct_option_ids: string[] | null;
}

export interface AttemptSummary {
  elapsed_seconds: number;
  correct: number;
  wrong: number;
  unanswered: number;
  points_earned: number;
  points_max: number;
}

export interface AttemptView {
  id: string;
  status: 'in_progress' | 'finished' | 'reset';
  started_at: string;
  deadline_at: string | null;
  finished_at: string | null;
  finish_reason: 'manual' | 'timeout' | 'homework_submit' | null;
  server_now: string;
  answers: AnswerView[];
  summary: AttemptSummary | null;
  revealed: boolean;
}

export interface ExerciseView {
  exercise_id: string;
  scope: Scope;
  section_id: string | null;
  kind: string;
  position: number;
  hidden: boolean;
  version: ExerciseVersion;
  attempt: AttemptView | null;
  locked: boolean;
  reset_count: number;
}

export type HomeworkStatus = 'not_submitted' | 'pending' | 'revision' | 'done';

export interface SnapshotExercise {
  exercise_id: string;
  version_id: string;
  title: string;
  version: ExerciseVersion;
  attempt_id: string | null;
  answers: AnswerView[];
  summary: AttemptSummary;
}

export interface HomeworkView {
  student_id: string;
  status: HomeworkStatus;
  status_changed_at: string | null;
  has_exercises: boolean;
  submissions_count: number;
  events: { from_status: HomeworkStatus; to_status: HomeworkStatus; actor_id: string | null; actor_name: string; created_at: string }[];
  latest_submission: { id: string; number: number; submitted_at: string; snapshot: SnapshotExercise[] } | null;
}

export type Grade = 'fine' | 'good' | 'amazing';

export interface Points {
  earned: number;
  max: number;
  exercises: number;
  finished: number;
  categories: { category_id: string | null; category_name: string; earned: number; max: number }[];
}

export interface Progress {
  lesson_id: string;
  lesson_title: string;
  course_title: string;
  student_id: string;
  sections: { id: string; title: string; position: number; completed: boolean; completed_at: string | null }[];
  completion: { completed: boolean; completed_at: string | null; completed_by_name: string; grade: Grade | null; graded_at: string | null };
  lesson_points: Points;
  homework_points: Points;
}

export interface Gif {
  id: string;
  title: string;
  url: string;
  preview_url: string;
  width: number;
  height: number;
}

export interface Attachment {
  id: string;
  classroom_id: string;
  name: string;
  mime_type: string;
  size_bytes: number;
  created_at: string;
}

export interface ChatMessage {
  id: string;
  classroom_id: string;
  client_id: string;
  author: { id: string; full_name: string; avatar_color: string; role: string };
  kind: 'text' | 'gif' | 'file';
  body: string;
  gif: Gif | null;
  attachment: { id: string; name: string; mime_type: string; size_bytes: number } | null;
  created_at: string;
  edited_at: string | null;
}

export interface RealtimeEvent {
  type: string;
  classroom_id: string;
  payload: unknown;
  at: string;
}

export interface CategoryPoints { category_id: string | null; category_name: string; earned: number; max: number }

/** A student's result over lessons, recomputed by the server from answers. */
export interface MarathonSummary { earned: number; max: number; exercises: number; finished_exercises: number; categories: CategoryPoints[] }

/** One participant in the "Статистика" tab and the export (teacher only). place 0: not rated (deactivated). */
export interface MarathonStatsRow {
  student_id: string;
  full_name: string;
  email: string;
  avatar_color: string;
  status: 'active' | 'inactive';
  inactive_reason: MarathonInactiveReason | null;
  completed_lessons: number;
  total_lessons: number;
  completed: boolean;
  points: MarathonSummary;
  place: number;
}

export interface MarathonStats {
  items: MarathonStatsRow[];
  categories: { category_id: string | null; category_name: string }[];
  total_lessons: number;
}

/** A rating row: names and points only (other students see it). */
export interface MarathonRatingEntry { student_id: string; full_name: string; avatar_color: string; points: number; place: number; me: boolean }

export interface MarathonStudentsRating {
  items: MarathonRatingEntry[];
  /** The student's own row when they are outside the shown places and "Место ученика: Показывать". */
  me: MarathonRatingEntry | null;
  limit: 3 | 5 | 10 | null;
  show_place: boolean;
  total: number;
}

export interface MarathonLessonComment { body: string; updated_at: string; updated_by_name: string | null }

/** «Результаты урока». Student: points (own), comment, rating (when on). Teacher: comment, students, full rating. */
export interface MarathonLessonResults {
  lesson_id: string;
  title: string;
  points?: { earned: number; max: number; exercises: number; finished: number; categories: CategoryPoints[] };
  comment: MarathonLessonComment | null;
  rating: MarathonStudentsRating | null;
  students: MarathonStatsRow[] | null;
}

/** «Успехи ученика в марафоне». */
export interface MarathonMyResults {
  completed_lessons: number;
  total_lessons: number;
  finished_exercises: number;
  exercises: number;
  stats_enabled: boolean;
  points: MarathonSummary | null;
  rating: MarathonStudentsRating | null;
}

export interface MarathonStatsSettings { stats_enabled: boolean; rating_enabled: boolean; rating_limit: 3 | 5 | 10 | null; rating_show_place: boolean }

/** Invitation by e-mail (stage 9). status: pending / expired / accepted / revoked / participant; delivery of the last letter. */
export interface MarathonInvitation {
  id: string;
  email: string;
  student_id: string;
  student_name: string;
  avatar_color: string;
  locale: 'ru' | 'kk' | 'en';
  status: 'pending' | 'expired' | 'accepted' | 'revoked' | 'participant';
  delivery: 'queued' | 'sent' | 'failed';
  sends: number;
  created_at: string;
  sent_at: string;
  expires_at: string;
  accepted_at: string | null;
  revoked_at: string | null;
}

export type InviteResultCode = 'invited' | 'renewed' | 'already_invited' | 'already_participant' | 'unavailable' | 'invalid';
export interface InviteResult { email: string; result: InviteResultCode; invitation_id?: string }

/** Public page of an invitation link. */
export interface InvitationInfo { marathon_name: string; teacher_name: string; email_masked: string; status: 'pending' | 'expired' | 'accepted' | 'revoked'; expires_at: string }

// ---- Marathon appearance (stage 10) ----

export type CorrectIcon = 'party_face' | 'party_popper' | 'muscle' | 'fire' | 'brain' | 'check';
export type IncorrectIcon = 'sad' | 'grimace' | 'see_no_evil' | 'thinking' | 'confused' | 'cross';
export type MarathonBackground = 'plain' | 'sunrise' | 'waves' | 'petals' | 'leaves' | 'autumn' | 'custom';
export type ImageSlot = 'cover' | 'background';

export interface IconReaction<T extends string = string> { enabled: boolean; icons: T[] }
export interface SoundReaction { enabled: boolean; sound: number }
export interface Reactions {
  correct_icon: IconReaction<CorrectIcon>;
  incorrect_icon: IconReaction<IncorrectIcon>;
  correct_sound: SoundReaction;
  incorrect_sound: SoundReaction;
}

/** Appearance of the marathon or the own appearance of a lesson in it (enabled = «Своё оформление урока»). */
export interface Appearance {
  enabled: boolean;
  reactions_enabled: boolean;
  reactions: Reactions;
  background_enabled: boolean;
  background: MarathonBackground;
  background_image_id: string | null;
}

export interface MarathonAppearance {
  lesson_appearance: boolean;
  appearance: Appearance;
}

export interface AppearanceRequest {
  lesson_appearance?: boolean;
  enabled?: boolean;
  reactions_enabled: boolean;
  reactions: Reactions;
  background_enabled: boolean;
  background: MarathonBackground;
}

/** What the lesson page applies; parts switched off are null. */
export interface EffectiveAppearance {
  source: 'marathon' | 'lesson';
  reactions: Reactions | null;
  background: { preset: MarathonBackground; image_id: string | null } | null;
}

// Private teacher sign-up.
export interface SignupRequested { resend_after: number }
export interface SignupLink { email: string; expires_at: string }
