package boards

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/uploads"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// likePattern escapes LIKE wildcards so a search for «50%» finds that text.
func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

func (r *Repository) List(ctx context.Context, owner uuid.UUID, q string, limit, offset int) ([]Summary, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM boards WHERE owner_id = $1 AND ($2 = '' OR title ILIKE $3)`,
		owner, q, likePattern(q)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id, title, updated_at, created_at FROM boards
		WHERE owner_id = $1 AND ($2 = '' OR title ILIKE $3) ORDER BY updated_at DESC, id LIMIT $4 OFFSET $5`,
		owner, q, likePattern(q), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.ID, &s.Title, &s.UpdatedAt, &s.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

func (r *Repository) Create(ctx context.Context, owner uuid.UUID, title string) (*Board, error) {
	b := &Board{Content: Content{Items: []Item{}}}
	err := r.pool.QueryRow(ctx, `INSERT INTO boards (owner_id, title) VALUES ($1, $2)
		RETURNING id, title, version, updated_at, created_at`, owner, title).Scan(&b.ID, &b.Title, &b.Version, &b.UpdatedAt, &b.CreatedAt)
	return b, err
}

// Owned returns the owner's board, or nil when it does not exist or belongs to someone else.
func (r *Repository) Owned(ctx context.Context, owner, id uuid.UUID) (*Board, error) {
	b := &Board{}
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT id, title, version, content, updated_at, created_at FROM boards WHERE id = $1 AND owner_id = $2`,
		id, owner).Scan(&b.ID, &b.Title, &b.Version, &raw, &b.UpdatedAt, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &b.Content); err != nil {
		return nil, err
	}
	if b.Content.Items == nil {
		b.Content.Items = []Item{}
	}
	return b, nil
}

// Save writes the drawing when the stored version is still expected (or always with force). It reports whether
// the board exists for this owner and whether the version matched.
func (r *Repository) Save(ctx context.Context, owner, id uuid.UUID, title *string, content []byte, version int, force bool) (found, saved bool, err error) {
	tag, err := r.pool.Exec(ctx, `UPDATE boards SET content = $3, title = COALESCE($4, title), version = version + 1, updated_at = now()
		WHERE id = $1 AND owner_id = $2 AND ($5 OR version = $6)`, id, owner, content, title, force, version)
	if err != nil {
		return false, false, err
	}
	if tag.RowsAffected() == 1 {
		return true, true, nil
	}
	var exists bool
	err = r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM boards WHERE id = $1 AND owner_id = $2)`, id, owner).Scan(&exists)
	return exists, false, err
}

func (r *Repository) ImageCount(ctx context.Context, boardID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM board_images WHERE board_id = $1`, boardID).Scan(&n)
	return n, err
}

func (r *Repository) AddImage(ctx context.Context, boardID, owner uuid.UUID, original string, st *uploads.StoredImage) (*Image, error) {
	img := &Image{Width: st.Width, Height: st.Height, MimeType: st.MimeType, StoredName: st.StoredName, SizeBytes: st.SizeBytes}
	name := []rune(strings.TrimSpace(original))
	if len(name) > 200 {
		name = name[:200]
	}
	err := r.pool.QueryRow(ctx, `INSERT INTO board_images (board_id, owner_id, original_name, stored_name, mime_type, size_bytes, width, height)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		boardID, owner, string(name), st.StoredName, st.MimeType, st.SizeBytes, st.Width, st.Height).Scan(&img.ID)
	return img, err
}

