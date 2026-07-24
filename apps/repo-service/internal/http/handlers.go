package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	serviceconfig "codeatlas/apps/repo-service/internal/config"
	"codeatlas/apps/repo-service/internal/integrations"
	"codeatlas/apps/repo-service/internal/repos"
	"codeatlas/apps/repo-service/internal/repository"
	"codeatlas/packages/events"
	sharedgithub "codeatlas/packages/github"
	"codeatlas/packages/kafka"
)

type Handler struct {
	config           serviceconfig.Config
	logger           *slog.Logger
	repositoryRepo   *repository.RepositoryRepository
	installationRepo *repository.InstallationRepository
	tokenManager     *TokenManager
	githubApp        *integrations.GitHubApp
	producer         kafka.Producer
}

func NewHandler(
	config serviceconfig.Config,
	logger *slog.Logger,
	repositoryRepo *repository.RepositoryRepository,
	installationRepo *repository.InstallationRepository,
	tokenManager *TokenManager,
	githubApp *integrations.GitHubApp,
	producer kafka.Producer,
) *Handler {
	return &Handler{
		config:           config,
		logger:           logger,
		repositoryRepo:   repositoryRepo,
		installationRepo: installationRepo,
		tokenManager:     tokenManager,
		githubApp:        githubApp,
		producer:         producer,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/repos", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleRepositories)))
	mux.Handle("/repos/sync-status", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleRepositoriesSyncStatus)))
	mux.Handle("/repos/", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleRepositoryByID)))
	mux.Handle("/integrations/github/install", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleGitHubInstallURL)))
	mux.HandleFunc("/integrations/github/setup", h.handleGitHubSetup)
	mux.Handle("/integrations/github/installations", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleGitHubInstallations)))
	mux.Handle("/integrations/github/installations/claim", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleClaimInstallation)))
	mux.Handle("/integrations/github/installations/", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleGitHubInstallationRoutes)))
}

func (h *Handler) handleRepositoriesSyncStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	userID, ok := CurrentUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in token",
		})
		return
	}

	statuses, err := h.repositoryRepo.ListLatestSyncStatusForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("list latest sync status for user", "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list repository sync status",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"repositories": statuses,
	})
}

type claimInstallationRequest struct {
	InstallationID int64 `json:"installation_id"`
}

type connectInstallationRepositoryRequest struct {
	GitHubRepoID int64 `json:"github_repo_id"`
}

type createSyncRunRequest struct {
	SyncType string `json:"sync_type"`
}

func (h *Handler) handleGitHubInstallURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	installURL, err := h.githubApp.InstallationURL()
	if err != nil {
		h.logger.Error("build github app installation url", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "github app slug is not configured",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"install_url": installURL,
	})
}

func (h *Handler) handleGitHubSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	installationIDValue := strings.TrimSpace(r.URL.Query().Get("installation_id"))
	if installationIDValue == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing installation_id",
		})
		return
	}

	installationID, err := parseInt64(installationIDValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid installation_id",
		})
		return
	}

	var setupAction *string
	if value := strings.TrimSpace(r.URL.Query().Get("setup_action")); value != "" {
		setupAction = &value
	}

	installation, err := h.installationRepo.UpsertFromSetupCallback(r.Context(), installationID, setupAction)
	if err != nil {
		h.logger.Error("upsert installation from setup callback", "error", err, "installation_id", installationID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to record installation callback",
		})
		return
	}

	if strings.TrimSpace(h.config.FrontendGitHubSetupRedirectURL) != "" {
		redirectURL, err := url.Parse(h.config.FrontendGitHubSetupRedirectURL)
		if err != nil {
			h.logger.Error("parse frontend github setup redirect url", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "invalid frontend setup redirect configuration",
			})
			return
		}

		query := redirectURL.Query()
		query.Set("installation_id", installationIDValue)
		if setupAction != nil {
			query.Set("setup_action", *setupAction)
		}
		redirectURL.RawQuery = query.Encode()

		http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"installation": installation,
	})
}

func (h *Handler) handleClaimInstallation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	userID, ok := CurrentUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in token",
		})
		return
	}

	var payload claimInstallationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if payload.InstallationID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "installation_id is required",
		})
		return
	}

	installation, err := h.installationRepo.ClaimInstallation(r.Context(), payload.InstallationID, userID)
	if err != nil {
		h.logger.Error("claim installation", "error", err, "installation_id", payload.InstallationID, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to claim installation",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"installation": installation,
	})
}

