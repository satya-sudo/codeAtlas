package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	serviceconfig "codeatlas/apps/repo-service/internal/config"
	"codeatlas/apps/repo-service/internal/integrations"
	"codeatlas/apps/repo-service/internal/repos"
	"codeatlas/apps/repo-service/internal/repository"
)

type Handler struct {
	config           serviceconfig.Config
	logger           *slog.Logger
	repositoryRepo   *repository.RepositoryRepository
	installationRepo *repository.InstallationRepository
	tokenManager     *TokenManager
	githubApp        *integrations.GitHubApp
}

func NewHandler(
	config serviceconfig.Config,
	logger *slog.Logger,
	repositoryRepo *repository.RepositoryRepository,
	installationRepo *repository.InstallationRepository,
	tokenManager *TokenManager,
	githubApp *integrations.GitHubApp,
) *Handler {
	return &Handler{
		config:           config,
		logger:           logger,
		repositoryRepo:   repositoryRepo,
		installationRepo: installationRepo,
		tokenManager:     tokenManager,
		githubApp:        githubApp,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/repos", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleRepositories)))
	mux.Handle("/repos/", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleRepositoryByID)))
	mux.Handle("/integrations/github/install", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleGitHubInstallURL)))
	mux.HandleFunc("/integrations/github/setup", h.handleGitHubSetup)
	mux.Handle("/integrations/github/installations", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleGitHubInstallations)))
	mux.Handle("/integrations/github/installations/claim", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleClaimInstallation)))
	mux.Handle("/integrations/github/installations/", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleGitHubInstallationRoutes)))
}

type claimInstallationRequest struct {
	InstallationID int64 `json:"installation_id"`
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
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing installation route",
		})
		return
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[1] != "repositories" {
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

	switch r.Method {
	case http.MethodGet:
		h.handleInstallationRepositories(w, r, installationID)
	default:
		w.Header().Set("Allow", "GET, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
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

		repo, err := h.repositoryRepo.ConnectRepository(r.Context(), input)
		if err != nil {
			h.logger.Error("connect repository", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to connect repository",
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"repository": repo,
		})
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

func (h *Handler) handleRepositoryByID(w http.ResponseWriter, r *http.Request) {
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

	repositoryIDValue := strings.TrimPrefix(r.URL.Path, "/repos/")
	repositoryIDValue = strings.TrimSpace(repositoryIDValue)
	if repositoryIDValue == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing repository id",
		})
		return
	}

	repositoryID, err := parseInt64(repositoryIDValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid repository id",
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
}
