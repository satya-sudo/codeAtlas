package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	serviceconfig "codeatlas/apps/auth-service/internal/config"
	"codeatlas/apps/auth-service/internal/oauth"
	"codeatlas/apps/auth-service/internal/repository"
	"codeatlas/apps/auth-service/internal/tokens"
)

type Handler struct {
	config       serviceconfig.Config
	logger       *slog.Logger
	githubClient *oauth.GitHubClient
	userRepo     *repository.UserRepository
	tokenManager *tokens.Manager
}

func NewHandler(
	cfg serviceconfig.Config,
	logger *slog.Logger,
	githubClient *oauth.GitHubClient,
	userRepo *repository.UserRepository,
	tokenManager *tokens.Manager,
) *Handler {
	return &Handler{
		config:       cfg,
		logger:       logger,
		githubClient: githubClient,
		userRepo:     userRepo,
		tokenManager: tokenManager,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/auth/github/login", h.handleGitHubLogin)
	mux.HandleFunc("/auth/github/callback", h.handleGitHubCallback)
	mux.Handle("/auth/me", AuthMiddleware(h.tokenManager, h.userRepo)(http.HandlerFunc(h.handleMe)))
}

func (h *Handler) handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateStateToken()
	if err != nil {
		h.logger.Error("generate oauth state", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to initialize login",
		})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.config.StateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
		Expires:  time.Now().Add(h.config.StateCookieTTL),
	})

	http.Redirect(w, r, h.githubClient.BuildAuthorizeURL(state), http.StatusTemporaryRedirect)
}

func (h *Handler) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing github callback parameters",
		})
		return
	}

	stateCookie, err := r.Cookie(h.config.StateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != state {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid oauth state",
		})
		return
	}

	accessToken, err := h.githubClient.ExchangeCode(r.Context(), code)
	if err != nil {
		h.logger.Error("exchange github code", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "failed to exchange github code",
		})
		return
	}

	githubUser, err := h.githubClient.FetchUser(r.Context(), accessToken)
	if err != nil {
		h.logger.Error("fetch github user", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "failed to fetch github user",
		})
		return
	}

	user, err := h.userRepo.UpsertGitHubUser(r.Context(), githubUser.ID, githubUser.Login, githubUser.AvatarURL)
	if err != nil {
		h.logger.Error("upsert local user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to persist user",
		})
		return
	}

	token, err := h.tokenManager.Issue(user)
	if err != nil {
		h.logger.Error("issue jwt", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to issue token",
		})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.config.StateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	if h.config.FrontendRedirectURL != "" {
		redirectURL, err := url.Parse(h.config.FrontendRedirectURL)
		if err != nil {
			h.logger.Error("parse frontend redirect url", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "invalid frontend redirect configuration",
			})
			return
		}

		query := redirectURL.Query()
		query.Set("token", token)
		redirectURL.RawQuery = query.Encode()

		http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "user not found in context",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
	})
}

func generateStateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
