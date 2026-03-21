package notifications

import "sync"

// Event is sent to WebSocket subscribers when a notification is created.
type Event struct {
	Type         string        `json:"type"`
	Notification *Notification `json:"notification,omitempty"`
	UnreadCount  int           `json:"unread_count"`
}

// Hub tracks active WebSocket connections and broadcasts notification events
// to the relevant admin subscribers. Thread-safe for concurrent use.
type Hub struct {
	mu   sync.Mutex
	subs map[int64]map[*subscriber]struct{} // adminID -> set of subscribers
}

type subscriber struct {
	ch chan Event
}

// NewHub creates a notification broadcast hub.
func NewHub() *Hub {
	return &Hub{
		subs: make(map[int64]map[*subscriber]struct{}),
	}
}

// Subscribe registers a WebSocket connection for a specific admin.
// Returns a channel that receives events and an unsubscribe function.
func (h *Hub) Subscribe(adminID int64) (<-chan Event, func()) {
	s := &subscriber{ch: make(chan Event, 16)}

	h.mu.Lock()
	if h.subs[adminID] == nil {
		h.subs[adminID] = make(map[*subscriber]struct{})
	}
	h.subs[adminID][s] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		delete(h.subs[adminID], s)
		if len(h.subs[adminID]) == 0 {
			delete(h.subs, adminID)
		}
		h.mu.Unlock()
	}

	return s.ch, unsub
}

// Publish sends an event to all subscribers for the given admin.
// Non-blocking: if a subscriber's buffer is full, the event is dropped.
// Holds the lock during sends — safe because sends are non-blocking.
func (h *Hub) Publish(adminID int64, evt Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for s := range h.subs[adminID] {
		select {
		case s.ch <- evt:
		default:
			// Subscriber too slow — drop event.
		}
	}
}

// PublishAll sends an event to every connected subscriber across all admins.
// Non-blocking per subscriber.
func (h *Hub) PublishAll(evt Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, subs := range h.subs {
		for s := range subs {
			select {
			case s.ch <- evt:
			default:
			}
		}
	}
}
