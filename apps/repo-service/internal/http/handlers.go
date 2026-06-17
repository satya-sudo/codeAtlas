package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"codeatlas/apps/repo-service/internal/repos"
	"codeatlas/apps/repo-service/internal/repository"
)

type Handler struct {
	logger         *slog.Logger
	repositoryRepo *repository.RepositoryRepository
	tokenManager   *TokenManager
}

func NewHandler(logger *slog.Logger, repositoryRepo *repository.RepositoryRepository, tokenManager *TokenManager) *Handler {
	return &Handler{
		logger:         logger,
		repositoryRepo: repositoryRepo,
		tokenManager:   tokenManager,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/repos", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleRepositories)))
	mux.Handle("/repos/", AuthMiddleware(h.tokenManager)(http.HandlerFunc(h.handleRepositoryByID)))
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
