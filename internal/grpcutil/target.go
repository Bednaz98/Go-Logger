// Package grpcutil holds small shared helpers for gRPC client wiring.
package grpcutil

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseDialTarget returns host:port for grpc.NewClient. Bare "host:port" is accepted as-is.
// URLs with scheme grpc:// or grpcs:// are parsed to host:port only; the caller still chooses
// TLS credentials (there is no plaintext dial in this codebase).
func ParseDialTarget(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("grpc dial target: empty address")
	}
	if !strings.Contains(s, "://") {
		return s, nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("grpc dial target: invalid url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "grpc", "grpcs":
		if u.Host == "" {
			return "", fmt.Errorf("grpc dial target: url missing host")
		}
		return u.Host, nil
	default:
		return "", fmt.Errorf("grpc dial target: unsupported scheme %q (use grpc://host:port, grpcs://host:port, or host:port)", u.Scheme)
	}
}
