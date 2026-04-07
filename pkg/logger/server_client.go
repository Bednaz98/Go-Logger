package logger

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// ServerClient sends each log line directly to the remote LoggerService over gRPC with no local
// persistence or background sync. Use it from backend services that can reach the logger server
// and do not need offline buffering. For edge devices and local queues, use NewDeviceClient.
type ServerClient struct {
	opts      Options
	transport *grpcTransport
	sessionID string
	mu        sync.Mutex
	analytics bool
}

// NewServerClient opens a gRPC connection and sends Log/Track calls via IngestBatch immediately.
// Options.DisableRemote must be false; set GRPCAddress or RemoteURL, TLS fields, and ApplicationName.
// Store-related options (batch thresholds, purge intervals) are ignored.
func NewServerClient(opts Options) (*ServerClient, error) {
	if opts.DisableRemote {
		return nil, ErrServerClientDisableRemote
	}
	opts.applyDefaults()
	tr, err := dialGRPC(opts)
	if err != nil {
		return nil, err
	}
	return &ServerClient{
		opts:      opts,
		transport: tr,
		sessionID: uuid.NewString(),
		analytics: true,
	}, nil
}

// Close closes the gRPC connection.
func (s *ServerClient) Close() error {
	if s == nil || s.transport == nil {
		return nil
	}
	return s.transport.Close()
}

// SetAnalyticsEnabled controls whether Track sends analytics records.
func (s *ServerClient) SetAnalyticsEnabled(on bool) {
	s.mu.Lock()
	s.analytics = on
	s.mu.Unlock()
}

// Log sends one operational record to the server immediately. On RPC failure it returns ("", err)
// so the log id is only returned when the server accepted the batch.
func (s *ServerClient) Log(ctx context.Context, level, message string, metadataJSON []byte) (string, error) {
	rec := newOperationalRecord(s.opts, s.sessionID, level, message, metadataJSON)
	if _, err := s.transport.IngestBatch(ctx, s.opts.ApplicationName, []LocalRecord{rec}); err != nil {
		return "", err
	}
	return rec.LogID, nil
}

// Track sends one analytics record when analytics is enabled.
func (s *ServerClient) Track(ctx context.Context, eventName string, metadataJSON []byte) (string, error) {
	s.mu.Lock()
	on := s.analytics
	s.mu.Unlock()
	if !on {
		return "", nil
	}
	rec := newAnalyticsRecord(s.opts, s.sessionID, eventName, metadataJSON)
	if _, err := s.transport.IngestBatch(ctx, s.opts.ApplicationName, []LocalRecord{rec}); err != nil {
		return "", err
	}
	return rec.LogID, nil
}

// Flush is a no-op for ServerClient; there is no local queue.
func (s *ServerClient) Flush(_ context.Context) error {
	return nil
}
