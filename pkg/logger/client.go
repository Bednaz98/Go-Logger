package logger

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Options configures device and server clients. Fields used only by the device sync loop
// (batch sizes, poll intervals, purge) are ignored by NewServerClient.
type Options struct {
	ApplicationName string
	// GRPCAddress is the remote gRPC target as host:port (e.g. localhost:5000).
	GRPCAddress string
	// RemoteURL optionally overrides GRPCAddress when non-empty. Use host:port or grpc:// / grpcs://host:port
	// (scheme selects parsing only; TLS still comes from TLSCAPEM / InsecureSkipVerify).
	RemoteURL string
	// DisableRemote, when true, skips the gRPC connection and never uploads; Log/Track still append locally.
	DisableRemote bool
	BearerToken   string

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

// Client is the device-oriented client: it appends to a LocalLogStore and uploads batches in the
// background (or on Flush) when remote sending is enabled. Construct with NewDeviceClient or NewClient.
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

// NewDeviceClient buffers logs in store and syncs to the remote LoggerService according to Options.
func NewDeviceClient(store LocalLogStore, opts Options) (*Client, error) {
	if store == nil {
		return nil, errNilStore
	}
	opts.applyDefaults()
	tr, err := dialGRPC(opts)
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

// NewClient is an alias for NewDeviceClient.
func NewClient(store LocalLogStore, opts Options) (*Client, error) {
	return NewDeviceClient(store, opts)
}

// dialGRPC returns nil transport when opts.DisableRemote is true.
func dialGRPC(opts Options) (*grpcTransport, error) {
	if opts.DisableRemote {
		return nil, nil
	}
	target, err := grpcDialTarget(opts.GRPCAddress, opts.RemoteURL)
	if err != nil {
		return nil, err
	}
	if target == "" {
		return nil, ErrNoRemoteTarget
	}
	return newGRPCTransport(target, opts.BearerToken, dialTLSConfig{
		CAPEM:              opts.TLSCAPEM,
		InsecureSkipVerify: opts.InsecureSkipVerify,
	})
}

func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	if c.transport == nil {
		return nil
	}
	return c.transport.Close()
}

func (c *Client) SetAnalyticsEnabled(on bool) {
	c.mu.Lock()
	c.analytics = on
	c.mu.Unlock()
}

// Log records an operational log line.
func (c *Client) Log(ctx context.Context, level, message string, metadataJSON []byte) (string, error) {
	rec := newOperationalRecord(c.opts, c.sessionID, level, message, metadataJSON)
	if err := c.store.Append(ctx, []LocalRecord{rec}); err != nil {
		return "", err
	}
	return rec.LogID, nil
}

// Track records an analytics event when analytics is enabled.
func (c *Client) Track(ctx context.Context, eventName string, metadataJSON []byte) (string, error) {
	c.mu.Lock()
	on := c.analytics
	c.mu.Unlock()
	if !on {
		return "", nil
	}
	rec := newAnalyticsRecord(c.opts, c.sessionID, eventName, metadataJSON)
	if err := c.store.Append(ctx, []LocalRecord{rec}); err != nil {
		return "", err
	}
	return rec.LogID, nil
}

// Flush attempts to upload pending records immediately.
func (c *Client) Flush(ctx context.Context) error {
	return c.uploadPending(ctx)
}
