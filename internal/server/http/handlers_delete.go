package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/joshuabednaz/go-logger/internal/config"
	"github.com/joshuabednaz/go-logger/internal/store"
)

type DeleteDeps struct {
	Repo   *store.Repository
	Config config.Server
}

func HandleDeleteLogs(dep DeleteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body QueryFilterJSON
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid_json", "could not decode body")
			return
		}
		f, err := queryBodyToFilter(body)
		if err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		if strings.TrimSpace(f.ApplicationName) == "" {
			WriteProblem(w, http.StatusBadRequest, "invalid_argument", "application_name required")
			return
		}
		n, err := dep.Repo.DeleteLogs(r.Context(), f)
		if err != nil {
			WriteProblem(w, http.StatusInternalServerError, "internal", "delete failed")
			return
		}
		WriteJSON(w, http.StatusOK, DeleteLogsResponseJSON{DeletedCount: uint64(n)})
	}
}
