package store

import "time"

// Log is the persisted unified log / analytics row.
type Log struct {
	ID uint `gorm:"primaryKey"`

	ApplicationName string `gorm:"size:512;not null;uniqueIndex:idx_app_logid;index:idx_app_event_time"`
	LogID           string `gorm:"size:512;not null;uniqueIndex:idx_app_logid"`

	RecordKind         string    `gorm:"size:64;index"`
	AnalyticsEventName string    `gorm:"size:512;index"`
	UserActorID        string    `gorm:"size:512;index"`
	Source             string    `gorm:"size:256"`
	SourceEnvironment  string    `gorm:"size:64"`
	SessionID          string    `gorm:"size:256;index"`
	LogMessage         string    `gorm:"type:text"`
	MetadataJSON       []byte    `gorm:"type:blob"`
	EventTimestamp     time.Time `gorm:"index:idx_app_event_time"`
	LogLevel           string    `gorm:"size:32;index"`
	TraceID            string    `gorm:"size:256"`
	SpanID             string    `gorm:"size:256"`

	CreatedAt time.Time
}

func (Log) TableName() string {
	return "logs"
}
