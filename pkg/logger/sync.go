package logger

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *Client) syncLoop(ctx context.Context) {
	tick := time.NewTicker(c.opts.BackgroundPollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_ = c.tick(ctx)
		}
	}
}

func (c *Client) tick(ctx context.Context) error {
	if err := c.maybeUpload(ctx); err != nil {
		return err
	}
	return c.store.DeleteSyncedOlderThan(ctx, c.purgeCutoff())
}

func (c *Client) maybeUpload(ctx context.Context) error {
	if c.opts.DisableRemote || c.transport == nil {
		return nil
	}
	n, err := c.store.CountUnsent(ctx)
	if err != nil {
		return err
	}
	should := n >= int64(c.opts.AutoSendMinUnsentCount)
	if !should {
		age, ok, err := c.store.OldestUnsentAge(ctx)
		if err != nil {
			return err
		}
		if ok && age >= c.opts.AutoSendMaxUnsentAge {
			should = true
		}
	}
	if !should || n == 0 {
		return nil
	}
	return c.uploadPending(ctx)
}

func (c *Client) uploadPending(ctx context.Context) error {
	if c.opts.DisableRemote || c.transport == nil {
		return nil
	}
	backoff := 200 * time.Millisecond
	const maxBackoff = 10 * time.Second
	transientAttempts := 0
	const maxTransientAttempts = 32

	for {
		batch, err := c.store.ListUnsent(ctx, c.opts.MaxRecordsPerUpload)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		ids := make([]string, 0, len(batch))
		for i := range batch {
			ids = append(ids, batch[i].LogID)
		}
		_, err = c.transport.IngestBatch(ctx, c.opts.ApplicationName, batch)
		if err != nil {
			if st, ok := status.FromError(err); ok {
				switch st.Code() {
				case codes.Unavailable, codes.DeadlineExceeded:
					transientAttempts++
					if transientAttempts > maxTransientAttempts {
						return err
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(backoff):
					}
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}
			}
			return err
		}
		transientAttempts = 0
		backoff = 200 * time.Millisecond
		if err := c.store.MarkSent(ctx, ids); err != nil {
			return err
		}
		_ = c.store.DeleteSyncedOlderThan(ctx, c.purgeCutoff())
	}
}
