package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/joshuabednaz/go-logger/internal/config"
	"github.com/joshuabednaz/go-logger/internal/store"
)

type LogsDeps struct {
	Repo   *store.Repository
	Config config.Server
}

func HandleListLogsQuery(dep LogsDeps) http.HandlerFunc {
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
		rows, next, err := dep.Repo.ListLogs(r.Context(), f)
		if err != nil {
			WriteProblem(w, http.StatusInternalServerError, "internal", "list failed")
			return
		}
		out := make([]LogRecordJSON, 0, len(rows))
		for i := range rows {
			out = append(out, logModelToJSON(&rows[i]))
		}
		WriteJSON(w, http.StatusOK, ListLogsResponseJSON{Records: out, NextPageToken: next})
	}
}

func HandleListLogsGET(dep LogsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := queryFromQueryParams(r.URL.Query())
		if err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		if strings.TrimSpace(f.ApplicationName) == "" {
			WriteProblem(w, http.StatusBadRequest, "invalid_argument", "application_name required")
			return
		}
		rows, next, err := dep.Repo.ListLogs(r.Context(), f)
		if err != nil {
			WriteProblem(w, http.StatusInternalServerError, "internal", "list failed")
			return
		}
		out := make([]LogRecordJSON, 0, len(rows))
		for i := range rows {
			out = append(out, logModelToJSON(&rows[i]))
		}
		WriteJSON(w, http.StatusOK, ListLogsResponseJSON{Records: out, NextPageToken: next})
	}
}
