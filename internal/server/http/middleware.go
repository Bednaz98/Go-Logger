package httpserver

import (
	"net/http"
	"strings"

	"github.com/joshuabednaz/go-logger/internal/config"
	"github.com/joshuabednaz/go-logger/internal/securecmp"
)

func WithBearerAuth(cfg config.Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthDisabled || strings.TrimSpace(cfg.AuthBearerToken) == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := r.Header.Get("Authorization")
		const p = "Bearer "
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(h)), strings.ToLower(p)) {
			WriteProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid Authorization header")
			return
		}
		tok := strings.TrimSpace(h[len(p):])
		if !securecmp.Equal(tok, cfg.AuthBearerToken) {
			WriteProblem(w, http.StatusUnauthorized, "unauthenticated", "invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func MaxBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if n > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}
