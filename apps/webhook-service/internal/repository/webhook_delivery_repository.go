package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	WebhookDeliveryStatusReceived  = "received"
	WebhookDeliveryStatusPublished = "published"
	WebhookDeliveryStatusIgnored   = "ignored"
	WebhookDeliveryStatusFailed    = "failed"
)

type WebhookDelivery struct {
	ID             int64
	DeliveryID     string
	Event          string
	Action         *string
	RepositoryID   *int64
	InstallationID *int64
	Status         string
	ErrorMessage   *string
	PayloadJSON    []byte
	ReceivedAt     time.Time
	ProcessedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateWebhookDeliveryInput struct {
	DeliveryID     string
	Event          string
	Action         *string
	RepositoryID   *int64
	InstallationID *int64
	Status         string
	PayloadJSON    []byte
	ReceivedAt     time.Time
}

var ErrWebhookDeliveryNotFound = errors.New("webhook delivery not found")

type WebhookDeliveryRepository struct {
	db *pgxpool.Pool
}

func NewWebhookDeliveryRepository(db *pgxpool.Pool) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{db: db}
}

func (r *WebhookDeliveryRepository) CreateIfNotExists(ctx context.Context, input CreateWebhookDeliveryInput) (WebhookDelivery, bool, error) {
	const query = `
		INSERT INTO github_webhook_deliveries (
			delivery_id,
			event,
			action,
			repository_id,
			installation_id,
			status,
			payload_json,
			received_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
		ON CONFLICT (delivery_id) DO NOTHING
		RETURNING
			id,
			delivery_id,
			event,
			action,
			repository_id,
			installation_id,
			status,
			error_message,
			payload_json,
			received_at,
			processed_at,
			created_at,
			updated_at
	`

	var delivery WebhookDelivery
	err := r.db.QueryRow(
		ctx,
		query,
		input.DeliveryID,
		input.Event,
		input.Action,
		input.RepositoryID,
		input.InstallationID,
		input.Status,
		json.RawMessage(input.PayloadJSON),
		input.ReceivedAt,
	).Scan(
		&delivery.ID,
		&delivery.DeliveryID,
		&delivery.Event,
		&delivery.Action,
		&delivery.RepositoryID,
		&delivery.InstallationID,
		&delivery.Status,
		&delivery.ErrorMessage,
		&delivery.PayloadJSON,
		&delivery.ReceivedAt,
		&delivery.ProcessedAt,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)
	if err == nil {
		return delivery, true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		delivery, err = r.FindByDeliveryID(ctx, input.DeliveryID)
		if err != nil {
			return WebhookDelivery{}, false, err
		}
		return delivery, false, nil
	}

	return WebhookDelivery{}, false, fmt.Errorf("create webhook delivery: %w", err)
}

func (r *WebhookDeliveryRepository) FindByDeliveryID(ctx context.Context, deliveryID string) (WebhookDelivery, error) {
	const query = `
		SELECT
			id,
			delivery_id,
			event,
			action,
			repository_id,
			installation_id,
			status,
			error_message,
			payload_json,
			received_at,
			processed_at,
			created_at,
			updated_at
		FROM github_webhook_deliveries
		WHERE delivery_id = $1
	`

	var delivery WebhookDelivery
	err := r.db.QueryRow(ctx, query, deliveryID).Scan(
		&delivery.ID,
		&delivery.DeliveryID,
		&delivery.Event,
		&delivery.Action,
		&delivery.RepositoryID,
		&delivery.InstallationID,
		&delivery.Status,
		&delivery.ErrorMessage,
		&delivery.PayloadJSON,
		&delivery.ReceivedAt,
		&delivery.ProcessedAt,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WebhookDelivery{}, ErrWebhookDeliveryNotFound
		}
		return WebhookDelivery{}, fmt.Errorf("find webhook delivery by delivery id: %w", err)
	}

	return delivery, nil
}

func (r *WebhookDeliveryRepository) MarkStatus(ctx context.Context, deliveryID string, status string, errorMessage *string) error {
	const query = `
		UPDATE github_webhook_deliveries
		SET status = $2,
			error_message = $3,
			processed_at = NOW(),
			updated_at = NOW()
		WHERE delivery_id = $1
	`

	tag, err := r.db.Exec(ctx, query, deliveryID, status, errorMessage)
	if err != nil {
		return fmt.Errorf("mark webhook delivery status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWebhookDeliveryNotFound
	}

	return nil
}
