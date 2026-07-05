package httpapi

import (
	"net/http"
	"strings"
)

func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	allowed := parseAllowedOrigins(allowedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := resolveAllowedOrigin(r.Header.Get("Origin"), allowed); origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseAllowedOrigins(value string) map[string]struct{} {
	items := map[string]struct{}{}
	for _, part := range strings.Split(value, ",") {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		items[origin] = struct{}{}
	}
	return items
}

func resolveAllowedOrigin(origin string, allowed map[string]struct{}) string {
	if origin == "" {
		return ""
	}
	if len(allowed) == 0 {
		return origin
	}
	if _, ok := allowed[origin]; ok {
		return origin
	}
	return ""
}
