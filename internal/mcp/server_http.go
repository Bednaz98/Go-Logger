package mcpmod

import (
	"net/http"
	"strings"

	"github.com/joshuabednaz/go-logger/internal/config"
	"github.com/joshuabednaz/go-logger/internal/securecmp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StreamableHTTPHandler returns an MCP streamable HTTP handler with optional Bearer auth.
func StreamableHTTPHandler(srv *mcp.Server, cfg config.Server) http.Handler {
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	if cfg.AuthDisabled || strings.TrimSpace(cfg.AuthBearerToken) == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimSpace(r.Header.Get("Authorization"))
		const p = "Bearer "
		if !strings.HasPrefix(strings.ToLower(raw), strings.ToLower(p)) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tok := strings.TrimSpace(raw[len(p):])
		if !securecmp.Equal(tok, cfg.AuthBearerToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}
