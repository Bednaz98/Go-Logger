package config

import "strings"

// Server holds runtime configuration for the logger server process.
type Server struct {
	DatabaseURL string

	ListenBindAddress string
	GRPCPort          int
	HTTPPort          int
	MCPHTTPPort       int
	MCPHTTPListen     bool

	AuthBearerToken string
	AuthDisabled    bool

	TLS TLSConfig

	EnforceMetadataLimit bool
	MaxMetadataBytes     int

	MaxGRPCRecvBytes int
	MaxGRPCSendBytes int

	MCPEnableDeleteLogs bool
}

type TLSConfig struct {
	CertPath string
	CertPEM  string
	KeyPath  string
	KeyPEM   string

	MustUseProvided bool

	ExtraSANHosts []string
}

func LoadServerFromEnv() Server {
	return Server{
		DatabaseURL: getenv("DATABASE_URL", "file:logger.db?cache=shared"),

		ListenBindAddress: getenv("LISTEN_BIND_ADDRESS", "0.0.0.0"),
		GRPCPort:          getenvInt("GRPC_PORT", 7443),
		HTTPPort:          getenvInt("HTTP_PORT", 8443),
		MCPHTTPPort:       getenvInt("MCP_HTTP_PORT", 8444),
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
	}
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
