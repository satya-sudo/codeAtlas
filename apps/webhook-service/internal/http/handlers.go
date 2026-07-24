package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	serviceconfig "codeatlas/apps/webhook-service/internal/config"
	"codeatlas/apps/webhook-service/internal/repository"
	"codeatlas/packages/events"
	"codeatlas/packages/kafka"
)

const maxGitHubWebhookBodyBytes = 5 << 20

type Handler struct {
	config     serviceconfig.Config
	logger     *slog.Logger
	producer   kafka.Producer
	deliveries *repository.WebhookDeliveryRepository
}

func NewHandler(config serviceconfig.Config, logger *slog.Logger, producer kafka.Producer, deliveries *repository.WebhookDeliveryRepository) *Handler {
	return &Handler{
		config:     config,
		logger:     logger,
		producer:   producer,
		deliveries: deliveries,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/webhooks/github", h.handleGitHubWebhook)
	mux.HandleFunc("/webhooks/github/", h.handleGitHubWebhook)
}

type githubWebhookEnvelope struct {
	Action       string `json:"action"`
	Ref          string `json:"ref"`
	Before       string `json:"before"`
	After        string `json:"after"`
	Installation *struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
	HeadCommit *struct {
		ID string `json:"id"`
	} `json:"head_commit"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func (h *Handler) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGitHubWebhookBodyBytes))
	if err != nil {
		h.logger.Error("read github webhook body", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "failed to read request body",
		})
		return
	}

	if !verifyGitHubSignature(h.config.GitHubWebhookSecret, r.Header.Get("X-Hub-Signature-256"), body) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid webhook signature",
		})
		return
	}

	eventName := strings.TrimSpace(r.Header.Get("X-GitHub-Event"))
	deliveryID := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	if eventName == "" || deliveryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing github webhook headers",
		})
		return
	}

	if eventName == "ping" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "ok",
			"event":       eventName,
			"delivery_id": deliveryID,
		})
		return
	}

	var payload githubWebhookEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("decode github webhook payload", "error", err, "delivery_id", deliveryID, "event", eventName)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid github webhook payload",
		})
		return
	}

	var installationID *int64
	if payload.Installation != nil && payload.Installation.ID != 0 {
		installationID = &payload.Installation.ID
	}

	var repositoryID *int64
	if payload.Repository.ID != 0 {
		repositoryID = &payload.Repository.ID
	}

	var action *string
	if value := strings.TrimSpace(payload.Action); value != "" {
		action = &value
	}

	delivery, created, err := h.deliveries.CreateIfNotExists(r.Context(), repository.CreateWebhookDeliveryInput{
		DeliveryID:     deliveryID,
		Event:          eventName,
		Action:         action,
		RepositoryID:   repositoryID,
		InstallationID: installationID,
		Status:         repository.WebhookDeliveryStatusReceived,
		PayloadJSON:    body,
		ReceivedAt:     time.Now().UTC(),
	})
	if err != nil {
		h.logger.Error("store github webhook delivery", "error", err, "delivery_id", deliveryID, "event", eventName)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to persist webhook delivery",
		})
		return
	}

	if !created {
		h.logger.Info(
			"duplicate github webhook delivery ignored",
			"delivery_id", deliveryID,
			"event", eventName,
			"status", delivery.Status,
		)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "duplicate_ignored",
			"event":       eventName,
			"delivery_id": deliveryID,
		})
		return
	}

	if eventName != "push" {
		if err := h.deliveries.MarkStatus(r.Context(), deliveryID, repository.WebhookDeliveryStatusIgnored, nil); err != nil {
			h.logger.Error("mark github webhook delivery ignored", "error", err, "delivery_id", deliveryID, "event", eventName)
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":      "ignored",
			"event":       eventName,
			"delivery_id": deliveryID,
		})
		return
	}

	if payload.Repository.ID == 0 || strings.TrimSpace(payload.Repository.FullName) == "" {
		message := "missing repository information in webhook payload"
		if err := h.deliveries.MarkStatus(r.Context(), deliveryID, repository.WebhookDeliveryStatusFailed, &message); err != nil {
			h.logger.Error("mark github webhook delivery failed", "error", err, "delivery_id", deliveryID, "event", eventName)
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": message,
		})
		return
	}

	var headCommitSHA *string
	if payload.HeadCommit != nil && strings.TrimSpace(payload.HeadCommit.ID) != "" {
		headCommitSHA = &payload.HeadCommit.ID
	}

	normalized := events.GitHubPushReceived{
		DeliveryID:              deliveryID,
		Event:                   eventName,
		RepositoryID:            payload.Repository.ID,
		RepositoryFullName:      payload.Repository.FullName,
		RepositoryDefaultBranch: payload.Repository.DefaultBranch,
		InstallationID:          installationID,
		Ref:                     payload.Ref,
		BeforeSHA:               payload.Before,
		AfterSHA:                payload.After,
		HeadCommitSHA:           headCommitSHA,
		SenderLogin:             payload.Sender.Login,
		ReceivedAt:              time.Now().UTC(),
	}

	if err := h.producer.Publish(
		r.Context(),
		h.config.GitHubPushTopic,
		fmt.Sprintf("%d", normalized.RepositoryID),
		normalized,
	); err != nil {
		h.logger.Error(
			"publish github push event",
			"delivery_id", deliveryID,
			"repository_id", normalized.RepositoryID,
			"error", err,
		)
		message := "failed to publish webhook event"
		if markErr := h.deliveries.MarkStatus(r.Context(), deliveryID, repository.WebhookDeliveryStatusFailed, &message); markErr != nil {
			h.logger.Error("mark github webhook delivery failed", "error", markErr, "delivery_id", deliveryID)
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": message,
		})
		return
	}

	if err := h.deliveries.MarkStatus(r.Context(), deliveryID, repository.WebhookDeliveryStatusPublished, nil); err != nil {
		if !errors.Is(err, repository.ErrWebhookDeliveryNotFound) {
			h.logger.Error("mark github webhook delivery published", "error", err, "delivery_id", deliveryID)
		}
	}

	h.logger.Info(
		"published github push event",
		"delivery_id", deliveryID,
		"repository_id", normalized.RepositoryID,
		"repository_full_name", normalized.RepositoryFullName,
		"topic", h.config.GitHubPushTopic,
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":      "accepted",
		"event":       eventName,
		"delivery_id": deliveryID,
		"repository": map[string]any{
			"id":        normalized.RepositoryID,
			"full_name": normalized.RepositoryFullName,
		},
	})
}

func verifyGitHubSignature(secret string, signatureHeader string, body []byte) bool {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(signatureHeader) == "" {
		return false
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}

	providedSignature, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, prefix))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSignature := mac.Sum(nil)

	return hmac.Equal(providedSignature, expectedSignature)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
