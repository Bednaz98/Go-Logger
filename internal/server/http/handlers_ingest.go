package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/joshuabednaz/go-logger/internal/config"
	grpcserver "github.com/joshuabednaz/go-logger/internal/server/grpc"
	"github.com/joshuabednaz/go-logger/internal/store"
)

type IngestDeps struct {
	Repo   *store.Repository
	Config config.Server
}

func HandleIngestBatch(dep IngestDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body IngestBatchBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid_json", "could not decode body")
			return
		}
		app, protos, err := IngestBatchBodyToProtos(&body)
		if err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		maxMeta := dep.Config.MaxMetadataBytes
		if maxMeta <= 0 {
			maxMeta = 256 * 1024
		}
		if err := grpcserver.ValidateIngestBatch(app, protos, maxMeta, dep.Config.EnforceMetadataLimit); err != nil {
			if strings.Contains(err.Error(), "metadata_json exceeds") {
				WriteProblem(w, http.StatusRequestEntityTooLarge, "payload_too_large", err.Error())
				return
			}
			WriteProblem(w, http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		inputs := make([]store.LogInput, 0, len(protos))
		for _, p := range protos {
			in, err := grpcserver.ProtoToLogInput(p)
			if err != nil {
				WriteProblem(w, http.StatusBadRequest, "invalid_argument", err.Error())
				return
			}
			inputs = append(inputs, in)
		}
		n, err := dep.Repo.IngestBatch(r.Context(), app, inputs)
		if err != nil {
			WriteProblem(w, http.StatusInternalServerError, "internal", "ingest failed")
			return
		}
		WriteJSON(w, http.StatusOK, IngestBatchResponseJSON{
			AcceptedCount: uint32(n),
			BatchID:       uuid.NewString(),
		})
	}
}
