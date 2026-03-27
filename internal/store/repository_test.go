package store

import (
	"context"
	"testing"
	"time"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/domain"
)

func TestRepositoryIngestBatchIdempotent(t *testing.T) {
	db, _, err := OpenDB(":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	rec := LogInput{
		LogID:          "id-1",
		RecordKind:     "operational",
		LogMessage:     "hello",
		EventTimestamp: time.Now().UTC(),
		LogLevel:       "info",
	}
	n1, err := repo.IngestBatch(ctx, "app-a", []LogInput{rec})
	if err != nil || n1 != 1 {
		t.Fatalf("first ingest: n=%d err=%v", n1, err)
	}
	n2, err := repo.IngestBatch(ctx, "app-a", []LogInput{rec})
	if err != nil || n2 != 0 {
		t.Fatalf("duplicate ingest: n=%d err=%v", n2, err)
	}
}

func TestRepositoryListDeleteIsolation(t *testing.T) {
	db, _, err := OpenDB(":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	_, _ = repo.IngestBatch(ctx, "app-a", []LogInput{{
		LogID: "a1", RecordKind: "operational", LogMessage: "x", EventTimestamp: time.Now().UTC(), LogLevel: "info",
	}})
	_, _ = repo.IngestBatch(ctx, "app-b", []LogInput{{
		LogID: "b1", RecordKind: "operational", LogMessage: "y", EventTimestamp: time.Now().UTC(), LogLevel: "info",
	}})
	rows, _, err := repo.ListLogs(ctx, domain.QueryFilter{ApplicationName: "app-a", Limit: 10})
	if err != nil || len(rows) != 1 || rows[0].LogID != "a1" {
		t.Fatalf("list: %+v err=%v", rows, err)
	}
	n, err := repo.DeleteLogs(ctx, domain.QueryFilter{ApplicationName: "app-a"})
	if err != nil || n != 1 {
		t.Fatalf("delete: n=%d err=%v", n, err)
	}
	rowsB, _, _ := repo.ListLogs(ctx, domain.QueryFilter{ApplicationName: "app-b", Limit: 10})
	if len(rowsB) != 1 {
		t.Fatalf("other tenant affected")
	}
}

func TestRepositoryRegexSQLite(t *testing.T) {
	db, _, err := OpenDB(":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	_, _ = repo.IngestBatch(ctx, "app", []LogInput{{
		LogID: "1", RecordKind: "operational", LogMessage: "alpha beta", EventTimestamp: time.Now().UTC(), LogLevel: "info",
	}})
	f := domain.QueryFilter{
		ApplicationName: "app",
		MessageRegex:    `beta`,
		Limit:           10,
		RecordKinds:     []loggerv1.RecordKind{loggerv1.RecordKind_RECORD_KIND_OPERATIONAL},
	}
	rows, _, err := repo.ListLogs(ctx, f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}
