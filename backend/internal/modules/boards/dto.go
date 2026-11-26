package boards

import "github.com/google/uuid"

type CreateRequest struct {
	Title string `json:"title"`
}

// SaveRequest saves the drawing (and optionally the title). Version is the version the editor loaded: when the
// board was saved elsewhere meanwhile, the server answers 409 unless Force is set («Сохранить мою версию»).
type SaveRequest struct {
	Title   *string `json:"title"`
	Content Content `json:"content"`
	Version int     `json:"version"`
	Force   bool    `json:"force"`
}

// OpenRequest opens an owned board in a class lesson. With Copy («Создать копию для занятия») a copy of the board
// is made and opened instead; RequestID (required then) makes a repeated request return the same copy.
type OpenRequest struct {
	BoardID   uuid.UUID `json:"board_id"`
	Copy      bool      `json:"copy"`
	RequestID uuid.UUID `json:"request_id"`
}

// StrokeRequest is a student's pen stroke on the lesson board. BoardID is the board the student drew on (a stroke
// from a tab of a closed or replaced board is refused); ClientID identifies the stroke for safe resending.
type StrokeRequest struct {
	BoardID  uuid.UUID    `json:"board_id"`
	ClientID uuid.UUID    `json:"client_id"`
	Color    string       `json:"color"`
	Width    float64      `json:"width"`
	Points   [][2]float64 `json:"points"`
}
