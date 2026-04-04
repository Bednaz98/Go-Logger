package config

import (
	"errors"
	"os"
	"strings"
)

// Server holds runtime configuration for the logger server process.
type Server struct {
	DatabaseURL string

	ListenBindAddress string
	GRPCPort          int
	HTTPPort          int
	// HTTPPlainListen enables a second listener with the same /api/v1 routes over cleartext HTTP (no TLS).
	HTTPPlainListen bool
	HTTPPlainPort   int
	MCPHTTPPort     int
	MCPHTTPListen   bool

	AuthBearerToken string
	AuthDisabled    bool

	TLS TLSConfig

	EnforceMetadataLimit bool
	MaxMetadataBytes     int

	MaxGRPCRecvBytes int
	MaxGRPCSendBytes int

	MCPEnableDeleteLogs bool

	// MCPRemote* configures optional forwarding of MCP ingest_batch to another LoggerService (stdio or HTTP MCP).
	MCPRemoteGRPCAddress        string
	MCPRemoteSending            bool
	MCPRemoteBearerToken        string
	MCPRemoteTLSCAPath          string
	MCPRemoteInsecureSkipVerify bool
	// MCPRemoteStrict, when true, makes invalid remote ingest config fatal at startup instead of disabling forward only.
	MCPRemoteStrict bool
}

type TLSConfig struct {
	CertPath string
	CertPEM  string
	KeyPath  string
	KeyPEM   string

	MustUseProvided bool

	ExtraSANHosts []string
}

func LoadServerFromEnv() (Server, error) {
	mcpRemoteGRPC := getenv("MCP_REMOTE_GRPC_ADDRESS", "")
	grpcPort, e1 := getenvPort("GRPC_PORT", 5000)
	httpPort, e2 := getenvPort("HTTP_PORT", 5001)
	plainPort, e3 := getenvPort("HTTP_PLAIN_PORT", 5003)
	mcpHTTPPort, e4 := getenvPort("MCP_HTTP_PORT", 5002)
	if err := errors.Join(e1, e2, e3, e4); err != nil {
		return Server{}, err
	}
	return Server{
		DatabaseURL: getenv("DATABASE_URL", "file:logger.db?cache=shared"),

		ListenBindAddress: getenv("LISTEN_BIND_ADDRESS", "0.0.0.0"),
		GRPCPort:          grpcPort,
		HTTPPort:          httpPort,
		HTTPPlainListen:   getenvBool("HTTP_PLAIN_LISTEN", false),
		HTTPPlainPort:     plainPort,
		MCPHTTPPort:       mcpHTTPPort,
		MCPHTTPListen:     getenvBool("MCP_HTTP_LISTEN", true),

		AuthBearerToken: getenv("LOGGER_AUTH_TOKEN", ""),
		AuthDisabled:    getenvBool("LOGGER_AUTH_DISABLED", false),

		TLS: TLSConfig{
			CertPath:        getenv("TLS_CERT_PATH", ""),
			CertPEM:         getenv("TLS_CERT_PEM", ""),
			KeyPath:         getenv("TLS_KEY_PATH", ""),
			KeyPEM:          getenv("TLS_KEY_PEM", ""),
			MustUseProvided: getenvBool("TLS_MUST_USE_PROVIDED_CERT", false),
			ExtraSANHosts:   splitComma(getenv("TLS_EXTRA_SAN_HOSTS", "")),
		},

		EnforceMetadataLimit: getenvBool("LOGGER_ENFORCE_METADATA_LIMIT", true),
		MaxMetadataBytes:     getenvInt("LOGGER_MAX_METADATA_BYTES", 256*1024),

		MaxGRPCRecvBytes: getenvInt("LOGGER_GRPC_MAX_RECV_BYTES", 4*1024*1024),
		MaxGRPCSendBytes: getenvInt("LOGGER_GRPC_MAX_SEND_BYTES", 4*1024*1024),

		MCPEnableDeleteLogs: getenvBool("MCP_ENABLE_DELETE_LOGS", false),

		MCPRemoteGRPCAddress: mcpRemoteGRPC,
		MCPRemoteSending:     mcpRemoteSendingFromEnv(mcpRemoteGRPC),
		// When MCP_REMOTE_BEARER_TOKEN is unset, LOGGER_AUTH_TOKEN is reused for outbound gRPC to the remote (dev convenience only; production should set an explicit remote token).
		MCPRemoteBearerToken:        firstNonEmpty(getenv("MCP_REMOTE_BEARER_TOKEN", ""), getenv("LOGGER_AUTH_TOKEN", "")),
		MCPRemoteTLSCAPath:          getenv("MCP_REMOTE_TLS_CA_PATH", ""),
		MCPRemoteInsecureSkipVerify: getenvBool("MCP_REMOTE_INSECURE_SKIP_VERIFY", false),
		MCPRemoteStrict:             getenvBool("MCP_REMOTE_STRICT", false),
	}, nil
}

func mcpRemoteSendingFromEnv(grpcAddr string) bool {
	if strings.TrimSpace(grpcAddr) == "" {
		return false
	}
	v := strings.TrimSpace(os.Getenv("MCP_REMOTE_SENDING"))
	if v == "" {
		return true
	}
	return getenvBool("MCP_REMOTE_SENDING", false)
}

func firstNonEmpty(a, b string) string {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a != "" {
		return a
	}
	return b
}

func splitComma(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