func (h *Handler) handleGitHubInstallations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	userID, ok := CurrentUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in token",
		})
		return
	}

	installations, err := h.installationRepo.ListInstallationsForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("list installations for user", "error", err, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list installations",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"installations": installations,
	})
}

func (h *Handler) handleGitHubInstallationRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/integrations/github/installations/")
	path = strings.TrimSpace(path)
	if path == "" {
		h.handleGitHubInstallations(w, r)
		return
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[1] != "repositories" {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "route not found",
		})
		return
	}

	installationID, err := parseInt64(parts[0])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid installation id",
		})
		return
	}

	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			h.handleInstallationRepositories(w, r, installationID)
		default:
			w.Header().Set("Allow", "GET, OPTIONS")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
		}
		return
	}

	if len(parts) == 3 && parts[2] == "connect" {
		switch r.Method {
		case http.MethodPost:
			h.handleConnectInstallationRepository(w, r, installationID)
		default:
			w.Header().Set("Allow", "POST, OPTIONS")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
		}
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "route not found",
	})
}

func (h *Handler) handleConnectInstallationRepository(w http.ResponseWriter, r *http.Request, installationID int64) {
	userID, ok := CurrentUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in token",
		})
		return
	}

	installation, err := h.installationRepo.FindClaimedInstallationForUser(r.Context(), installationID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrInstallationNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "installation not found for user",
			})
			return
		}

		h.logger.Error("find claimed installation for connect", "error", err, "installation_id", installationID, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to resolve installation ownership",
		})
		return
	}

	var payload connectInstallationRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if payload.GitHubRepoID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "github_repo_id is required",
		})
		return
	}

	githubRepositories, err := h.githubApp.ListInstallationRepositories(r.Context(), installation.InstallationID)
	if err != nil {
		h.logger.Error("list github installation repositories for connect", "error", err, "installation_id", installation.InstallationID)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "failed to list repositories from github app installation",
		})
		return
	}

	var selectedRepo *integrationsRepository
	for i := range githubRepositories {
		repo := githubRepositories[i]
		if repo.ID == payload.GitHubRepoID {
			selectedRepo = &integrationsRepository{
				ID:            repo.ID,
				Name:          repo.Name,
				FullName:      repo.FullName,
				Private:       repo.Private,
				DefaultBranch: repo.DefaultBranch,
				OwnerLogin:    repo.Owner.Login,
			}
			break
		}
	}

	if selectedRepo == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "repository not found in installation",
		})
		return
	}

	result, err := h.repositoryRepo.ConnectRepository(r.Context(), repos.ConnectRepositoryInput{
		UserID:         userID,
		GitHubRepoID:   selectedRepo.ID,
		Owner:          selectedRepo.OwnerLogin,
		Name:           selectedRepo.Name,
		DefaultBranch:  selectedRepo.DefaultBranch,
		IsPrivate:      selectedRepo.Private,
		InstallationID: &installation.InstallationID,
	})
	if err != nil {
		h.logger.Error("connect repository from installation", "error", err, "installation_id", installation.InstallationID, "github_repo_id", payload.GitHubRepoID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to connect repository",
		})
		return
	}

	statusCode := http.StatusOK
	if result.ConnectionStatus == repos.ConnectionStatusCreated {
		statusCode = http.StatusCreated
	}

	if result.Repository.WebhookID == nil {
		webhookURL := strings.TrimSpace(h.config.GitHubWebhookURL)
		if webhookURL == "" {
			h.logger.Error(
				"github webhook url is not configured",
				"repository_id", result.Repository.ID,
				"github_repo_id", result.Repository.GitHubRepoID,
			)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "github webhook url is not configured",
			})
			return
		}

		normalizedWebhookURL := sharedgithub.NormalizeWebhookURL(webhookURL)
		webhooks, err := h.githubApp.ListRepositoryWebhooks(r.Context(), installation.InstallationID, selectedRepo.OwnerLogin, selectedRepo.Name)
		if err != nil {
			h.logger.Error(
				"list github repository webhooks",
				"error", err,
				"repository_id", result.Repository.ID,
				"github_repo_id", result.Repository.GitHubRepoID,
				"installation_id", installation.InstallationID,
			)
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": "failed to inspect github repository webhooks",
			})
			return
		}

		var webhookID int64
		for i := range webhooks {
			if sharedgithub.NormalizeWebhookURL(webhooks[i].Config.URL) == normalizedWebhookURL {
				webhookID = webhooks[i].ID
				break
			}
		}

		if webhookID == 0 {
			webhook, err := h.githubApp.CreateRepositoryWebhook(r.Context(), installation.InstallationID, selectedRepo.OwnerLogin, selectedRepo.Name, sharedgithub.RepositoryWebhookInput{
				URL:         webhookURL,
				Secret:      h.config.GitHubWebhookSecret,
				ContentType: "json",
				Events:      []string{"push", "pull_request"},
				Active:      true,
			})
			if err != nil {
				h.logger.Error(
					"create github repository webhook",
					"error", err,
					"repository_id", result.Repository.ID,
					"github_repo_id", result.Repository.GitHubRepoID,
					"installation_id", installation.InstallationID,
				)
				writeJSON(w, http.StatusBadGateway, map[string]string{
					"error": "failed to create github repository webhook",
				})
				return
			}

			webhookID = webhook.ID
		}

		updatedRepository, err := h.repositoryRepo.UpdateRepositoryWebhookID(r.Context(), userID, result.Repository.ID, webhookID)
		if err != nil {
			h.logger.Error(
				"persist repository webhook id",
				"error", err,
				"repository_id", result.Repository.ID,
				"github_repo_id", result.Repository.GitHubRepoID,
				"webhook_id", webhookID,
			)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to persist repository webhook metadata",
			})
			return
		}

		result.Repository = updatedRepository
	}

	writeJSON(w, statusCode, result)
}

