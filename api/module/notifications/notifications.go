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
	"time"

	"github.com/stanza-go/framework/pkg/email"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
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
	logger *log.Logger
}

// NewService creates a notification Service. The email client and logger may
// be nil — email delivery is silently skipped when the client is nil or not
// configured.
func NewService(db *sqlite.DB, emailClient *email.Client, logger *log.Logger) *Service {
	return &Service{db: db, email: emailClient, logger: logger}
}

// DB returns the underlying database handle so endpoint modules can query
// notifications without needing a separate db dependency.
func (s *Service) DB() *sqlite.DB { return s.db }

// NotifyAdmin creates a notification for an admin and optionally sends an
// email.
func (s *Service) NotifyAdmin(adminID int64, notifType, title, message string, options ...Option) (int64, error) {
	id, err := NotifyAdmin(s.db, adminID, notifType, title, message)
	if err != nil {
		return 0, err
	}
	o := applyOpts(options)
	if o.sendEmail {
		s.sendEmail(o.ctx, EntityAdmin, adminID, notifType, title, message)
	}
	return id, nil
}

// NotifyUser creates a notification for an end user and optionally sends an
// email.
func (s *Service) NotifyUser(userID int64, notifType, title, message string, options ...Option) (int64, error) {
	id, err := NotifyUser(s.db, userID, notifType, title, message)
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
// optionally sends emails to each.
func (s *Service) NotifyAllAdmins(notifType, title, message string, options ...Option) error {
	o := applyOpts(options)

	rows, err := s.db.Query("SELECT id FROM admins WHERE is_active = 1 AND deleted_at IS NULL")
	if err != nil {
		return err
	}

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		if _, err := NotifyAdmin(s.db, id, notifType, title, message); err != nil {
			return err
		}
		if o.sendEmail {
			s.sendEmail(o.ctx, EntityAdmin, id, notifType, title, message)
		}
	}
	return nil
}

// sendEmail looks up the recipient email and sends a notification email.
// Failures are logged but never returned — email is best-effort.
func (s *Service) sendEmail(ctx context.Context, entityType string, entityID int64, notifType, title, message string) {
	if s.email == nil || !s.email.Configured() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	addr, err := s.lookupEmail(entityType, entityID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("notification email: lookup failed",
				log.String("entity_type", entityType),
				log.Int64("entity_id", entityID),
				log.String("error", err.Error()),
			)
		}
		return
	}
	if addr == "" {
		return
	}

	html := renderHTML(notifType, title, message)
	text := renderText(notifType, title, message)

	_, err = s.email.Send(ctx, email.Message{
		To:      []string{addr},
		Subject: formatSubject(notifType, title),
		HTML:    html,
		Text:    text,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Error("notification email: send failed",
				log.String("to", addr),
				log.String("type", notifType),
				log.String("error", err.Error()),
			)
		}
	}
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
	err := s.db.QueryRow("SELECT email FROM "+table+" WHERE id = ?", entityID).Scan(&addr)
	if err != nil {
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

// --- Standalone functions (backward compatible, no email) ---

// Notify creates a notification for a specific entity.
func Notify(db *sqlite.DB, entityType string, entityID int64, notifType, title, message, data string) (int64, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	sql, args := sqlite.Insert("notifications").
		Set("entity_type", entityType).
		Set("entity_id", entityID).
		Set("type", notifType).
		Set("title", title).
		Set("message", message).
		Set("data", data).
		Set("created_at", now).
		Build()
	result, err := db.Exec(sql, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertID, nil
}

// NotifyAdmin creates a notification for an admin user.
func NotifyAdmin(db *sqlite.DB, adminID int64, notifType, title, message string) (int64, error) {
	return Notify(db, EntityAdmin, adminID, notifType, title, message, "")
}

// NotifyUser creates a notification for an end user.
func NotifyUser(db *sqlite.DB, userID int64, notifType, title, message string) (int64, error) {
	return Notify(db, EntityUser, userID, notifType, title, message, "")
}

// NotifyAllAdmins creates a notification for every active admin.
func NotifyAllAdmins(db *sqlite.DB, notifType, title, message string) error {
	rows, err := db.Query("SELECT id FROM admins WHERE is_active = 1 AND deleted_at IS NULL")
	if err != nil {
		return err
	}

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		if _, err := NotifyAdmin(db, id, notifType, title, message); err != nil {
			return err
		}
	}
	return nil
}

// UnreadCount returns the number of unread notifications for an entity.
func UnreadCount(db *sqlite.DB, entityType string, entityID int64) int {
	sql, args := sqlite.Count("notifications").
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", entityID).
		Where("read_at IS NULL").
		Build()
	var count int
	_ = db.QueryRow(sql, args...).Scan(&count)
	return count
}
