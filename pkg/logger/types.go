package logger

import "time"

// LocalRecord holds the log payload fields required for persistence and upload.
type LocalRecord struct {
	LogID              string
	RecordKind         string // operational | analytics
	AnalyticsEventName string
	UserActorID        string
	Source             string
	SourceEnvironment  string
	SessionID          string
	ApplicationName    string
	LogMessage         string
	MetadataJSON       []byte
	EventTimestamp     time.Time
	LogLevel           string
	TraceID            string
	SpanID             string
}
