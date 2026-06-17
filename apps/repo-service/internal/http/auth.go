package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type contextKey string

const userContextKey contextKey = "authenticated_user_id"

type Claims struct {
	Subject  int64  `json:"sub"`
	GitHubID int64  `json:"github_id"`
	Username string `json:"username"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
}

type TokenManager struct {
	secret []byte
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
	}
}

func (m *TokenManager) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("invalid token format")
	}

	unsignedToken := parts[0] + "." + parts[1]
	expectedSignature := m.sign(unsignedToken)
	if !hmac.Equal([]byte(expectedSignature), []byte(parts[2])) {
		return Claims{}, fmt.Errorf("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("decode token payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, fmt.Errorf("unmarshal token claims: %w", err)
	}

	if time.Now().UTC().Unix() > claims.Expiry {
		return Claims{}, fmt.Errorf("token expired")
	}

	return claims, nil
}

func (m *TokenManager) sign(data string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func AuthMiddleware(tokenManager *TokenManager) func(http.Handler) http.Handler {
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

			ctx := context.WithValue(r.Context(), userContextKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CurrentUserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(userContextKey).(int64)
	return userID, ok
}

func CORSMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
