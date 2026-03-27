package logger

import "time"

func (c *Client) purgeCutoff() time.Time {
	d := *c.opts.LocalPurgeSyncedOlderThan
	if d == 0 {
		return time.Now().UTC()
	}
	return time.Now().UTC().Add(-d)
}
