package mcpmod

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/domain"
	"github.com/joshuabednaz/go-logger/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolConfig struct {
	EnableDeleteLogs bool

	MaxMetadataBytes     int
	EnforceMetadataLimit bool

	// RemoteIngest is optional; when set, ingest_batch also forwards to this gRPC LoggerService.
	RemoteIngest *RemoteLoggerClient
}

func RegisterTools(s *mcp.Server, repo *store.Repository, cfg ToolConfig) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ingest_batch",
		Description: "Insert log records into the local database (same shape as HTTPS POST /api/v1/ingest/batch). When MCP_REMOTE_GRPC_ADDRESS is set and MCP_REMOTE_SENDING is enabled, also forwards the batch to that remote LoggerService over gRPC. If the remote call fails after local ingest succeeds, the tool errors (local ingest is idempotent on log_id).",
	}, ingestBatch(repo, cfg))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_applications",
		Description: "List distinct application_name values stored in the database (cross-tenant directory).",
	}, listApplications(repo))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_log_by_id",
		Description: "Fetch a single log row by application_name (tenant) and log_id.",
	}, getLogByID(repo))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "query_logs",
		Description: "Query logs for a tenant (application_name) with optional filters. Returns records and next_page_token.",
	}, queryLogs(repo))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "count_logs",
		Description: "Count logs for a tenant with optional filters.",
	}, countLogs(repo))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_logs",
		Description: "Delete logs matching filters for a tenant. Destructive; requires MCP_ENABLE_DELETE_LOGS=true.",
	}, deleteLogs(repo, cfg))
}

type listAppsInput struct{}

func listApplications(repo *store.Repository) func(context.Context, *mcp.CallToolRequest, listAppsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in listAppsInput) (*mcp.CallToolResult, any, error) {
		names, err := repo.ListApplicationNames(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"application_names": names}, nil
	}
}

type getLogInput struct {
	ApplicationName string `json:"application_name" jsonschema:"required tenant application_name"`
	LogID           string `json:"log_id" jsonschema:"required client log id"`
}

func getLogByID(repo *store.Repository) func(context.Context, *mcp.CallToolRequest, *getLogInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in *getLogInput) (*mcp.CallToolResult, any, error) {
		if in == nil || strings.TrimSpace(in.ApplicationName) == "" || strings.TrimSpace(in.LogID) == "" {
			return nil, nil, fmt.Errorf("application_name and log_id required")
		}
		row, err := repo.GetByLogID(ctx, strings.TrimSpace(in.ApplicationName), strings.TrimSpace(in.LogID))
		if err != nil {
			if err == store.ErrNotFound {
				return nil, map[string]any{"found": false}, nil
			}
			return nil, nil, err
		}
		return nil, map[string]any{"found": true, "record": logRowJSON(row)}, nil
	}
}

// QueryFilterJSON mirrors HTTPS query body for tool arguments.
type QueryFilterJSON struct {
	ApplicationName          string   `json:"application_name" jsonschema:"required tenant application_name"`
	LogIDs                   []string `json:"log_ids,omitempty"`
	RecordKinds              []string `json:"record_kinds,omitempty"`
	AnalyticsEventNameExact  string   `json:"analytics_event_name_exact,omitempty"`
	AnalyticsEventNamePrefix string   `json:"analytics_event_name_prefix,omitempty"`
	TimeStartRFC3339         string   `json:"time_start_rfc3339,omitempty"`
	TimeEndRFC3339           string   `json:"time_end_rfc3339,omitempty"`
	SessionID                string   `json:"session_id,omitempty"`
	UserActorID              string   `json:"user_actor_id,omitempty"`
	MessageRegex             string   `json:"message_regex,omitempty"`
	LogLevels                []string `json:"log_levels,omitempty"`
	Limit                    uint32   `json:"limit,omitempty"`
	PageToken                string   `json:"page_token,omitempty"`
}

type queryLogsInput struct {
	QueryFilterJSON
}

func queryLogs(repo *store.Repository) func(context.Context, *mcp.CallToolRequest, *queryLogsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in *queryLogsInput) (*mcp.CallToolResult, any, error) {
		if in == nil || strings.TrimSpace(in.ApplicationName) == "" {
			return nil, nil, fmt.Errorf("application_name required")
		}
		f, err := queryFilterFromMCP(&in.QueryFilterJSON)
		if err != nil {
			return nil, nil, err
		}
		rows, next, err := repo.ListLogs(ctx, f)
		if err != nil {
			return nil, nil, err
		}
		out := make([]map[string]any, 0, len(rows))
		for i := range rows {
			out = append(out, logRowJSON(&rows[i]))
		}
		return nil, map[string]any{"records": out, "next_page_token": next}, nil
	}
}

type countLogsInput struct {
	QueryFilterJSON
}

