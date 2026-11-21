-- +goose Up
-- Templates: course -> lesson -> section. Owned by a teacher.
CREATE TABLE courses (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '',
    cover_color text NOT NULL DEFAULT '#1AC6FF',
    position    integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX courses_owner_idx ON courses (owner_id, position);

-- Material parameters (filter fields) live on lessons.
CREATE TABLE lessons (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id    uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    title        text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    description  text NOT NULL DEFAULT '',
    position     integer NOT NULL DEFAULT 0,
    language     text NOT NULL DEFAULT '',
    age          text NOT NULL DEFAULT '',
    level        text NOT NULL DEFAULT '',
    lesson_type  text NOT NULL DEFAULT '',
    duration_min integer,
    skills       text[] NOT NULL DEFAULT '{}',
    grammar      text[] NOT NULL DEFAULT '{}',
    vocabulary   text[] NOT NULL DEFAULT '{}',
    functions    text[] NOT NULL DEFAULT '{}',
    tags         text[] NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lessons_course_idx ON lessons (course_id, position);

CREATE TABLE sections (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id  uuid NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    title      text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    position   integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sections_lesson_idx ON sections (lesson_id, position);

-- Classrooms. Members table allows group classes later.
CREATE TABLE classrooms (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name              text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    lesson_type       text NOT NULL DEFAULT 'individual' CHECK (lesson_type IN ('individual', 'group')),
    owner_id          uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    cover_color       text NOT NULL DEFAULT '#1AC6FF',
    current_lesson_id uuid REFERENCES lessons(id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX classrooms_owner_idx ON classrooms (owner_id);

CREATE TABLE classroom_members (
    classroom_id uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         text NOT NULL CHECK (role IN ('teacher', 'student')),
    joined_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, user_id)
);
CREATE INDEX classroom_members_user_idx ON classroom_members (user_id);

-- Which courses are available to a classroom.
CREATE TABLE classroom_courses (
    classroom_id uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    course_id    uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    added_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, course_id)
);
CREATE INDEX classroom_courses_course_idx ON classroom_courses (course_id);

-- A lesson as passed in a particular classroom (completion + grade live here, not on the template).
CREATE TABLE classroom_lessons (
    classroom_id uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    lesson_id    uuid NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    opened_at    timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    completed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    grade        text CHECK (grade IN ('fine', 'good', 'amazing')),
    graded_at    timestamptz,
    graded_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (classroom_id, lesson_id),
    CHECK ((completed_at IS NULL) = (grade IS NULL))
);
CREATE INDEX classroom_lessons_lesson_idx ON classroom_lessons (lesson_id);

CREATE TABLE classroom_lesson_events (
    id           bigserial PRIMARY KEY,
    classroom_id uuid NOT NULL,
    lesson_id    uuid NOT NULL,
    actor_id     uuid REFERENCES users(id) ON DELETE SET NULL,
    event        text NOT NULL CHECK (event IN ('completed', 'uncompleted', 'grade_changed')),
    grade        text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (classroom_id, lesson_id) REFERENCES classroom_lessons(classroom_id, lesson_id) ON DELETE CASCADE
);
CREATE INDEX classroom_lesson_events_idx ON classroom_lesson_events (classroom_id, lesson_id, created_at);

CREATE TABLE section_completions (
    classroom_id uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    section_id   uuid NOT NULL REFERENCES sections(id) ON DELETE CASCADE,
    completed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    completed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, section_id)
);

-- +goose Down
DROP TABLE section_completions;
DROP TABLE classroom_lesson_events;
DROP TABLE classroom_lessons;
DROP TABLE classroom_courses;
DROP TABLE classroom_members;
DROP TABLE classrooms;
DROP TABLE sections;
DROP TABLE lessons;
DROP TABLE courses;
