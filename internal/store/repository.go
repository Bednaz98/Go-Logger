package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LogInput is a normalized row for ingest (tenant validated by caller).
type LogInput struct {
	LogID              string
	RecordKind         string
	AnalyticsEventName string
	UserActorID        string
	Source             string
	SourceEnvironment  string
	SessionID          string
	LogMessage         string
	MetadataJSON       []byte
	EventTimestamp     time.Time
	LogLevel           string
	TraceID            string
	SpanID             string
}

type Repository struct {
	db      *gorm.DB
	dialect string
}

func NewRepository(db *gorm.DB, dialect string) *Repository {
	return &Repository{db: db, dialect: dialect}
}

func (r *Repository) IngestBatch(ctx context.Context, applicationName string, records []LogInput) (accepted int, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inserted := 0
		for _, rec := range records {
			row := Log{
				ApplicationName:    applicationName,
				LogID:              rec.LogID,
				RecordKind:         rec.RecordKind,
				AnalyticsEventName: rec.AnalyticsEventName,
				UserActorID:        rec.UserActorID,
				Source:             rec.Source,
				SourceEnvironment:  rec.SourceEnvironment,
				SessionID:          rec.SessionID,
				LogMessage:         rec.LogMessage,
				MetadataJSON:       rec.MetadataJSON,
				EventTimestamp:     rec.EventTimestamp,
				LogLevel:           rec.LogLevel,
				TraceID:            rec.TraceID,
				SpanID:             rec.SpanID,
				CreatedAt:          time.Now().UTC(),
			}
			res := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "application_name"}, {Name: "log_id"}},
				DoNothing: true,
			}).Create(&row)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				inserted++
			}
		}
		accepted = inserted
		return nil
	})
	return accepted, err
}

func (r *Repository) ListApplicationNames(ctx context.Context) ([]string, error) {
	var names []string
	err := r.db.WithContext(ctx).Model(&Log{}).
		Distinct("application_name").
		Order("application_name ASC").
		Pluck("application_name", &names).Error
	return names, err
}

func (r *Repository) GetByLogID(ctx context.Context, applicationName, logID string) (*Log, error) {
	var row Log
	err := r.db.WithContext(ctx).
		Where("application_name = ? AND log_id = ?", applicationName, logID).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *Repository) ListLogs(ctx context.Context, f domain.QueryFilter) ([]Log, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset, err := decodePageToken(f.PageToken)
	if err != nil {
		return nil, "", err
	}

	q := r.db.WithContext(ctx).Model(&Log{})
	q = r.applyFilter(q, f)
	q = q.Order("event_timestamp DESC").Order("id DESC").Limit(limit + 1).Offset(offset)

	var rows []Log
	if err := q.Find(&rows).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		next = encodePageToken(offset + limit)
		rows = rows[:limit]
	}
	return rows, next, nil
}

func (r *Repository) CountLogs(ctx context.Context, f domain.QueryFilter) (int64, error) {
	q := r.db.WithContext(ctx).Model(&Log{})
	q = r.applyFilter(q, f)
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *Repository) DeleteLogs(ctx context.Context, f domain.QueryFilter) (int64, error) {
	q := r.applyFilter(r.db.WithContext(ctx).Model(&Log{}), f)
	res := q.Delete(&Log{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *Repository) applyFilter(db *gorm.DB, f domain.QueryFilter) *gorm.DB {
	db = db.Where("application_name = ?", f.ApplicationName)
	if len(f.LogIDs) > 0 {
		db = db.Where("log_id IN ?", f.LogIDs)
	}
	if len(f.RecordKinds) > 0 {
		var kinds []string
		for _, k := range f.RecordKinds {
			if k == loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED {
				continue
			}
			kinds = append(kinds, recordKindString(k))
		}
		if len(kinds) > 0 {
			db = db.Where("record_kind IN ?", kinds)
		}
	}
	if f.AnalyticsEventNameExact != "" {
		db = db.Where("analytics_event_name = ?", f.AnalyticsEventNameExact)
	}
	if f.AnalyticsEventNamePrefix != "" {
		db = db.Where("analytics_event_name LIKE ?", f.AnalyticsEventNamePrefix+"%")
	}
	if f.TimeStart != nil {
		db = db.Where("event_timestamp >= ?", *f.TimeStart)
	}
	if f.TimeEnd != nil {
		db = db.Where("event_timestamp <= ?", *f.TimeEnd)
	}
	if f.SessionID != "" {
		db = db.Where("session_id = ?", f.SessionID)
	}
	if f.UserActorID != "" {
		db = db.Where("user_actor_id = ?", f.UserActorID)
	}
	if f.MessageRegex != "" {
		if r.dialect == "postgres" {
			db = db.Where("log_message ~ ?", f.MessageRegex)
		} else {
			db = db.Where("log_message REGEXP ?", f.MessageRegex)
		}
	}
	if len(f.LogLevels) > 0 {
		var levels []string
		for _, lv := range f.LogLevels {
			if lv == loggerv1.LogLevel_LOG_LEVEL_UNSPECIFIED {
				continue
			}
			levels = append(levels, logLevelString(lv))
		}
		if len(levels) > 0 {
			db = db.Where("log_level IN ?", levels)
		}
	}
	return db
}

func decodePageToken(tok string) (int, error) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return 0, nil
	}
	o, err := strconv.Atoi(tok)
	if err != nil || o < 0 {
		return 0, fmt.Errorf("invalid page_token")
	}
	return o, nil
}

func encodePageToken(offset int) string {
	return strconv.Itoa(offset)
}

func recordKindString(k loggerv1.RecordKind) string {
	switch k {
	case loggerv1.RecordKind_RECORD_KIND_OPERATIONAL:
		return "operational"
	case loggerv1.RecordKind_RECORD_KIND_ANALYTICS:
		return "analytics"
	default:
		return "unspecified"
	}
}

func logLevelString(l loggerv1.LogLevel) string {
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
		return "unspecified"
	}
}

// Domain helpers used by gRPC/HTTP layers
func ParseRecordKind(s string) loggerv1.RecordKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "operational":
		return loggerv1.RecordKind_RECORD_KIND_OPERATIONAL
	case "analytics":
		return loggerv1.RecordKind_RECORD_KIND_ANALYTICS
	default:
		return loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED
	}
}

func ParseLogLevel(s string) loggerv1.LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return loggerv1.LogLevel_LOG_LEVEL_TRACE
	case "debug":
		return loggerv1.LogLevel_LOG_LEVEL_DEBUG
	case "info":
		return loggerv1.LogLevel_LOG_LEVEL_INFO
	case "warn", "warning":
		return loggerv1.LogLevel_LOG_LEVEL_WARN
	case "error":
		return loggerv1.LogLevel_LOG_LEVEL_ERROR
	case "fatal":
		return loggerv1.LogLevel_LOG_LEVEL_FATAL
	default:
		return loggerv1.LogLevel_LOG_LEVEL_UNSPECIFIED
	}
}
