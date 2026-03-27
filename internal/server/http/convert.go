package httpserver

import (
	"fmt"
	"strings"
	"time"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/domain"
	"github.com/joshuabednaz/go-logger/internal/store"
)

func jsonRecordToProto(j *LogRecordJSON) (*loggerv1.LogRecord, error) {
	if j == nil {
		return nil, fmt.Errorf("nil record")
	}
	kind, err := parseRecordKind(j.RecordKind)
	if err != nil {
		return nil, err
	}
	lvl, err := parseLogLevel(j.LogLevel)
	if err != nil {
		return nil, err
	}
	return &loggerv1.LogRecord{
		LogId:              strings.TrimSpace(j.LogID),
		RecordKind:         kind,
		AnalyticsEventName: strings.TrimSpace(j.AnalyticsEventName),
		UserActorId:        strings.TrimSpace(j.UserActorID),
		Source:             strings.TrimSpace(j.Source),
		SourceEnvironment:  strings.TrimSpace(j.SourceEnvironment),
		SessionId:          strings.TrimSpace(j.SessionID),
		ApplicationName:    strings.TrimSpace(j.ApplicationName),
		LogMessage:         j.LogMessage,
		MetadataJson:       append([]byte(nil), j.MetadataJSON...),
		EventTimestamp:     strings.TrimSpace(j.EventTimestamp),
		LogLevel:           lvl,
		TraceId:            strings.TrimSpace(j.TraceID),
		SpanId:             strings.TrimSpace(j.SpanID),
	}, nil
}

func logModelToJSON(l *store.Log) LogRecordJSON {
	return LogRecordJSON{
		LogID:              l.LogID,
		RecordKind:         l.RecordKind,
		AnalyticsEventName: l.AnalyticsEventName,
		UserActorID:        l.UserActorID,
		Source:             l.Source,
		SourceEnvironment:  l.SourceEnvironment,
		SessionID:          l.SessionID,
		ApplicationName:    l.ApplicationName,
		LogMessage:         l.LogMessage,
		MetadataJSON:       append([]byte(nil), l.MetadataJSON...),
		EventTimestamp:     l.EventTimestamp.UTC().Format(time.RFC3339Nano),
		LogLevel:           l.LogLevel,
		TraceID:            l.TraceID,
		SpanID:             l.SpanID,
	}
}

func parseRecordKind(s string) (loggerv1.RecordKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "operational", "record_kind_operational":
		return loggerv1.RecordKind_RECORD_KIND_OPERATIONAL, nil
	case "analytics", "record_kind_analytics":
		return loggerv1.RecordKind_RECORD_KIND_ANALYTICS, nil
	case "":
		return loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED, fmt.Errorf("record_kind required")
	default:
		return loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED, fmt.Errorf("unknown record_kind")
	}
}

func parseLogLevel(s string) (loggerv1.LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace", "log_level_trace":
		return loggerv1.LogLevel_LOG_LEVEL_TRACE, nil
	case "debug", "log_level_debug":
		return loggerv1.LogLevel_LOG_LEVEL_DEBUG, nil
	case "info", "log_level_info", "":
		return loggerv1.LogLevel_LOG_LEVEL_INFO, nil
	case "warn", "warning", "log_level_warn":
		return loggerv1.LogLevel_LOG_LEVEL_WARN, nil
	case "error", "log_level_error":
		return loggerv1.LogLevel_LOG_LEVEL_ERROR, nil
	case "fatal", "log_level_fatal":
		return loggerv1.LogLevel_LOG_LEVEL_FATAL, nil
	default:
		return loggerv1.LogLevel_LOG_LEVEL_UNSPECIFIED, fmt.Errorf("unknown log_level")
	}
}

func queryBodyToFilter(q QueryFilterJSON) (domain.QueryFilter, error) {
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
		k, err := parseRecordKind(s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		if k != loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED {
			f.RecordKinds = append(f.RecordKinds, k)
		}
	}
	for _, s := range q.LogLevels {
		lv, err := parseLogLevel(s)
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

func queryFromQueryParams(q map[string][]string) (domain.QueryFilter, error) {
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	body := QueryFilterJSON{
		ApplicationName:          get("application_name"),
		AnalyticsEventNameExact:  get("analytics_event_name_exact"),
		AnalyticsEventNamePrefix: get("analytics_event_name_prefix"),
		TimeStartRFC3339:         get("time_start_rfc3339"),
		TimeEndRFC3339:           get("time_end_rfc3339"),
		SessionID:                get("session_id"),
		UserActorID:              get("user_actor_id"),
		MessageRegex:             get("message_regex"),
		PageToken:                get("page_token"),
	}
	// limit optional
	if lim := strings.TrimSpace(get("limit")); lim != "" {
		var n uint32
		_, _ = fmt.Sscanf(lim, "%d", &n)
		body.Limit = n
	}
	// repeated params log_ids
	if ids, ok := q["log_id"]; ok {
		body.LogIDs = append(body.LogIDs, ids...)
	}
	if ids, ok := q["log_ids"]; ok {
		body.LogIDs = append(body.LogIDs, ids...)
	}
	return queryBodyToFilter(body)
}