func countLogs(repo *store.Repository) func(context.Context, *mcp.CallToolRequest, *countLogsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in *countLogsInput) (*mcp.CallToolResult, any, error) {
		if in == nil || strings.TrimSpace(in.ApplicationName) == "" {
			return nil, nil, fmt.Errorf("application_name required")
		}
		f, err := queryFilterFromMCP(&in.QueryFilterJSON)
		if err != nil {
			return nil, nil, err
		}
		n, err := repo.CountLogs(ctx, f)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"count": n}, nil
	}
}

type deleteLogsInput struct {
	QueryFilterJSON
}

func deleteLogs(repo *store.Repository, cfg ToolConfig) func(context.Context, *mcp.CallToolRequest, *deleteLogsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in *deleteLogsInput) (*mcp.CallToolResult, any, error) {
		if !cfg.EnableDeleteLogs {
			slog.Info("mcp: delete_logs refused because MCP_ENABLE_DELETE_LOGS is not enabled")
			return nil, nil, fmt.Errorf("delete_logs disabled (set MCP_ENABLE_DELETE_LOGS=true)")
		}
		if in == nil || strings.TrimSpace(in.ApplicationName) == "" {
			return nil, nil, fmt.Errorf("application_name required")
		}
		f, err := queryFilterFromMCP(&in.QueryFilterJSON)
		if err != nil {
			return nil, nil, err
		}
		n, err := repo.DeleteLogs(ctx, f)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"deleted_count": n}, nil
	}
}

func logRowJSON(l *store.Log) map[string]any {
	var meta any = json.RawMessage(l.MetadataJSON)
	if len(l.MetadataJSON) == 0 {
		meta = nil
	}
	return map[string]any{
		"log_id":               l.LogID,
		"application_name":     l.ApplicationName,
		"record_kind":          l.RecordKind,
		"analytics_event_name": l.AnalyticsEventName,
		"user_actor_id":        l.UserActorID,
		"source":               l.Source,
		"source_environment":   l.SourceEnvironment,
		"session_id":           l.SessionID,
		"log_message":          l.LogMessage,
		"metadata_json":        meta,
		"event_timestamp":      l.EventTimestamp.UTC().Format(time.RFC3339Nano),
		"log_level":            l.LogLevel,
		"trace_id":             l.TraceID,
		"span_id":              l.SpanID,
	}
}

func queryFilterFromMCP(q *QueryFilterJSON) (domain.QueryFilter, error) {
	f := domain.QueryFilter{
		ApplicationName:          strings.TrimSpace(q.ApplicationName),
		LogIDs:                   append([]string(nil), q.LogIDs...),
		AnalyticsEventNameExact:  strings.TrimSpace(q.AnalyticsEventNameExact),
		AnalyticsEventNamePrefix: strings.TrimSpace(q.AnalyticsEventNamePrefix),
		SessionID:                strings.TrimSpace(q.SessionID),
		UserActorID:              strings.TrimSpace(q.UserActorID),
		MessageRegex:             strings.TrimSpace(q.MessageRegex),
		Limit:                    int(q.Limit),
		PageToken:                strings.TrimSpace(q.PageToken),
	}
	for _, s := range q.RecordKinds {
		k, err := parseRecordKindMCP(s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		if k != loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED {
			f.RecordKinds = append(f.RecordKinds, k)
		}
	}
	for _, s := range q.LogLevels {
		lv, err := parseLogLevelMCP(s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		if lv != loggerv1.LogLevel_LOG_LEVEL_UNSPECIFIED {
			f.LogLevels = append(f.LogLevels, lv)
		}
	}
	if s := strings.TrimSpace(q.TimeStartRFC3339); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		f.TimeStart = &t
	}
	if s := strings.TrimSpace(q.TimeEndRFC3339); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		f.TimeEnd = &t
	}
	return f, nil
}

func parseRecordKindMCP(s string) (loggerv1.RecordKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "operational", "record_kind_operational":
		return loggerv1.RecordKind_RECORD_KIND_OPERATIONAL, nil
	case "analytics", "record_kind_analytics":
		return loggerv1.RecordKind_RECORD_KIND_ANALYTICS, nil
	case "":
		return loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED, nil
	default:
		return loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED, fmt.Errorf("unknown record_kind %q", s)
	}
}

func parseLogLevelMCP(s string) (loggerv1.LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return loggerv1.LogLevel_LOG_LEVEL_UNSPECIFIED, nil
	case "trace", "log_level_trace":
		return loggerv1.LogLevel_LOG_LEVEL_TRACE, nil
	case "debug", "log_level_debug":
		return loggerv1.LogLevel_LOG_LEVEL_DEBUG, nil
	case "info", "log_level_info":
		return loggerv1.LogLevel_LOG_LEVEL_INFO, nil
	case "warn", "warning", "log_level_warn":
		return loggerv1.LogLevel_LOG_LEVEL_WARN, nil
	case "error", "log_level_error":
		return loggerv1.LogLevel_LOG_LEVEL_ERROR, nil
	case "fatal", "log_level_fatal":
		return loggerv1.LogLevel_LOG_LEVEL_FATAL, nil
	default:
		return loggerv1.LogLevel_LOG_LEVEL_UNSPECIFIED, fmt.Errorf("unknown log_level %q", s)
	}
}
