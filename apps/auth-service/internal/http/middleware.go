package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"codeatlas/apps/auth-service/internal/repository"
	"codeatlas/apps/auth-service/internal/tokens"
	"codeatlas/apps/auth-service/internal/users"
)

type contextKey string

const userContextKey contextKey = "authenticated_user"

func AuthMiddleware(tokenManager *tokens.Manager, userRepo *repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "missing bearer token",
				})
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			claims, err := tokenManager.Parse(token)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "invalid token",
				})
				return
			}

			user, err := userRepo.FindByID(r.Context(), claims.Subject)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "user not found",
				})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CurrentUser(r *http.Request) (users.User, bool) {
	user, ok := r.Context().Value(userContextKey).(users.User)
	return user, ok
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
