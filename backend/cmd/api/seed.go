package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/content"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
)

// seed creates demo accounts and template materials. It refuses to run outside
// APP_ENV=local and takes passwords only from environment variables.
func seed(cfg config.Config) error {
	if !cfg.IsLocal() {
		return fmt.Errorf("seed is only allowed with APP_ENV=local")
	}
	pw := map[string]string{
		"admin":   os.Getenv("SEED_ADMIN_PASSWORD"),
		"teacher": os.Getenv("SEED_TEACHER_PASSWORD"),
		"student": os.Getenv("SEED_STUDENT_PASSWORD"),
	}
	for k, v := range pw {
		if msg := auth.ValidatePassword(v); msg != "" {
			return fmt.Errorf("SEED_%s_PASSWORD: %s", k, msg)
		}
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	type acc struct{ email, name, role, color string }
	accounts := []acc{
		{"admin@ortalyq.local", "Администратор", "admin", "#2F80ED"},
		{"teacher@ortalyq.local", "Айгерим Сапарова", "teacher", "#FF7A45"},
		{"student@ortalyq.local", "Данияр Нурланов", "student", "#9254DE"},
		{"student2@ortalyq.local", "Алия Ермекова", "student", "#16C784"},
		{"teacher2@ortalyq.local", "Олжас Ахметов", "teacher", "#F759AB"},
	}
	ids := map[string]uuid.UUID{}
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		for _, a := range accounts {
			p := pw[a.role]
			hash, err := auth.HashPassword(p)
			if err != nil {
				return err
			}
			var id uuid.UUID
			if err := tx.QueryRow(ctx, `INSERT INTO users (email, full_name, role, password_hash, avatar_color)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (lower(email)) DO UPDATE SET password_hash = EXCLUDED.password_hash
				RETURNING id`, a.email, a.name, a.role, hash, a.color).Scan(&id); err != nil {
				return err
			}
			ids[a.email] = id
		}
		t, s, s2, t2 := ids["teacher@ortalyq.local"], ids["student@ortalyq.local"], ids["student2@ortalyq.local"], ids["teacher2@ortalyq.local"]
		admin := ids["admin@ortalyq.local"]
		for _, pair := range [][2]uuid.UUID{{t, s}, {t, s2}, {t2, s2}} {
			if _, err := tx.Exec(ctx, `INSERT INTO teacher_students (teacher_id, student_id, assigned_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, pair[0], pair[1], admin); err != nil {
				return err
			}
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM courses WHERE owner_id = $1)`, t).Scan(&exists); err != nil {
			return err
		}
		if exists {
			fmt.Println("demo materials already present; accounts refreshed")
			return nil
		}
		// Two individual classes for the main teacher, one for the second teacher.
		mkClass := func(owner, student uuid.UUID, name, color string) error {
			var cid uuid.UUID
			if err := tx.QueryRow(ctx, `INSERT INTO classrooms (name, owner_id, cover_color) VALUES ($1, $2, $3) RETURNING id`, name, owner, color).Scan(&cid); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO classroom_members (classroom_id, user_id, role) VALUES ($1, $2, 'teacher'), ($1, $3, 'student')`, cid, owner, student)
			return err
		}
		if err := mkClass(t, s, "Английский — Данияр", "#1AC6FF"); err != nil {
			return err
		}
		if err := mkClass(t, s2, "Английский — Алия", "#FF7A45"); err != nil {
			return err
		}
		if err := mkClass(t2, s2, "Grammar club", "#9254DE"); err != nil {
			return err
		}
		var course uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO courses (owner_id, title, description, cover_color) VALUES ($1, 'English Starter A1',
			'Базовый курс английского языка для начинающих: глагол to be, приветствия, числа.', '#1AC6FF') RETURNING id`, t).Scan(&course); err != nil {
			return err
		}
		lessons := []struct {
			title, level           string
			skills, grammar, vocab []string
			sections               []string
		}{
			{"Lesson 1. Hello!", "A1", []string{"Speaking", "Listening"}, []string{"to be"}, []string{"Greetings"}, []string{"Warm-up", "Grammar: to be", "Practice"}},
			{"Lesson 2. My family", "A1", []string{"Reading"}, []string{"Possessive adjectives"}, []string{"Family"}, []string{"Warm-up", "Vocabulary", "Reading"}},
			{"Lesson 3. Numbers", "A1", []string{"Listening"}, []string{"Plural nouns"}, []string{"Numbers"}, []string{"Numbers 1-20", "Practice"}},
		}
		for i, l := range lessons {
			var lid uuid.UUID
			if err := tx.QueryRow(ctx, `INSERT INTO lessons (course_id, title, position, language, age, level, lesson_type, duration_min, skills, grammar, vocabulary, functions, tags)
				VALUES ($1, $2, $3, 'English', 'Adults', $4, 'General', 60, $5, $6, $7, '{"Introductions"}', '{"Starter"}') RETURNING id`,
				course, l.title, i, l.level, l.skills, l.grammar, l.vocab).Scan(&lid); err != nil {
				return err
			}
			for j, sec := range l.sections {
				if _, err := tx.Exec(ctx, `INSERT INTO sections (lesson_id, title, position) VALUES ($1, $2, $3)`, lid, sec, j); err != nil {
					return err
				}
			}
		}
		var course2 uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO courses (owner_id, title, description, cover_color, position) VALUES ($1, 'Kazakh for travellers',
			'Короткий курс казахского для поездок.', '#16C784', 1) RETURNING id`, t).Scan(&course2); err != nil {
			return err
		}
		var l2 uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO lessons (course_id, title, language, age, level, lesson_type, duration_min, skills, tags)
			VALUES ($1, 'Сәлеметсіз бе!', 'Kazakh', 'Teens', 'A1', 'Conversation', 45, '{"Speaking"}', '{"Travel"}') RETURNING id`, course2).Scan(&l2); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO sections (lesson_id, title, position) VALUES ($1, 'Приветствия', 0)`, l2)
		fmt.Println("demo data created")
		return err
	})
	if err != nil {
		return err
	}
	// The author course is loaded on every seed: it adds only what this teacher has never received.
	return loadCourse(ctx, pool, "teacher@ortalyq.local")
}

// seedCourse loads the author courses of internal/content into one teacher's «Материалы» without touching
// accounts: `api seed-course [teacher-email]` (default teacher@ortalyq.local). Safe to repeat.
func seedCourse(cfg config.Config, args []string) error {
	if !cfg.IsLocal() {
		return fmt.Errorf("seed-course is only allowed with APP_ENV=local")
	}
	email := "teacher@ortalyq.local"
	if len(args) > 0 {
		email = args[0]
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	return loadCourse(ctx, pool, email)
}

func loadCourse(ctx context.Context, pool *pgxpool.Pool, email string) error {
	var owner uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1) AND role = 'teacher'`, email).Scan(&owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("teacher %s not found", email)
		}
		return err
	}
	for _, c := range []content.Course{content.EnglishA1} {
		r, err := content.Load(ctx, pool, owner, c)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Title, err)
		}
		switch {
		case r.CourseDeleted:
			fmt.Printf("«%s»: loaded before and deleted by %s — not restored\n", c.Title, email)
			continue
		case r.CourseCreated:
			fmt.Printf("«%s»: created for %s (course %s)\n", c.Title, email, r.CourseID)
		default:
			fmt.Printf("«%s»: already in %s's materials (course %s)\n", c.Title, email, r.CourseID)
		}
		for _, t := range r.Created {
			fmt.Printf("  + %s\n", t)
		}
		for _, t := range r.Kept {
			fmt.Printf("  = %s (loaded before, the teacher's version is kept)\n", t)
		}
		for _, t := range r.Deleted {
			fmt.Printf("  - %s (deleted by the teacher, not restored)\n", t)
		}
		fmt.Printf("  text blocks: %d added, %d loaded before (left as is)\n", r.BlocksAdded, r.BlocksKept)
		for _, b := range r.BlocksSkipped {
			fmt.Printf("  ! %s: the section is renamed or deleted, block not added\n", b)
		}
	}
	return nil
}
