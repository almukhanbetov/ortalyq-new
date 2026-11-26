package library

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/content"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Owned maps catalog keys to the teacher's existing copies.
func (r *Repository) Owned(ctx context.Context, owner uuid.UUID, keys []string) (map[string]uuid.UUID, error) {
	return content.OwnedCourses(ctx, r.pool, owner, keys)
}

// Add gives the teacher a copy of the course (or finds the existing one).
func (r *Repository) Add(ctx context.Context, owner uuid.UUID, c content.Course) (*content.Result, error) {
	return content.AddFromCatalog(ctx, r.pool, owner, c)
}
