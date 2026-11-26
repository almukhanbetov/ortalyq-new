// Package boards is «Материалы» → «Интерактивные доски»: a teacher's personal whiteboards with pen strokes and
// texts. Only the owner (a teaching role) lists, opens and saves a board; anyone else gets 404, also by a direct link.
package boards

import (
	"time"

	"github.com/google/uuid"
)

// Board coordinates. The editor's initial view is CanvasWidth × CanvasHeight at 100%; with «Рука» and zoom the
// teacher can draw anywhere inside ±WorldLimit (older boards lie inside the initial view).
const (
	CanvasWidth  = 1600
	CanvasHeight = 1000
	WorldLimit   = 20000

	MaxSticky = 1000 // characters of a sticky note
	MinShape  = 4
	MaxShape  = 8000
	MaxImages = 200 // pictures per board

	MaxItems       = 5000
	MaxPoints      = 4000 // per stroke
	MaxText        = 500  // characters per text
	MaxContentSize = 2 << 20
)

const (
	ItemStroke  = "stroke"  // pen: points, width
	ItemText    = "text"    // x, y, size, text
	ItemSticky  = "sticky"  // x, y, w, h, text (may be empty); color is the paper
	ItemRect    = "rect"    // x, y, w, h, width (outline)
	ItemEllipse = "ellipse" // x, y, w, h (bounding box), width
	ItemArrow   = "arrow"   // points: start and end, width
	ItemImage   = "image"   // x, y, w, h, image_id (a picture of this board); no color
)

// Item is one element of the drawing. ID is the editor's stable id of the element (selection, undo); boards saved
// before it existed have no ids, the editor gives them ids on load.
type Item struct {
	ID      string       `json:"id,omitempty"`
	Type    string       `json:"type"`
	Color   string       `json:"color,omitempty"`
	Width   float64      `json:"width,omitempty"`
	Points  [][2]float64 `json:"points,omitempty"`
	X       float64      `json:"x,omitempty"`
	Y       float64      `json:"y,omitempty"`
	W       float64      `json:"w,omitempty"`
	H       float64      `json:"h,omitempty"`
	Text    string       `json:"text,omitempty"`
	Size    float64      `json:"size,omitempty"`
	ImageID string       `json:"image_id,omitempty"`
}

// Image is an uploaded picture of a board.
type Image struct {
	ID         uuid.UUID `json:"id"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	MimeType   string    `json:"mime_type"`
	StoredName string    `json:"-"`
	SizeBytes  int64     `json:"-"`
}

// Content is the whole drawing, stored as JSON.
type Content struct {
	Items []Item `json:"items"`
}

// Summary is a board in the list (without the drawing).
type Summary struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Board is a board with its drawing.
type Board struct {
	Summary
	Version int     `json:"version"`
	Content Content `json:"content"`
	// OpenIn lists the owner's classes where the board is open in the lesson (only in the owner's view).
	OpenIn []OpenClass `json:"open_in,omitempty"`
}

// OpenClass is a class where a board is open.
type OpenClass struct {
	ClassroomID uuid.UUID `json:"classroom_id"`
	Name        string    `json:"name"`
}

// ClassBoard is the board of a lesson as its members see it: the drawing without the owner's details.
type ClassBoard struct {
	Board    *Board    `json:"board"`
	OpenedAt time.Time `json:"opened_at,omitempty"`
	// CanDraw: the caller may add pen strokes (the owner always; a student only with the teacher's permission).
	CanDraw bool `json:"can_draw"`
	// Drawers: students allowed to draw now (shown to the teacher of the class only).
	Drawers []uuid.UUID `json:"drawers"`
	// Copy: the board made by «Создать копию для занятия» in this request (only in that answer).
	Copy *Summary `json:"copy,omitempty"`
}

// StrokeResult is the board version after a student's stroke; Duplicate when the same client_id was sent before.
type StrokeResult struct {
	ItemID    string `json:"item_id"`
	Version   int    `json:"version"`
	Duplicate bool   `json:"duplicate"`
}
