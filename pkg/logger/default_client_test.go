package logger

// Tests in this file use package-level Init/Close; do not add t.Parallel() here.

import (
	"context"
	"errors"
	"testing"
	"time"
)

type nopStore struct{}

func (nopStore) Append(ctx context.Context, records []LocalRecord) error { return nil }
func (nopStore) ListUnsent(ctx context.Context, limit int) ([]LocalRecord, error) {
	return nil, nil
}
func (nopStore) MarkSent(ctx context.Context, logIDs []string) error { return nil }
func (nopStore) CountUnsent(ctx context.Context) (int64, error) { return 0, nil }
func (nopStore) OldestUnsentAge(ctx context.Context) (time.Duration, bool, error) {
	return 0, false, nil
}
func (nopStore) DeleteSyncedOlderThan(ctx context.Context, cutoff time.Time) error {
	return nil
}

func testClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(nopStore{}, Options{
		ApplicationName:    "test",
		GRPCAddress:        "127.0.0.1:7443",
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInitNil(t *testing.T) {
	err := Init(nil)
	if !errors.Is(err, ErrNilClient) {
		t.Fatalf("Init(nil): got %v, want ErrNilClient", err)
	}
}

func TestPackageAPIsBeforeInit(t *testing.T) {
	ctx := context.Background()
	if _, err := Log(ctx, "info", "x", nil); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Log: %v", err)
	}
	if _, err := Track(ctx, "e", nil); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Track: %v", err)
	}
	if err := Flush(ctx); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Flush: %v", err)
	}
	if err := SetAnalyticsEnabled(false); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("SetAnalyticsEnabled: %v", err)
	}
	if err := Close(); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Close: %v", err)
	}
}

func TestInitDouble(t *testing.T) {
	c := testClient(t)
	t.Cleanup(func() { _ = Close() })

	if err := Init(c); err != nil {
		t.Fatal(err)
	}
	orphan := testClient(t)
	defer func() { _ = orphan.Close() }()
	if err := Init(orphan); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second Init: got %v, want ErrAlreadyInitialized", err)
	}
}

func TestInitLogCloseReinit(t *testing.T) {
	if _, ok := Default(); ok {
		t.Fatal("expected no default before Init")
	}
	c1 := testClient(t)
	if err := Init(c1); err != nil {
		t.Fatal(err)
	}
	d, ok := Default()
	if !ok || d != c1 {
		t.Fatal("Default after Init should return the same client")
	}
	ctx := context.Background()
	if _, err := Log(ctx, "info", "hello", nil); err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if _, ok := Default(); ok {
		t.Fatal("expected no default after Close")
	}

	c2 := testClient(t)
	if err := Init(c2); err != nil {
		t.Fatal(err)
	}
	if d, ok := Default(); !ok || d != c2 {
		t.Fatal("Default after second Init should return c2")
	}
	t.Cleanup(func() { _ = Close() })
}
