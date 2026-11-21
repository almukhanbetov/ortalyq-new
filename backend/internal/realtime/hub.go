// Package realtime delivers classroom events over WebSocket. Events are
// notifications only; PostgreSQL remains the source of truth and clients
// re-read state through the REST API after (re)connecting.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const (
	EventMessageCreated    = "chat.message.created"
	EventMessageUpdated    = "chat.message.updated"
	EventMessageDeleted    = "chat.message.deleted"
	EventHomeworkStatus    = "homework.status_changed"
	EventLessonCompletion  = "lesson.completion_changed"
	EventSectionCompletion = "section.completion_changed"
	EventCurrentLesson     = "classroom.lesson_changed"
	EventAttemptUpdated    = "attempt.updated"
	EventMaterialsChanged  = "materials.changed"
	// EventMarathonChanged: lessons, students, progress or dates of a marathon changed (notification only).
	EventMarathonChanged = "marathon.changed"
)

type Event struct {
	Type        string    `json:"type"`
	ClassroomID uuid.UUID `json:"classroom_id"`
	Payload     any       `json:"payload"`
	At          time.Time `json:"at"`
}

// Publisher is what modules depend on.
type Publisher interface {
	Publish(classroomID uuid.UUID, typ string, payload any)
}

type client struct {
	userID uuid.UUID
	send   chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	rooms   map[uuid.UUID]map[*client]struct{}
	origins []string
}

func NewHub(allowedOrigins []string) *Hub {
	var hosts []string
	for _, o := range allowedOrigins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			hosts = append(hosts, u.Host)
		}
	}
	return &Hub{rooms: map[uuid.UUID]map[*client]struct{}{}, origins: hosts}
}

func (h *Hub) Publish(classroomID uuid.UUID, typ string, payload any) {
	b, err := json.Marshal(Event{Type: typ, ClassroomID: classroomID, Payload: payload, At: time.Now().UTC()})
	if err != nil {
		slog.Error("realtime marshal", "err", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[classroomID] {
		select {
		case c.send <- b:
		default: // slow consumer: drop; it will resync on reconnect
		}
	}
}

func (h *Hub) add(room uuid.UUID, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = map[*client]struct{}{}
	}
	h.rooms[room][c] = struct{}{}
}

func (h *Hub) remove(room uuid.UUID, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[room], c)
	if len(h.rooms[room]) == 0 {
		delete(h.rooms, room)
	}
}

// Serve upgrades the connection. Authorization must be done by the caller.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, room, userID uuid.UUID) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return
	}
	c := &client{userID: userID, send: make(chan []byte, 64)}
	h.add(room, c)
	defer h.remove(room, c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// Reader: we do not accept client messages; it only detects closure.
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()
	hello, _ := json.Marshal(Event{Type: "hello", ClassroomID: room, At: time.Now().UTC()})
	_ = conn.Write(ctx, websocket.MessageText, hello)
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case msg := <-c.send:
			wctx, wc := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			wc()
			if err != nil {
				return
			}
		case <-ping.C:
			// Protocol ping detects dead peers server-side; the app-level "ping"
			// event lets the browser detect a silent (half-open) connection.
			pctx, pc := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			if err == nil {
				b, _ := json.Marshal(Event{Type: "ping", ClassroomID: room, At: time.Now().UTC()})
				err = conn.Write(pctx, websocket.MessageText, b)
			}
			pc()
			if err != nil {
				return
			}
		}
	}
}
