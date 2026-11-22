package users

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ortalyq/backend/internal/database"
)

type Repository struct{ db database.Querier }

func NewRepository(db database.Querier) *Repository { return &Repository{db: db} }

const userCols = `id, email, full_name, role, avatar_color, disabled_at, created_at`

func scanUser(row interface{ Scan(...any) error }, u *User) error {
	return row.Scan(&u.ID, &u.Email, &u.FullName, &u.Role, &u.AvatarColor, &u.DisabledAt, &u.CreatedAt)
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]User, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE ($1 = '' OR role = $1)
		AND ($2 = '' OR full_name ILIKE '%' || $2 || '%' OR email ILIKE '%' || $2 || '%')`, f.Role, f.Query).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+userCols+` FROM users WHERE ($1 = '' OR role = $1)
		AND ($2 = '' OR full_name ILIKE '%' || $2 || '%' OR email ILIKE '%' || $2 || '%')
		ORDER BY created_at DESC, id LIMIT $3 OFFSET $4`, f.Role, f.Query, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := scanUser(rows, &u); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	if err := scanUser(r.db.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) Create(ctx context.Context, email, name, role, hash, color string) (*User, error) {
	var u User
	err := scanUser(r.db.QueryRow(ctx, `INSERT INTO users (email, full_name, role, password_hash, avatar_color)
		VALUES ($1, $2, $3, $4, $5) RETURNING `+userCols, email, name, role, hash, color), &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, name, role *string, disabled *bool, hash *string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET
		full_name = COALESCE($2, full_name),
		role = COALESCE($3, role),
		disabled_at = CASE WHEN $4::bool IS NULL THEN disabled_at WHEN $4 THEN COALESCE(disabled_at, now()) ELSE NULL END,
		password_hash = COALESCE($5, password_hash),
		updated_at = now()
		WHERE id = $1`, id, name, role, disabled, hash)
	return err
}

func (r *Repository) Assign(ctx context.Context, teacherID, studentID, by uuid.UUID) error {
	_, err := r.db.Exec(ctx, `INSERT INTO teacher_students (teacher_id, student_id, assigned_by) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, teacherID, studentID, by)
	return err
}

func (r *Repository) Unassign(ctx context.Context, teacherID, studentID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM teacher_students WHERE teacher_id = $1 AND student_id = $2`, teacherID, studentID)
	return err
}

func (r *Repository) IsAssigned(ctx context.Context, teacherID, studentID uuid.UUID) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM teacher_students WHERE teacher_id = $1 AND student_id = $2)`, teacherID, studentID).Scan(&ok)
	return ok, err
}

func (r *Repository) Assignments(ctx context.Context, limit, offset int) ([]Assignment, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM teacher_students`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `SELECT
		t.id, t.email, t.full_name, t.role, t.avatar_color, t.disabled_at, t.created_at,
		s.id, s.email, s.full_name, s.role, s.avatar_color, s.disabled_at, s.created_at, ts.created_at
		FROM teacher_students ts JOIN users t ON t.id = ts.teacher_id JOIN users s ON s.id = ts.student_id
		ORDER BY ts.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Assignment{}
	for rows.Next() {
		var a Assignment
		t, s := &a.Teacher, &a.Student
		if err := rows.Scan(&t.ID, &t.Email, &t.FullName, &t.Role, &t.AvatarColor, &t.DisabledAt, &t.CreatedAt,
			&s.ID, &s.Email, &s.FullName, &s.Role, &s.AvatarColor, &s.DisabledAt, &s.CreatedAt, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (r *Repository) StudentsOf(ctx context.Context, teacherID uuid.UUID) ([]User, error) {
	rows, err := r.db.Query(ctx, `SELECT u.id, u.email, u.full_name, u.role, u.avatar_color, u.disabled_at, u.created_at
		FROM teacher_students ts JOIN users u ON u.id = ts.student_id
		WHERE ts.teacher_id = $1 AND u.disabled_at IS NULL ORDER BY u.full_name`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := scanUser(rows, &u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// HasAssignments reports whether the user has students assigned as a teacher.
func (r *Repository) HasAssignments(ctx context.Context, teacherID uuid.UUID) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM teacher_students WHERE teacher_id = $1)`, teacherID).Scan(&ok)
	return ok, err
}

// EmailTaken reports whether any account (also a disabled one) uses the address, case-insensitively.
// q may be a transaction.
func (r *Repository) EmailTaken(ctx context.Context, q database.Querier, email string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1))`, email).Scan(&ok)
	return ok, err
}

// CreateSelfRegistered inserts an account created by the sign-up flow (inside the caller's transaction).
func (r *Repository) CreateSelfRegistered(ctx context.Context, q database.Querier, email, name, role, hash, color string, termsAt time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `INSERT INTO users (email, full_name, role, password_hash, avatar_color, terms_accepted_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, email, name, role, hash, color, termsAt).Scan(&id)
	return id, err
}
