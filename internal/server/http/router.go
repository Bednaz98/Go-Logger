package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joshuabednaz/go-logger/internal/config"
	"github.com/joshuabednaz/go-logger/internal/store"
)

func NewRouter(cfg config.Server, repo *store.Repository) http.Handler {
	r := chi.NewRouter()
	max := int64(cfg.MaxGRPCRecvBytes)
	if max <= 0 {
		max = 4 * 1024 * 1024
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", HandleHealth)
		r.Group(func(r chi.Router) {
			r.Use(MaxBytes(max))
			r.Use(func(next http.Handler) http.Handler {
				return WithBearerAuth(cfg, next)
			})
			r.Post("/ingest/batch", HandleIngestBatch(IngestDeps{Repo: repo, Config: cfg}))
			r.Get("/logs", HandleListLogsGET(LogsDeps{Repo: repo, Config: cfg}))
			r.Post("/logs/query", HandleListLogsQuery(LogsDeps{Repo: repo, Config: cfg}))
			r.Delete("/logs", HandleDeleteLogs(DeleteDeps{Repo: repo, Config: cfg}))
		})
	})
	return r
}
