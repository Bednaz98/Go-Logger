package logger

import (
	"strings"

	"github.com/joshuabednaz/go-logger/internal/grpcutil"
)

// grpcDialTarget returns the gRPC dial target (host:port). If remoteURL is non-empty it is parsed
// (bare host:port or grpc:// / grpcs:// — see grpcutil.ParseDialTarget); otherwise primary is used.
// TLS is always configured separately by the client (CA PEM or insecure skip verify); schemes are for parsing only.
func grpcDialTarget(primary, remoteURL string) (string, error) {
	if u := strings.TrimSpace(remoteURL); u != "" {
		return grpcutil.ParseDialTarget(u)
	}
	return strings.TrimSpace(primary), nil
}
