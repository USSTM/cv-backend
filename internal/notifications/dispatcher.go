package notifications

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/logging"
	"github.com/USSTM/cv-backend/internal/queue"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// defines a set of recipients and an optional email template.
// Template = "" means in-app notification (no-email) only.
type NotifierGroup struct {
	IDs          []uuid.UUID
	Template     string
	TemplateData map[string]interface{}
}

// resolves UUIDs to email address.
type EmailLookupFunc func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)

// full NotificationService interface needed by the dispatcher.
type notificationSvc interface {
	Publish(ctx context.Context, actorID uuid.UUID, entityTypeName string, entityID uuid.UUID, notifierIDs []uuid.UUID) error
	GetUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int64) ([]db.GetUserNotificationsRow, error)
	MarkAsRead(ctx context.Context, userID, notificationID uuid.UUID) (db.Notification, error)
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	GetTotalCount(ctx context.Context, userID uuid.UUID) (int64, error)
}

// subset of TaskQueue.
type queueService interface {
	Enqueue(taskType string, data interface{}) (*asynq.TaskInfo, error)
}

type NotificationDispatcher struct {
	svc         notificationSvc
	queue       queueService
	templates   *template.Template
	emailLookup EmailLookupFunc
}

func NewNotificationDispatcher(svc notificationSvc, q queueService, tmpl *template.Template, lookup EmailLookupFunc) *NotificationDispatcher {
	return &NotificationDispatcher{
		svc:         svc,
		queue:       q,
		templates:   tmpl,
		emailLookup: lookup,
	}
}

// writes in-app notifications for all groups, then enqueues emails for
// groups that specify a template. Email failures are logged, not returned.
func (d *NotificationDispatcher) Notify(ctx context.Context, actorID uuid.UUID, entityType string, entityID uuid.UUID, groups []NotifierGroup) error {
	var allIDs []uuid.UUID
	for _, g := range groups {
		allIDs = append(allIDs, g.IDs...)
	}

	if len(allIDs) == 0 {
		return nil
	}

	if err := d.svc.Publish(ctx, actorID, entityType, entityID, allIDs); err != nil {
		return fmt.Errorf("failed to publish in-app notification: %w", err)
	}

	for _, g := range groups {
		if g.Template == "" {
			continue
		}
		d.sendGroupEmails(ctx, g)
	}

	return nil
}

func (d *NotificationDispatcher) sendGroupEmails(ctx context.Context, g NotifierGroup) {
	if len(g.IDs) == 0 {
		return
	}
	if d.emailLookup == nil {
		logging.Error("email lookup func is nil, skipping email dispatch", "template", g.Template)
		return
	}
	emails, err := d.emailLookup(ctx, g.IDs)
	if err != nil {
		logging.Error("failed to look up emails for notification", "template", g.Template, "error", err)
		return
	}

	subject, body, err := d.renderTemplate(g.Template, g.TemplateData)
	if err != nil {
		logging.Error("failed to render notification template", "template", g.Template, "error", err)
		return
	}

	for _, email := range emails {
		if _, err := d.queue.Enqueue(queue.TypeEmailDelivery, queue.EmailDeliveryPayload{
			To:      email,
			Subject: subject,
			Body:    body,
		}); err != nil {
			logging.Error("failed to enqueue notification email", "to", email, "template", g.Template, "error", err)
		}
	}
}

// only expose dispatcher, notiService should be wrapped under disptacher

func (d *NotificationDispatcher) Publish(ctx context.Context, actorID uuid.UUID, entityTypeName string, entityID uuid.UUID, notifierIDs []uuid.UUID) error {
	return d.svc.Publish(ctx, actorID, entityTypeName, entityID, notifierIDs)
}

func (d *NotificationDispatcher) GetUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int64) ([]db.GetUserNotificationsRow, error) {
	return d.svc.GetUserNotifications(ctx, userID, limit, offset)
}

func (d *NotificationDispatcher) MarkAsRead(ctx context.Context, userID, notificationID uuid.UUID) (db.Notification, error) {
	return d.svc.MarkAsRead(ctx, userID, notificationID)
}

func (d *NotificationDispatcher) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	return d.svc.MarkAllAsRead(ctx, userID)
}

func (d *NotificationDispatcher) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return d.svc.GetUnreadCount(ctx, userID)
}

func (d *NotificationDispatcher) GetTotalCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return d.svc.GetTotalCount(ctx, userID)
}

// {{define "name:subject"}} and {{define "name:body"}}
func (d *NotificationDispatcher) renderTemplate(name string, data map[string]interface{}) (subject, body string, err error) {
	var subjectBuf bytes.Buffer
	if err = d.templates.ExecuteTemplate(&subjectBuf, name+":subject", data); err != nil {
		return "", "", fmt.Errorf("render subject for %q: %w", name, err)
	}

	var bodyBuf bytes.Buffer
	if err = d.templates.ExecuteTemplate(&bodyBuf, name+":body", data); err != nil {
		return "", "", fmt.Errorf("render body for %q: %w", name, err)
	}

	return subjectBuf.String(), bodyBuf.String(), nil
}
