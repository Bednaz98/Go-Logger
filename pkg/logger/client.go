package logger

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Options configures the client SDK.
type Options struct {
	ApplicationName string
	GRPCAddress     string
	BearerToken     string

	TLSCAPEM           []byte
	InsecureSkipVerify bool

	MaxRecordsPerUpload int

	BackgroundPollInterval time.Duration

	AutoSendMinUnsentCount int
	AutoSendMaxUnsentAge   time.Duration

	// LocalPurgeSyncedOlderThan is the age (by server_acked_at) after which synced rows may be deleted.
	// nil uses the spec default (168h). A non-nil zero duration purges acked rows as soon as possible.
	LocalPurgeSyncedOlderThan *time.Duration
	LocalPurgeRunInterval     time.Duration

	Source            string
	SourceEnvironment string
}

func (o *Options) applyDefaults() {
	if o.MaxRecordsPerUpload <= 0 {
		o.MaxRecordsPerUpload = 100
	}
	if o.BackgroundPollInterval <= 0 {
		o.BackgroundPollInterval = 5 * time.Second
	}
	if o.AutoSendMinUnsentCount <= 0 {
		o.AutoSendMinUnsentCount = 100
	}
	if o.AutoSendMaxUnsentAge <= 0 {
		o.AutoSendMaxUnsentAge = 168 * time.Hour
	}
	if o.LocalPurgeSyncedOlderThan == nil {
		d := 168 * time.Hour
		o.LocalPurgeSyncedOlderThan = &d
	}
	if o.LocalPurgeRunInterval <= 0 {
		o.LocalPurgeRunInterval = o.BackgroundPollInterval
	}
}

// Client batches local records and uploads them via gRPC.
type Client struct {
	store LocalLogStore
	opts  Options

	transport *grpcTransport

	sessionID string
	mu        sync.Mutex
	analytics bool

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClient(store LocalLogStore, opts Options) (*Client, error) {
	if store == nil {
		return nil, errNilStore
	}
	opts.applyDefaults()
	tr, err := newGRPCTransport(opts.GRPCAddress, opts.BearerToken, dialTLSConfig{
		CAPEM:              opts.TLSCAPEM,
		InsecureSkipVerify: opts.InsecureSkipVerify,
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		store:     store,
		opts:      opts,
		transport: tr,
		sessionID: uuid.NewString(),
		analytics: true,
		cancel:    cancel,
	}
	c.wg.Add(1)
	go func() { defer c.wg.Done(); c.syncLoop(ctx) }()
	return c, nil
}

func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	return c.transport.Close()
}

func (c *Client) SetAnalyticsEnabled(on bool) {
	c.mu.Lock()
	c.analytics = on
	c.mu.Unlock()
}

// Log records an operational log line.
func (c *Client) Log(ctx context.Context, level, message string, metadataJSON []byte) (string, error) {
	id := uuid.NewString()
	rec := LocalRecord{
		LogID:             id,
		RecordKind:        "operational",
		Source:            c.opts.Source,
		SourceEnvironment: c.opts.SourceEnvironment,
		SessionID:         c.sessionID,
		ApplicationName:   c.opts.ApplicationName,
		LogMessage:        message,
		MetadataJSON:      append([]byte(nil), metadataJSON...),
		EventTimestamp:    time.Now().UTC(),
		LogLevel:          level,
	}
	if err := c.store.Append(ctx, []LocalRecord{rec}); err != nil {
		return "", err
	}
	return id, nil
}

// Track records an analytics event when analytics is enabled.
func (c *Client) Track(ctx context.Context, eventName string, metadataJSON []byte) (string, error) {
	c.mu.Lock()
	on := c.analytics
	c.mu.Unlock()
	if !on {
		return "", nil
	}
	id := uuid.NewString()
	rec := LocalRecord{
		LogID:              id,
		RecordKind:         "analytics",
		AnalyticsEventName: eventName,
		Source:             c.opts.Source,
		SourceEnvironment:  c.opts.SourceEnvironment,
		SessionID:          c.sessionID,
		ApplicationName:    c.opts.ApplicationName,
		MetadataJSON:       append([]byte(nil), metadataJSON...),
		EventTimestamp:     time.Now().UTC(),
		LogLevel:           "info",
	}
	if err := c.store.Append(ctx, []LocalRecord{rec}); err != nil {
		return "", err
	}
	return id, nil
}

// Flush attempts to upload pending records immediately.
func (c *Client) Flush(ctx context.Context) error {
	return c.uploadPending(ctx)
}