type integrationsRepository struct {
	ID            int64
	Name          string
	FullName      string
	Private       bool
	DefaultBranch string
	OwnerLogin    string
}

func (h *Handler) handleInstallationRepositories(w http.ResponseWriter, r *http.Request, installationID int64) {
	userID, ok := CurrentUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in token",
		})
		return
	}

	installation, err := h.installationRepo.FindClaimedInstallationForUser(r.Context(), installationID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrInstallationNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "installation not found for user",
			})
			return
		}

		h.logger.Error("find claimed installation for repositories", "error", err, "installation_id", installationID, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to resolve installation ownership",
		})
		return
	}

	repositories, err := h.githubApp.ListInstallationRepositories(r.Context(), installation.InstallationID)
	if err != nil {
		h.logger.Error("list github installation repositories", "error", err, "installation_id", installation.InstallationID)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "failed to list repositories from github app installation",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"installation": installation,
		"repositories": repositories,
	})
}

func (h *Handler) handleRepositories(w http.ResponseWriter, r *http.Request) {
	userID, ok := CurrentUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in token",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		repositories, err := h.repositoryRepo.ListRepositoriesForUser(r.Context(), userID)
		if err != nil {
			h.logger.Error("list repositories", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to list repositories",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"repositories": repositories,
		})
	case http.MethodPost:
		var input repos.ConnectRepositoryInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		input.UserID = userID
		if strings.TrimSpace(input.Owner) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.DefaultBranch) == "" || input.GitHubRepoID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "github_repo_id, owner, name, and default_branch are required",
			})
			return
		}

		result, err := h.repositoryRepo.ConnectRepository(r.Context(), input)
		if err != nil {
			h.logger.Error("connect repository", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to connect repository",
			})
			return
		}

		statusCode := http.StatusOK
		if result.ConnectionStatus == repos.ConnectionStatusCreated {
			statusCode = http.StatusCreated
		}

		writeJSON(w, statusCode, result)
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

