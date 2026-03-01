package notifications

import (
	"context"
	"fmt"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationService struct {
	pool *pgxpool.Pool
	db   *db.Queries
}

func NewNotificationService(pool *pgxpool.Pool, queries *db.Queries) *NotificationService {
	return &NotificationService{
		pool: pool,
		db:   queries,
	}
}

func (s *NotificationService) Publish(ctx context.Context, actorID uuid.UUID, entityTypeName string, entityID uuid.UUID, notifierIDs []uuid.UUID) error {
	entityType, err := s.db.GetNotificationEntityTypeByName(ctx, entityTypeName)
	if err != nil {
		return fmt.Errorf("failed to get entity type %s: %w", entityTypeName, err)
	}

	// because we are inserting multiple records cohesively, we use a transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.db.WithTx(tx)

	obj, err := qtx.CreateNotificationObject(ctx, db.CreateNotificationObjectParams{
		EntityTypeID: entityType.ID,
		EntityID:     entityID,
	})
	if err != nil {
		return fmt.Errorf("failed to create notification object: %w", err)
	}

	_, err = qtx.CreateNotificationChange(ctx, db.CreateNotificationChangeParams{
		NotificationObjectID: obj.ID,
		ActorID:              actorID,
	})
	if err != nil {
		return fmt.Errorf("failed to create notification change: %w", err)
	}

	for _, notifierID := range notifierIDs {
		if notifierID == actorID {
			continue
		}

		_, err = qtx.CreateNotification(ctx, db.CreateNotificationParams{
			NotificationObjectID: obj.ID,
			NotifierID:           notifierID,
		})
		if err != nil {
			return fmt.Errorf("failed to create notification for %s: %w", notifierID, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *NotificationService) GetUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int64) ([]db.GetUserNotificationsRow, error) {
	return s.db.GetUserNotifications(ctx, db.GetUserNotificationsParams{
		NotifierID: userID,
		Limit:      limit,
		Offset:     offset,
	})
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID, notificationID uuid.UUID) (db.Notification, error) {
	return s.db.MarkNotificationAsRead(ctx, db.MarkNotificationAsReadParams{
		ID:         notificationID,
		NotifierID: userID,
	})
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	return s.db.MarkAllNotificationsAsRead(ctx, userID)
}

func (s *NotificationService) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.db.CountUserNotifications(ctx, userID)
}

func (s *NotificationService) GetTotalCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.db.CountAllUserNotifications(ctx, userID)
}
