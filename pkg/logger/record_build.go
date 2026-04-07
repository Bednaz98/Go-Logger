package logger

import (
	"time"

	"github.com/google/uuid"
)

func newOperationalRecord(opts Options, sessionID, level, message string, metadataJSON []byte) LocalRecord {
	id := uuid.NewString()
	return LocalRecord{
		LogID:             id,
		RecordKind:        "operational",
		Source:            opts.Source,
		SourceEnvironment: opts.SourceEnvironment,
		SessionID:         sessionID,
		ApplicationName:   opts.ApplicationName,
		LogMessage:        message,
		MetadataJSON:      append([]byte(nil), metadataJSON...),
		EventTimestamp:    time.Now().UTC(),
		LogLevel:          level,
	}
}

func newAnalyticsRecord(opts Options, sessionID, eventName string, metadataJSON []byte) LocalRecord {
	id := uuid.NewString()
	return LocalRecord{
		LogID:              id,
		RecordKind:         "analytics",
		AnalyticsEventName: eventName,
		Source:             opts.Source,
		SourceEnvironment:  opts.SourceEnvironment,
		SessionID:          sessionID,
		ApplicationName:    opts.ApplicationName,
		MetadataJSON:       append([]byte(nil), metadataJSON...),
		EventTimestamp:     time.Now().UTC(),
		LogLevel:           "info",
	}
}