// Image returns a picture of the owner's board, or nil.
func (r *Repository) Image(ctx context.Context, boardID, owner, id uuid.UUID) (*Image, error) {
	img := &Image{ID: id}
	err := r.pool.QueryRow(ctx, `SELECT i.width, i.height, i.mime_type, i.stored_name, i.size_bytes FROM board_images i
		JOIN boards b ON b.id = i.board_id WHERE i.id = $1 AND i.board_id = $2 AND b.owner_id = $3`, id, boardID, owner).
		Scan(&img.Width, &img.Height, &img.MimeType, &img.StoredName, &img.SizeBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return img, err
}

// ImagesBelong tells whether every id is a picture of this owner's board.
func (r *Repository) ImagesBelong(ctx context.Context, boardID, owner uuid.UUID, ids []uuid.UUID) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(DISTINCT i.id) FROM board_images i JOIN boards b ON b.id = i.board_id
		WHERE i.board_id = $1 AND b.owner_id = $2 AND i.id = ANY($3)`, boardID, owner, ids).Scan(&n)
	if err != nil {
		return false, err
	}
	uniq := map[uuid.UUID]bool{}
	for _, id := range ids {
		uniq[id] = true
	}
	return n == len(uniq), nil
}

// OpenIn lists the classes where the board is open.
func (r *Repository) OpenIn(ctx context.Context, boardID uuid.UUID) ([]OpenClass, error) {
	rows, err := r.pool.Query(ctx, `SELECT c.id, c.name FROM classroom_boards cb JOIN classrooms c ON c.id = cb.classroom_id
		WHERE cb.board_id = $1 ORDER BY c.name, c.id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpenClass{}
	for rows.Next() {
		var o OpenClass
		if err := rows.Scan(&o.ClassroomID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// OpenInClass makes the board the one open in the class lesson (replacing another board). Drawing permissions
// start over: they never carry to another board or to a new opening.
func (r *Repository) OpenInClass(ctx context.Context, classroomID, boardID, by uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM classroom_board_drawers WHERE classroom_id = $1`, classroomID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO classroom_boards (classroom_id, board_id, opened_by) VALUES ($1, $2, $3)
			ON CONFLICT (classroom_id) DO UPDATE SET board_id = EXCLUDED.board_id, opened_by = EXCLUDED.opened_by, opened_at = now()`,
			classroomID, boardID, by)
		return err
	})
}

// OpenBoardID returns the board open in the class, or nil.
func (r *Repository) OpenBoardID(ctx context.Context, classroomID uuid.UUID) (*uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT board_id FROM classroom_boards WHERE classroom_id = $1`, classroomID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &id, err
}

// Drawers lists the students allowed to draw on the board open in the class.
func (r *Repository) Drawers(ctx context.Context, classroomID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT d.user_id FROM classroom_board_drawers d JOIN classroom_boards cb
		ON cb.classroom_id = d.classroom_id AND cb.board_id = d.board_id WHERE d.classroom_id = $1 ORDER BY d.granted_at, d.user_id`, classroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Grant lets the student draw on the board open in the class (idempotent).
func (r *Repository) Grant(ctx context.Context, classroomID, boardID, userID, by uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO classroom_board_drawers (classroom_id, user_id, board_id, granted_by) VALUES ($1, $2, $3, $4)
		ON CONFLICT (classroom_id, user_id) DO UPDATE SET board_id = EXCLUDED.board_id, granted_by = EXCLUDED.granted_by, granted_at = now()`,
		classroomID, userID, boardID, by)
	return err
}

func (r *Repository) Revoke(ctx context.Context, classroomID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM classroom_board_drawers WHERE classroom_id = $1 AND user_id = $2`, classroomID, userID)
	return err
}

// errStroke tells AddStroke's caller why a stroke was refused.
type strokeRefusal int

const (
	strokeOK         strokeRefusal = iota
	strokeNoBoard                  // no board open, or another board than the one drawn on
	strokeNotAllowed               // no permission for this board
	strokeFull                     // the board has the most items allowed
)

// AddStroke appends a student's stroke to the board open in the class as one action, under the lock of the class
// board and the board rows: the permission is checked in the same transaction, a repeated client_id adds nothing.
func (r *Repository) AddStroke(ctx context.Context, classroomID, userID uuid.UUID, req StrokeRequest, item []byte, itemID string) (res StrokeResult, refusal strokeRefusal, err error) {
	res.ItemID = itemID
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var open uuid.UUID
		err := tx.QueryRow(ctx, `SELECT board_id FROM classroom_boards WHERE classroom_id = $1 FOR UPDATE`, classroomID).Scan(&open)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && open != req.BoardID) {
			refusal = strokeNoBoard
			return nil
		}
		if err != nil {
			return err
		}
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM classroom_board_drawers WHERE classroom_id = $1 AND user_id = $2 AND board_id = $3)`,
			classroomID, userID, open).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			refusal = strokeNotAllowed
			return nil
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT jsonb_array_length(content->'items'), version FROM boards WHERE id = $1 FOR UPDATE`, open).Scan(&n, &res.Version); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO board_stroke_receipts (board_id, client_id, user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			open, req.ClientID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			res.Duplicate = true
			return nil
		}
		if n >= MaxItems {
			refusal = strokeFull
			return errRollback
		}
		return tx.QueryRow(ctx, `UPDATE boards SET content = jsonb_set(content, '{items}', COALESCE(content->'items', '[]'::jsonb) || jsonb_build_array($2::jsonb)),
			version = version + 1, updated_at = now() WHERE id = $1 RETURNING version`, open, item).Scan(&res.Version)
	})
	if errors.Is(err, errRollback) {
		err = nil
	}
	return res, refusal, err
}

