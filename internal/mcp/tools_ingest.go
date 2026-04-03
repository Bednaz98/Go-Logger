package mcpmod

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	grpcserver "github.com/joshuabednaz/go-logger/internal/server/grpc"
	httpserver "github.com/joshuabednaz/go-logger/internal/server/http"
	"github.com/joshuabednaz/go-logger/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ingestBatchInput struct {
	ApplicationName string                     `json:"application_name" jsonschema:"required tenant application_name"`
	Records         []httpserver.LogRecordJSON `json:"records" jsonschema:"required log records"`
}

func ingestBatch(repo *store.Repository, cfg ToolConfig) func(context.Context, *mcp.CallToolRequest, *ingestBatchInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in *ingestBatchInput) (*mcp.CallToolResult, any, error) {
		if in == nil || strings.TrimSpace(in.ApplicationName) == "" || len(in.Records) == 0 {
			return nil, nil, fmt.Errorf("application_name and non-empty records required")
		}
		body := &httpserver.IngestBatchBody{ApplicationName: in.ApplicationName, Records: in.Records}
		app, protos, err := httpserver.IngestBatchBodyToProtos(body)
		if err != nil {
			return nil, nil, err
		}
		maxMeta := cfg.MaxMetadataBytes
		if maxMeta <= 0 {
			maxMeta = 256 * 1024
		}
		if err := grpcserver.ValidateIngestBatch(app, protos, maxMeta, cfg.EnforceMetadataLimit); err != nil {
			return nil, nil, err
		}
		inputs := make([]store.LogInput, 0, len(protos))
		for _, p := range protos {
			li, err := grpcserver.ProtoToLogInput(p)
			if err != nil {
				return nil, nil, err
			}
			inputs = append(inputs, li)
		}
		n, err := repo.IngestBatch(ctx, app, inputs)
		if err != nil {
			return nil, nil, err
		}
		out := map[string]any{
			"local_accepted_count": uint32(n),
			"batch_id":             uuid.NewString(),
		}
		if cfg.RemoteIngest != nil {
			rn, rerr := cfg.RemoteIngest.IngestBatch(ctx, app, protos)
			out["remote_accepted_count"] = rn
			if rerr != nil {
				return nil, nil, fmt.Errorf("local ingest ok (%d accepted) but remote forward failed: %w", n, rerr)
			}
		}
		return nil, out, nil
	}
}