func (h *Handler) handleRepositoryByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := CurrentUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in token",
		})
		return
	}

	path := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/repos/"))
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing repository id",
		})
		return
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	repositoryID, err := parseInt64(parts[0])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid repository id",
		})
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, OPTIONS")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		repo, err := h.repositoryRepo.FindRepositoryForUser(r.Context(), userID, repositoryID)
		if err != nil {
			h.logger.Error("find repository", "repository_id", repositoryID, "error", err)
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"repository": repo,
		})
		return
	}

	switch parts[1] {
	case "dashboard":
		h.handleRepositoryDashboard(w, r, userID, repositoryID)
	case "co-change":
		h.handleRepositoryCoChange(w, r, userID, repositoryID)
	case "hotspots":
		h.handleRepositoryHotspots(w, r, userID, repositoryID)
	case "modules":
		h.handleRepositoryModules(w, r, userID, repositoryID, parts[2:])
	case "sync":
		h.handleRepositorySync(w, r, userID, repositoryID)
	case "sync-runs":
		h.handleRepositorySyncRuns(w, r, userID, repositoryID, parts[2:])
	case "contributors":
		h.handleRepositoryContributors(w, r, userID, repositoryID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "route not found",
		})
	}
}

func (h *Handler) handleRepositoryModules(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64, parts []string) {
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "missing module analytics route",
		})
		return
	}

	if moduleID, err := parseInt64(strings.TrimSpace(parts[0])); err == nil {
		h.handleRepositoryModuleDetail(w, r, userID, repositoryID, moduleID)
		return
	}

	switch parts[0] {
	case "ownership":
		h.handleRepositoryModuleOwnership(w, r, userID, repositoryID)
	case "expertise":
		h.handleRepositoryModuleExpertise(w, r, userID, repositoryID)
	case "bus-factor":
		h.handleRepositoryModuleBusFactor(w, r, userID, repositoryID)
	case "co-change":
		h.handleRepositoryModuleCoChange(w, r, userID, repositoryID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "route not found",
		})
	}
}

func (h *Handler) handleRepositoryModuleDetail(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64, moduleID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	module, err := h.repositoryRepo.BuildModuleDetailForRepository(r.Context(), userID, repositoryID, moduleID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "module not found",
			})
			return
		}

		h.logger.Error("build module detail", "repository_id", repositoryID, "module_id", moduleID, "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to build module detail",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"module": module,
	})
}

func (h *Handler) handleRepositoryModuleOwnership(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	modules, err := h.repositoryRepo.ListModuleOwnershipForRepository(r.Context(), userID, repositoryID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		h.logger.Error("list module ownership", "repository_id", repositoryID, "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list module ownership",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"modules": modules,
	})
}

func (h *Handler) handleRepositoryModuleExpertise(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	modules, err := h.repositoryRepo.ListModuleExpertiseForRepository(r.Context(), userID, repositoryID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		h.logger.Error("list module expertise", "repository_id", repositoryID, "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list module expertise",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"modules": modules,
	})
}

func (h *Handler) handleRepositoryModuleBusFactor(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	modules, err := h.repositoryRepo.ListModuleBusFactorForRepository(r.Context(), userID, repositoryID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		h.logger.Error("list module bus factor", "repository_id", repositoryID, "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list module bus factor",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"modules": modules,
	})
}

func (h *Handler) handleRepositoryDashboard(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	dashboard, err := h.repositoryRepo.BuildDashboardForRepository(r.Context(), userID, repositoryID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		h.logger.Error("build repository dashboard", "repository_id", repositoryID, "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to build repository dashboard",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dashboard": dashboard,
	})
}

func (h *Handler) handleRepositoryHotspots(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := parseInt64(rawLimit)
		if err != nil || parsedLimit <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid limit",
			})
			return
		}

		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = int(parsedLimit)
	}

	hotspots, err := h.repositoryRepo.ListHotspotsForRepository(r.Context(), userID, repositoryID, limit)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		h.logger.Error("list repository hotspots", "repository_id", repositoryID, "user_id", userID, "limit", limit, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list repository hotspots",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"hotspots": hotspots,
	})
}

func (h *Handler) handleRepositoryCoChange(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := parseInt64(rawLimit)
		if err != nil || parsedLimit <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid limit",
			})
			return
		}

		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = int(parsedLimit)
	}

	pathFilter := strings.TrimSpace(r.URL.Query().Get("path"))

	pairs, err := h.repositoryRepo.ListCoChangeForRepository(r.Context(), userID, repositoryID, limit, pathFilter)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		h.logger.Error("list repository co-change", "repository_id", repositoryID, "user_id", userID, "limit", limit, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list repository co-change",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"focused_path": pathFilter,
		"co_changes":   pairs,
	})
}