// errRollback undoes the transaction without reporting an error (the refusal says why).
var errRollback = errors.New("rollback")

// CloseInClass ends the lesson access; it returns the board that was open (nil when none).
func (r *Repository) CloseInClass(ctx context.Context, classroomID uuid.UUID) (*uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `DELETE FROM classroom_boards WHERE classroom_id = $1 RETURNING board_id`, classroomID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &id, err
}

// ClassBoard returns the board open in the class lesson, or nil.
func (r *Repository) ClassBoard(ctx context.Context, classroomID uuid.UUID) (*Board, time.Time, error) {
	var owner, id uuid.UUID
	var at time.Time
	err := r.pool.QueryRow(ctx, `SELECT cb.board_id, b.owner_id, cb.opened_at FROM classroom_boards cb JOIN boards b ON b.id = cb.board_id
		WHERE cb.classroom_id = $1`, classroomID).Scan(&id, &owner, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, at, nil
	}
	if err != nil {
		return nil, at, err
	}
	b, err := r.Owned(ctx, owner, id)
	return b, at, err
}

// ClassImage returns a picture of the board open in the class lesson, or nil (also for another board's picture).
func (r *Repository) ClassImage(ctx context.Context, classroomID, imageID uuid.UUID) (*Image, error) {
	img := &Image{ID: imageID}
	err := r.pool.QueryRow(ctx, `SELECT i.width, i.height, i.mime_type, i.stored_name, i.size_bytes FROM board_images i
		JOIN classroom_boards cb ON cb.board_id = i.board_id WHERE cb.classroom_id = $1 AND i.id = $2`, classroomID, imageID).
		Scan(&img.Width, &img.Height, &img.MimeType, &img.StoredName, &img.SizeBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return img, err
}

// ClassesOf lists the classes where the board is open (for update notifications).
func (r *Repository) ClassesOf(ctx context.Context, boardID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT classroom_id FROM classroom_boards WHERE board_id = $1`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ClassName returns the name of the class.
func (r *Repository) ClassName(ctx context.Context, classroomID uuid.UUID) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT name FROM classrooms WHERE id = $1`, classroomID).Scan(&name)
	return name, err
}

// Images lists the pictures of a board.
func (r *Repository) Images(ctx context.Context, boardID uuid.UUID) ([]Image, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, width, height, mime_type, stored_name, size_bytes FROM board_images WHERE board_id = $1`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Image
	for rows.Next() {
		var img Image
		if err := rows.Scan(&img.ID, &img.Width, &img.Height, &img.MimeType, &img.StoredName, &img.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

// CopyByRequest returns the copy made earlier by the same request, or nil.
func (r *Repository) CopyByRequest(ctx context.Context, owner, requestID uuid.UUID) (*uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT copy_board_id FROM board_copies WHERE owner_id = $1 AND request_id = $2`, owner, requestID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &id, err
}

// CopiedImage is a picture of the copy: its new id and its own copied file.
type CopiedImage struct {
	ID         uuid.UUID
	Source     Image
	StoredName string
}

// CreateCopy writes the copy board, its pictures and the request record in one transaction. When the same request
// was recorded meanwhile (a parallel repeat), nothing is written and the existing copy is returned (created false).
func (r *Repository) CreateCopy(ctx context.Context, owner, sourceID, classroomID, requestID uuid.UUID, title string, content []byte, images []CopiedImage) (uuid.UUID, bool, error) {
	var id uuid.UUID
	created := false
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "board-copy:"+owner.String()+":"+requestID.String()); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT copy_board_id FROM board_copies WHERE owner_id = $1 AND request_id = $2`, owner, requestID).Scan(&id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO boards (owner_id, title, content) VALUES ($1, $2, $3) RETURNING id`, owner, title, content).Scan(&id); err != nil {
			return err
		}
		for _, img := range images {
			if _, err := tx.Exec(ctx, `INSERT INTO board_images (id, board_id, owner_id, original_name, stored_name, mime_type, size_bytes, width, height)
				SELECT $1, $2, $3, original_name, $4, mime_type, size_bytes, width, height FROM board_images WHERE id = $5`,
				img.ID, id, owner, img.StoredName, img.Source.ID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO board_copies (owner_id, request_id, source_board_id, copy_board_id, classroom_id) VALUES ($1, $2, $3, $4, $5)`,
			owner, requestID, sourceID, id, classroomID); err != nil {
			return err
		}
		created = true
		return nil
	})
	return id, created, err
}
