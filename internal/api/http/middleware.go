package http

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"nehuang-iot-server/internal/auth"
	"nehuang-iot-server/internal/domain"
)

// contextKey is an unexported integer type used as context keys to avoid collisions.
type contextKey int

const claimsKey contextKey = 1

// ClaimsFromContext extracts the auth.Claims injected by authMiddleware.
// Returns (Claims{}, false) if no claims are present.
func ClaimsFromContext(ctx context.Context) (auth.Claims, bool) {
	v, ok := ctx.Value(claimsKey).(auth.Claims)
	return v, ok
}

// loggingMiddleware logs method, path, and elapsed time for every request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// authMiddleware parses the "Authorization: Bearer <token>" header, verifies the
// HS256 signature and expiry, and stores the resulting Claims in the request context.
// Responds with 401 if the token is absent, malformed, or expired.
func authMiddleware(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			writeCode(w, http.StatusUnauthorized, 401, "missing or malformed authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(raw, "Bearer ")
		claims, err := auth.Parse(tokenStr, secret)
		if err != nil {
			if err == auth.ErrTokenExpired {
				writeCode(w, http.StatusUnauthorized, 4011, "token expired")
				return
			}
			writeCode(w, http.StatusUnauthorized, 4012, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// roleMiddleware builds a middleware that enforces that the authenticated caller's
// role is one of allowedRoles. Must be chained after authMiddleware.
// Responds with 403 if the role is insufficient.
func roleMiddleware(allowedRoles ...domain.Role) func(http.Handler) http.Handler {
	allowed := make(map[domain.Role]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeCode(w, http.StatusForbidden, 403, "no claims in context")
				return
			}
			if _, permitted := allowed[claims.Role]; !permitted {
				writeCode(w, http.StatusForbidden, 4031, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeCode sends a unified error envelope: {"code": <code>, "message": "<message>"}.
func writeCode(w http.ResponseWriter, httpStatus, code int, message string) {
	writeJSON(w, httpStatus, map[string]any{
		"code":    code,
		"message": message,
	})
}

// corsMiddleware adds CORS headers to allow cross-origin browser requests.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