func (h *Handler) handleRepositoryModuleCoChange(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := parseInt64(rawLimit)
		if err != nil || parsedLimit <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid limit",
			})
			return
		}

		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = int(parsedLimit)
	}

	pairs, err := h.repositoryRepo.ListModuleCoChangeForRepository(r.Context(), userID, repositoryID, limit)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}

		h.logger.Error("list module co-change", "repository_id", repositoryID, "user_id", userID, "limit", limit, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list module co-change",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"module_co_changes": pairs,
	})
}

func (h *Handler) handleRepositorySync(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	payload := createSyncRunRequest{SyncType: repos.SyncTypeInitial}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}
	if strings.TrimSpace(payload.SyncType) == "" {
		payload.SyncType = repos.SyncTypeInitial
	}

	requestStartedAt := time.Now()
	h.logger.Debug(
		"queue repository sync requested",
		"repository_id", repositoryID,
		"user_id", userID,
		"sync_type", payload.SyncType,
	)

	result, err := h.repositoryRepo.CreateSyncRunForRepository(r.Context(), userID, repositoryID, payload.SyncType)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "repository not found",
			})
			return
		}
		h.logger.Error("create sync run", "repository_id", repositoryID, "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create sync run",
		})
		return
	}

	statusCode := http.StatusCreated
	if result.RequestStatus != repos.SyncRequestStatusQueued {
		statusCode = http.StatusOK
	}

	h.logger.Info(
		"repository sync queued result",
		"repository_id", repositoryID,
		"user_id", userID,
		"sync_run_id", result.SyncRun.ID,
		"sync_type", result.SyncRun.SyncType,
		"request_status", result.RequestStatus,
		"duration_ms", time.Since(requestStartedAt).Milliseconds(),
	)

	writeJSON(w, statusCode, result)

	if result.RequestStatus != repos.SyncRequestStatusQueued {
		return
	}

	event := events.RepositorySyncRequested{
		SyncRunID:         result.SyncRun.ID,
		SyncRunCreatedAt:  result.SyncRun.CreatedAt,
		RepositoryID:      result.SyncRun.RepositoryID,
		SyncType:          result.SyncRun.SyncType,
		RequestedByUserID: userID,
		RequestedAt:       result.SyncRun.CreatedAt,
	}

	if err := h.producer.Publish(
		r.Context(),
		h.config.RepositorySyncRequestedTopic,
		fmt.Sprintf("%d", result.SyncRun.RepositoryID),
		event,
	); err != nil {
		h.logger.Error(
			"publish repository sync requested event",
			"repository_id", repositoryID,
			"sync_run_id", result.SyncRun.ID,
			"error", err,
		)
		return
	}

	h.logger.Info(
		"published repository sync requested event",
		"repository_id", repositoryID,
		"sync_run_id", result.SyncRun.ID,
		"topic", h.config.RepositorySyncRequestedTopic,
		"key", result.SyncRun.RepositoryID,
	)
}

func (h *Handler) handleRepositorySyncRuns(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64, tail []string) {
	if len(tail) == 0 {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, OPTIONS")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		runs, err := h.repositoryRepo.ListSyncRunsForRepository(r.Context(), userID, repositoryID)
		if err != nil {
			h.logger.Error("list sync runs", "repository_id", repositoryID, "user_id", userID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to list sync runs",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"sync_runs": runs,
		})
		return
	}

	if len(tail) == 1 {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, OPTIONS")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}

		runID, err := parseInt64(tail[0])
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid sync run id",
			})
			return
		}

		run, err := h.repositoryRepo.FindSyncRunForRepository(r.Context(), userID, repositoryID, runID)
		if err != nil {
			if errors.Is(err, repository.ErrRepositoryNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{
					"error": "sync run not found",
				})
				return
			}
			h.logger.Error("find sync run", "repository_id", repositoryID, "run_id", runID, "user_id", userID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to find sync run",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"sync_run": run,
		})
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "route not found",
	})
}

func (h *Handler) handleRepositoryContributors(w http.ResponseWriter, r *http.Request, userID int64, repositoryID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	contributors, err := h.repositoryRepo.ListContributorsForRepository(r.Context(), userID, repositoryID)
	if err != nil {
		h.logger.Error("list contributors", "repository_id", repositoryID, "user_id", userID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list contributors",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"contributors": contributors,
	})
}
