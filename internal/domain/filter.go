package domain

import (
	"time"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
)

// QueryFilter is shared across gRPC, HTTPS JSON, and MCP list/delete paths.
type QueryFilter struct {
	ApplicationName string

	LogIDs []string

	RecordKinds []loggerv1.RecordKind

	AnalyticsEventNameExact  string
	AnalyticsEventNamePrefix string

	TimeStart *time.Time
	TimeEnd   *time.Time

	SessionID   string
	UserActorID string

	MessageRegex string

	LogLevels []loggerv1.LogLevel

	Limit     int
	PageToken string
}
