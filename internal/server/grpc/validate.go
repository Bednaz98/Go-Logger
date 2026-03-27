package grpcserver

import (
	"fmt"
	"strings"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
)

func ValidateIngestBatch(app string, records []*loggerv1.LogRecord, maxMeta int, enforceMeta bool) error {
	app = strings.TrimSpace(app)
	if app == "" {
		return fmt.Errorf("application_name required")
	}
	if len(records) == 0 {
		return fmt.Errorf("records required")
	}
	seen := make(map[string]struct{}, len(records))
	for _, r := range records {
		if r == nil {
			return fmt.Errorf("nil record")
		}
		id := strings.TrimSpace(r.GetLogId())
		if id == "" {
			return fmt.Errorf("log_id required")
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate log_id in batch: %s", id)
		}
		seen[id] = struct{}{}
		ra := strings.TrimSpace(r.GetApplicationName())
		if ra == "" {
			return fmt.Errorf("record application_name required")
		}
		if ra != app {
			return fmt.Errorf("record application_name mismatch")
		}
		if enforceMeta && len(r.GetMetadataJson()) > maxMeta {
			return fmt.Errorf("metadata_json exceeds limit")
		}
		switch r.GetRecordKind() {
		case loggerv1.RecordKind_RECORD_KIND_OPERATIONAL:
			if strings.TrimSpace(r.GetLogMessage()) == "" {
				return fmt.Errorf("log_message required for operational records")
			}
		case loggerv1.RecordKind_RECORD_KIND_ANALYTICS:
			if strings.TrimSpace(r.GetAnalyticsEventName()) == "" {
				return fmt.Errorf("analytics_event_name required for analytics records")
			}
		case loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED:
			return fmt.Errorf("record_kind required")
		}
	}
	return nil
}
