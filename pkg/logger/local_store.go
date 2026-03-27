package logger

import (
	"context"
	"time"
)

// LocalLogStore is implemented by the host. Implementations should be safe for concurrent use
// from multiple goroutines unless the SDK documents single-threaded access only.
type LocalLogStore interface {
	Append(ctx context.Context, records []LocalRecord) error

	ListUnsent(ctx context.Context, limit int) ([]LocalRecord, error)

	MarkSent(ctx context.Context, logIDs []string) error

	CountUnsent(ctx context.Context) (int64, error)

	OldestUnsentAge(ctx context.Context) (age time.Duration, ok bool, err error)

	DeleteSyncedOlderThan(ctx context.Context, cutoff time.Time) error
}
