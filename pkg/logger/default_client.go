package logger

import (
	"context"
	"errors"
	"sync"
)

var (
	defaultMu     sync.RWMutex
	defaultClient *Client
)

// ErrNotInitialized is returned when there is no active default client (Init never called,
// Close cleared it, or Close was called with no prior Init). Package-level Close with no default returns this as well.
var ErrNotInitialized = errors.New("logger: no active default client")

// ErrAlreadyInitialized is returned when Init is called while a default client is already set.
var ErrAlreadyInitialized = errors.New("logger: Init already called; Close the current client first")

// Init registers the application-owned client as the package default for Log, Track, Flush, and Close.
// The parent must construct the client with NewClient and pass that value here (non-nil).
//
// Only one default may exist at a time; call Close before Init again. Package-level methods hold a
// read lock for the duration of each call so Close cannot run concurrently with Log, Track,
// Flush, or SetAnalyticsEnabled on the default client.
func Init(c *Client) error {
	if c == nil {
		return ErrNilClient
	}
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultClient != nil {
		return ErrAlreadyInitialized
	}
	defaultClient = c
	return nil
}

// Default returns the client set by Init, if any.
func Default() (*Client, bool) {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultClient == nil {
		return nil, false
	}
	return defaultClient, true
}

// Log records an operational log line using the client from Init.
func Log(ctx context.Context, level, message string, metadataJSON []byte) (string, error) {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultClient == nil {
		return "", ErrNotInitialized
	}
	return defaultClient.Log(ctx, level, message, metadataJSON)
}

// Track records an analytics event using the client from Init.
func Track(ctx context.Context, eventName string, metadataJSON []byte) (string, error) {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultClient == nil {
		return "", ErrNotInitialized
	}
	return defaultClient.Track(ctx, eventName, metadataJSON)
}

// Flush uploads pending records for the client from Init.
func Flush(ctx context.Context) error {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultClient == nil {
		return ErrNotInitialized
	}
	return defaultClient.Flush(ctx)
}

// SetAnalyticsEnabled updates the client registered with Init.
func SetAnalyticsEnabled(on bool) error {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultClient == nil {
		return ErrNotInitialized
	}
	defaultClient.SetAnalyticsEnabled(on)
	return nil
}

// Close stops the client registered with Init and clears the package default only if shutdown succeeds.
// If Close returns a non-nil error, the default remains set so the caller can retry or replace the client after handling the failure.
func Close() error {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultClient == nil {
		return ErrNotInitialized
	}
	err := defaultClient.Close()
	if err != nil {
		return err
	}
	defaultClient = nil
	return nil
}
