package grpcserver

import (
	"fmt"
	"strings"
	"time"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/domain"
	"github.com/joshuabednaz/go-logger/internal/store"
)

func ProtoToLogInput(r *loggerv1.LogRecord) (store.LogInput, error) {
	ts, err := parseRFC3339Required(r.GetEventTimestamp())
	if err != nil {
		return store.LogInput{}, err
	}
	return store.LogInput{
		LogID:              strings.TrimSpace(r.GetLogId()),
		RecordKind:         recordKindToStore(r.GetRecordKind()),
		AnalyticsEventName: strings.TrimSpace(r.GetAnalyticsEventName()),
		UserActorID:        strings.TrimSpace(r.GetUserActorId()),
		Source:             strings.TrimSpace(r.GetSource()),
		SourceEnvironment:  strings.TrimSpace(r.GetSourceEnvironment()),
		SessionID:          strings.TrimSpace(r.GetSessionId()),
		LogMessage:         r.GetLogMessage(),
		MetadataJSON:       append([]byte(nil), r.GetMetadataJson()...),
		EventTimestamp:     ts,
		LogLevel:           logLevelToStore(r.GetLogLevel()),
		TraceID:            strings.TrimSpace(r.GetTraceId()),
		SpanID:             strings.TrimSpace(r.GetSpanId()),
	}, nil
}

func LogToProto(l *store.Log) *loggerv1.LogRecord {
	k := store.ParseRecordKind(l.RecordKind)
	lv := store.ParseLogLevel(l.LogLevel)
	return &loggerv1.LogRecord{
		LogId:              l.LogID,
		RecordKind:         k,
		AnalyticsEventName: l.AnalyticsEventName,
		UserActorId:        l.UserActorID,
		Source:             l.Source,
		SourceEnvironment:  l.SourceEnvironment,
		SessionId:          l.SessionID,
		ApplicationName:    l.ApplicationName,
		LogMessage:         l.LogMessage,
		MetadataJson:       append([]byte(nil), l.MetadataJSON...),
		EventTimestamp:     l.EventTimestamp.UTC().Format(time.RFC3339Nano),
		LogLevel:           lv,
		TraceId:            l.TraceID,
		SpanId:             l.SpanID,
	}
}

func listReqToFilter(req *loggerv1.ListLogsRequest) (domain.QueryFilter, error) {
	f := domain.QueryFilter{
		ApplicationName:          strings.TrimSpace(req.GetApplicationName()),
		LogIDs:                   append([]string(nil), req.GetLogIds()...),
		RecordKinds:              append([]loggerv1.RecordKind(nil), req.GetRecordKinds()...),
		AnalyticsEventNameExact:  strings.TrimSpace(req.GetAnalyticsEventNameExact()),
		AnalyticsEventNamePrefix: strings.TrimSpace(req.GetAnalyticsEventNamePrefix()),
		SessionID:                strings.TrimSpace(req.GetSessionId()),
		UserActorID:              strings.TrimSpace(req.GetUserActorId()),
		MessageRegex:             strings.TrimSpace(req.GetMessageRegex()),
		LogLevels:                append([]loggerv1.LogLevel(nil), req.GetLogLevels()...),
		Limit:                    int(req.GetLimit()),
		PageToken:                strings.TrimSpace(req.GetPageToken()),
	}
	if s := strings.TrimSpace(req.GetTimeStartRfc3339()); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		f.TimeStart = &t
	}
	if s := strings.TrimSpace(req.GetTimeEndRfc3339()); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		f.TimeEnd = &t
	}
	return f, nil
}

func deleteReqToFilter(req *loggerv1.DeleteLogsRequest) (domain.QueryFilter, error) {
	f := domain.QueryFilter{
		ApplicationName:          strings.TrimSpace(req.GetApplicationName()),
		LogIDs:                   append([]string(nil), req.GetLogIds()...),
		RecordKinds:              append([]loggerv1.RecordKind(nil), req.GetRecordKinds()...),
		AnalyticsEventNameExact:  strings.TrimSpace(req.GetAnalyticsEventNameExact()),
		AnalyticsEventNamePrefix: strings.TrimSpace(req.GetAnalyticsEventNamePrefix()),
		SessionID:                strings.TrimSpace(req.GetSessionId()),
		UserActorID:              strings.TrimSpace(req.GetUserActorId()),
		MessageRegex:             strings.TrimSpace(req.GetMessageRegex()),
		LogLevels:                append([]loggerv1.LogLevel(nil), req.GetLogLevels()...),
	}
	if s := strings.TrimSpace(req.GetTimeStartRfc3339()); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		f.TimeStart = &t
	}
	if s := strings.TrimSpace(req.GetTimeEndRfc3339()); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return domain.QueryFilter{}, err
		}
		f.TimeEnd = &t
	}
	return f, nil
}

func parseRFC3339Required(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("event_timestamp required")
	}
	return time.Parse(time.RFC3339Nano, s)
}

func recordKindToStore(k loggerv1.RecordKind) string {
	switch k {
	case loggerv1.RecordKind_RECORD_KIND_OPERATIONAL:
		return "operational"
	case loggerv1.RecordKind_RECORD_KIND_ANALYTICS:
		return "analytics"
	default:
		return "unspecified"
	}
}

func logLevelToStore(l loggerv1.LogLevel) string {
	switch l {
	case loggerv1.LogLevel_LOG_LEVEL_TRACE:
		return "trace"
	case loggerv1.LogLevel_LOG_LEVEL_DEBUG:
		return "debug"
	case loggerv1.LogLevel_LOG_LEVEL_INFO:
		return "info"
	case loggerv1.LogLevel_LOG_LEVEL_WARN:
		return "warn"
	case loggerv1.LogLevel_LOG_LEVEL_ERROR:
		return "error"
	case loggerv1.LogLevel_LOG_LEVEL_FATAL:
		return "fatal"
	default:
		return "info"
	}
}
