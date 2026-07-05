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

	writeJSON(w, statusCode, result)
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
