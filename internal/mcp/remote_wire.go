package mcpmod

import (
	"fmt"
	"os"
	"strings"

	"github.com/joshuabednaz/go-logger/internal/config"
)

// OpenRemoteIngest returns a client for MCP_REMOTE_* forwarding, or nil when remote ingest is disabled.
func OpenRemoteIngest(cfg config.Server) (*RemoteLoggerClient, error) {
	if !cfg.MCPRemoteSending || strings.TrimSpace(cfg.MCPRemoteGRPCAddress) == "" {
		return nil, nil
	}
	var pem []byte
	if p := strings.TrimSpace(cfg.MCPRemoteTLSCAPath); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read MCP_REMOTE_TLS_CA_PATH: %w", err)
		}
		pem = b
	}
	return NewRemoteLoggerClient(cfg.MCPRemoteGRPCAddress, cfg.MCPRemoteBearerToken, pem, cfg.MCPRemoteInsecureSkipVerify)
}
