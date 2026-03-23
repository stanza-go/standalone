// Package notifications provides the notification service and shared types.
// Notifications target a specific entity (admin or user) identified by
// entity_type and entity_id. Other modules use the exported functions to
// create notifications; the admin and user notification endpoints read them.
//
// The Service type extends these capabilities with optional email delivery
// via the Resend API. Callers opt-in to email per notification using
// WithEmail().
package notifications

import (
	"context"
	"fmt"

	"github.com/stanza-go/framework/pkg/email"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/task"
)

// Entity types for the entity_type column.
const (
	EntityAdmin = "admin"
	EntityUser  = "user"
)

// Notification represents a single notification row.
type Notification struct {
	ID         int64  `json:"id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	Data       string `json:"data,omitempty"`
	ReadAt     string `json:"read_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Option configures notification creation behavior.
type Option func(*opts)

type opts struct {
	sendEmail bool
	ctx       context.Context
}

// WithEmail opts-in to sending an email alongside the in-app notification.
// The recipient email is looked up from the admin/user table automatically.
// Email delivery is best-effort: failures are logged but do not prevent the
// in-app notification from being created.
func WithEmail(ctx context.Context) Option {
	return func(o *opts) {
		o.sendEmail = true
		o.ctx = ctx
	}
}

// Service provides notification creation with optional email delivery.
// Use NewService to create one.
type Service struct {
	db     *sqlite.DB
	email  *email.Client
	pool   *task.Pool
	logger *log.Logger
	hub    *Hub
}

// NewService creates a notification Service. The email client, pool, and
// logger may be nil — email delivery is silently skipped when the client
// is nil or not configured. When pool is non-nil, emails are sent
// asynchronously via the task pool; otherwise they are sent synchronously.
func NewService(db *sqlite.DB, emailClient *email.Client, pool *task.Pool, logger *log.Logger) *Service {
	return &Service{db: db, email: emailClient, pool: pool, logger: logger, hub: NewHub()}
}

// Hub returns the notification broadcast hub for stream subscriptions.
func (s *Service) Hub() *Hub { return s.hub }

// DB returns the underlying database handle so endpoint modules can query
// notifications without needing a separate db dependency.
func (s *Service) DB() *sqlite.DB { return s.db }

// NotifyAdmin creates a notification for an admin and optionally sends an
// email. Connected streaming clients for this admin receive the notification
// in real-time.
func (s *Service) NotifyAdmin(adminID int64, notifType, title, message string, options ...Option) (int64, error) {
	id, err := Notify(s.db, EntityAdmin, adminID, notifType, title, message, "")
	if err != nil {
		return 0, err
	}
	o := applyOpts(options)
	if o.sendEmail {
		s.sendEmail(o.ctx, EntityAdmin, adminID, notifType, title, message)
	}
	s.publishToAdmin(adminID, id, notifType, title, message)
	return id, nil
}

// NotifyUser creates a notification for an end user and optionally sends an
// email. (User-side streaming is not implemented — only admin hub is used.)
func (s *Service) NotifyUser(userID int64, notifType, title, message string, options ...Option) (int64, error) {
	id, err := Notify(s.db, EntityUser, userID, notifType, title, message, "")
	if err != nil {
		return 0, err
	}
	o := applyOpts(options)
	if o.sendEmail {
		s.sendEmail(o.ctx, EntityUser, userID, notifType, title, message)
	}
	return id, nil
}

// NotifyAllAdmins creates a notification for every active admin and
// optionally sends emails to each. Connected streaming clients for each
// admin receive the notification in real-time.
func (s *Service) NotifyAllAdmins(notifType, title, message string, options ...Option) error {
	o := applyOpts(options)

	ids, err := activeAdminIDs(s.db)
	if err != nil {
		return err
	}

	for _, id := range ids {
		nID, err := Notify(s.db, EntityAdmin, id, notifType, title, message, "")
		if err != nil {
			return err
		}
		if o.sendEmail {
			s.sendEmail(o.ctx, EntityAdmin, id, notifType, title, message)
		}
		s.publishToAdmin(id, nID, notifType, title, message)
	}
	return nil
}

// sendEmail looks up the recipient email and sends a notification email.
// When a task pool is available, the send is dispatched asynchronously so
// the caller returns immediately. Failures are logged but never returned
// — email is best-effort.
func (s *Service) sendEmail(_ context.Context, entityType string, entityID int64, notifType, title, message string) {
	if s.email == nil || !s.email.Configured() {
		return
	}

	addr, err := s.lookupEmail(entityType, entityID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("notification email: lookup failed",
				log.String("entity_type", entityType),
				log.Int64("entity_id", entityID),
				log.Err(err),
			)
		}
		return
	}
	if addr == "" {
		return
	}

	html := renderHTML(notifType, title, message)
	text := renderText(notifType, title, message)

	send := func() {
		// Use a detached context — the original HTTP request context may
		// already be cancelled by the time the pool runs this task.
		_, err := s.email.Send(context.Background(), email.Message{
			To:      []string{addr},
			Subject: formatSubject(notifType, title),
			HTML:    html,
			Text:    text,
		})
		if err != nil && s.logger != nil {
			s.logger.Error("notification email: send failed",
				log.String("to", addr),
				log.String("type", notifType),
				log.Err(err),
			)
		}
	}

	if s.pool != nil && s.pool.Submit(send) {
		return
	}
	// No pool or pool full — send synchronously as fallback.
	send()
}

