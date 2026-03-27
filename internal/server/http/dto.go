package httpserver

// JSON DTOs use snake_case field names aligned with protobuf JSON mapping.

type LogRecordJSON struct {
	LogID              string `json:"log_id"`
	RecordKind         string `json:"record_kind"`
	AnalyticsEventName string `json:"analytics_event_name"`
	UserActorID        string `json:"user_actor_id"`
	Source             string `json:"source"`
	SourceEnvironment  string `json:"source_environment"`
	SessionID          string `json:"session_id"`
	ApplicationName    string `json:"application_name"`
	LogMessage         string `json:"log_message"`
	MetadataJSON       []byte `json:"metadata_json"`
	EventTimestamp     string `json:"event_timestamp"`
	LogLevel           string `json:"log_level"`
	TraceID            string `json:"trace_id"`
	SpanID             string `json:"span_id"`
}

type IngestBatchBody struct {
	ApplicationName string          `json:"application_name"`
	Records         []LogRecordJSON `json:"records"`
}

type IngestBatchResponseJSON struct {
	AcceptedCount uint32 `json:"accepted_count"`
	BatchID       string `json:"batch_id,omitempty"`
}

type QueryFilterJSON struct {
	ApplicationName          string   `json:"application_name"`
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

type ListLogsResponseJSON struct {
	Records       []LogRecordJSON `json:"records"`
	NextPageToken string          `json:"next_page_token,omitempty"`
}

type DeleteLogsResponseJSON struct {
	DeletedCount uint64 `json:"deleted_count"`
}

type HealthResponse struct {
	Status string `json:"status"`
}
