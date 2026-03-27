package sqllogstore

import (
	"context"
	"errors"
	"time"

	"github.com/joshuabednaz/go-logger/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store is a reference LocalLogStore backed by SQLite (or any GORM SQL dialector).
type Store struct {
	db *gorm.DB
}

type row struct {
	ID uint `gorm:"primaryKey"`

	LogID              string `gorm:"size:512;not null;uniqueIndex:idx_client_app_logid"`
	RecordKind         string `gorm:"size:64"`
	AnalyticsEventName string `gorm:"size:512"`
	UserActorID        string `gorm:"size:512"`
	Source             string `gorm:"size:256"`
	SourceEnvironment  string `gorm:"size:64"`
	SessionID          string `gorm:"size:256"`
	ApplicationName    string `gorm:"size:512;not null;uniqueIndex:idx_client_app_logid"`
	LogMessage         string `gorm:"type:text"`
	MetadataJSON       []byte `gorm:"type:blob"`
	EventTimestamp     time.Time
	LogLevel           string `gorm:"size:32"`
	TraceID            string `gorm:"size:256"`
	SpanID             string `gorm:"size:256"`

	QueuedAt      time.Time
	ServerAckedAt *time.Time
}

func (row) TableName() string { return "client_local_logs" }

func New(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("sqllogstore: nil db")
	}
	if err := db.AutoMigrate(&row{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Append(ctx context.Context, records []logger.LocalRecord) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		for i := range records {
			r := records[i]
			rec := row{
				LogID:              r.LogID,
				RecordKind:         r.RecordKind,
				AnalyticsEventName: r.AnalyticsEventName,
				UserActorID:        r.UserActorID,
				Source:             r.Source,
				SourceEnvironment:  r.SourceEnvironment,
				SessionID:          r.SessionID,
				ApplicationName:    r.ApplicationName,
				LogMessage:         r.LogMessage,
				MetadataJSON:       append([]byte(nil), r.MetadataJSON...),
				EventTimestamp:     r.EventTimestamp,
				LogLevel:           r.LogLevel,
				TraceID:            r.TraceID,
				SpanID:             r.SpanID,
				QueuedAt:           now,
				ServerAckedAt:      nil,
			}
			res := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "application_name"}, {Name: "log_id"}},
				DoNothing: true,
			}).Create(&rec)
			if res.Error != nil {
				return res.Error
			}
		}
		return nil
	})
}

func (s *Store) ListUnsent(ctx context.Context, limit int) ([]logger.LocalRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Where("server_acked_at IS NULL").
		Order("queued_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]logger.LocalRecord, 0, len(rows))
	for i := range rows {
		out = append(out, rowToLocal(&rows[i]))
	}
	return out, nil
}

func (s *Store) MarkSent(ctx context.Context, logIDs []string) error {
	if len(logIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&row{}).
		Where("log_id IN ?", logIDs).
		Updates(map[string]any{"server_acked_at": now}).Error
}

func (s *Store) CountUnsent(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&row{}).Where("server_acked_at IS NULL").Count(&n).Error
	return n, err
}

func (s *Store) OldestUnsentAge(ctx context.Context) (time.Duration, bool, error) {
	var r row
	err := s.db.WithContext(ctx).
		Where("server_acked_at IS NULL").
		Order("queued_at ASC").
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return time.Since(r.QueuedAt), true, nil
}

func (s *Store) DeleteSyncedOlderThan(ctx context.Context, cutoff time.Time) error {
	return s.db.WithContext(ctx).
		Where("server_acked_at IS NOT NULL AND server_acked_at < ?", cutoff).
		Delete(&row{}).Error
}

func rowToLocal(l *row) logger.LocalRecord {
	return logger.LocalRecord{
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
		EventTimestamp:     l.EventTimestamp,
		LogLevel:           l.LogLevel,
		TraceID:            l.TraceID,
		SpanID:             l.SpanID,
	}
}