// lookupEmail fetches the email address for a given entity.
func (s *Service) lookupEmail(entityType string, entityID int64) (string, error) {
	var table string
	switch entityType {
	case EntityAdmin:
		table = "admins"
	case EntityUser:
		table = "users"
	default:
		return "", fmt.Errorf("unknown entity type: %s", entityType)
	}

	var addr string
	sq, sa := sqlite.Select("email").From(table).Where("id = ?", entityID).Build()
	if err := s.db.QueryRow(sq, sa...).Scan(&addr); err != nil {
		return "", err
	}
	return addr, nil
}

// formatSubject builds the email subject line.
func formatSubject(notifType, title string) string {
	switch notifType {
	case "error":
		return "[Alert] " + title
	case "warning":
		return "[Warning] " + title
	default:
		return title
	}
}

// renderHTML builds the HTML email body.
func renderHTML(notifType, title, message string) string {
	color := typeColor(notifType)
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; color: #1a1a1a;">
  <div style="border-left: 4px solid %s; padding-left: 16px; margin-bottom: 20px;">
    <h2 style="margin: 0 0 8px 0;">%s</h2>
    <p style="margin: 0; color: #444; line-height: 1.5;">%s</p>
  </div>
  <p style="color: #999; font-size: 12px; margin-top: 32px;">This is an automated notification. You can view all notifications in your dashboard.</p>
</body>
</html>`, color, title, message)
}

// renderText builds the plain-text email body.
func renderText(_, title, message string) string {
	return fmt.Sprintf("%s\n\n%s\n\nThis is an automated notification.", title, message)
}

// typeColor maps notification type to a border color for the email template.
func typeColor(notifType string) string {
	switch notifType {
	case "success":
		return "#22c55e"
	case "warning":
		return "#f59e0b"
	case "error":
		return "#ef4444"
	default:
		return "#3b82f6"
	}
}

func applyOpts(options []Option) opts {
	var o opts
	for _, fn := range options {
		fn(&o)
	}
	return o
}

// Notify creates a notification for a specific entity.
func Notify(db *sqlite.DB, entityType string, entityID int64, notifType, title, message, data string) (int64, error) {
	now := sqlite.Now()
	id, err := db.Insert(sqlite.Insert("notifications").
		Set("entity_type", entityType).
		Set("entity_id", entityID).
		Set("type", notifType).
		Set("title", title).
		Set("message", message).
		Set("data", data).
		Set("created_at", now))
	if err != nil {
		return 0, err
	}
	return id, nil
}

// activeAdminIDs returns the IDs of all active, non-deleted admins.
func activeAdminIDs(db *sqlite.DB) ([]int64, error) {
	sq, sa := sqlite.Select("id").From("admins").
		Where("is_active = 1").
		WhereNull("deleted_at").
		Build()
	return sqlite.QueryAll(db, sq, sa, func(rows *sqlite.Rows) (int64, error) {
		var id int64
		err := rows.Scan(&id)
		return id, err
	})
}

// publishToAdmin broadcasts a notification event to connected streaming
// subscribers for the given admin. It also includes the updated unread count.
func (s *Service) publishToAdmin(adminID, notifID int64, notifType, title, message string) {
	unread := UnreadCount(s.db, EntityAdmin, adminID)
	s.hub.Publish(adminID, Event{
		Type: "notification",
		Notification: &Notification{
			ID:         notifID,
			EntityType: EntityAdmin,
			EntityID:   adminID,
			Type:       notifType,
			Title:      title,
			Message:    message,
			CreatedAt:  sqlite.Now(),
		},
		UnreadCount: unread,
	})
}

// UnreadCount returns the number of unread notifications for an entity.
func UnreadCount(db *sqlite.DB, entityType string, entityID int64) int {
	sql, args := sqlite.Count("notifications").
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", entityID).
		WhereNull("read_at").
		Build()
	var count int
	_ = db.QueryRow(sql, args...).Scan(&count)
	return count
}
