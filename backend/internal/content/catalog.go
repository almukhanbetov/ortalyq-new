package content

import (
	"context"

	"github.com/google/uuid"

	"ortalyq/backend/internal/database"
)

// Catalog is the list of author courses offered in «Материалы» → «Каталог».
var Catalog = []Course{EnglishA1}

// Find returns the catalog course with this key.
func Find(key string) (Course, bool) {
	for _, c := range Catalog {
		if c.Key == key {
			return c, true
		}
	}
	return Course{}, false
}

// OwnedCourses maps catalog keys to the teacher's existing copies (a deleted copy is not listed).
func OwnedCourses(ctx context.Context, q database.Querier, owner uuid.UUID, keys []string) (map[string]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT ci.key, c.id FROM content_imports ci JOIN courses c ON c.id = ci.entity_id AND c.owner_id = ci.owner_id
		WHERE ci.owner_id = $1 AND ci.key = ANY($2)`, owner, keys)
	if err != nil {
		return nil, err
	}
	out := map[string]uuid.UUID{}
	for rows.Next() {
		var k string
		var id uuid.UUID
		if err := rows.Scan(&k, &id); err != nil {
			rows.Close()
			return nil, err
		}
		out[k] = id
	}
	rows.Close()
	return out, rows.Err()
}
