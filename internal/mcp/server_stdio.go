package mcpmod

import (
	"context"

	"github.com/joshuabednaz/go-logger/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewMCPServer(repo *store.Repository, cfg ToolConfig) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "go-logger", Version: "0.1.0"}, nil)
	RegisterTools(s, repo, cfg)
	return s
}

func RunStdio(ctx context.Context, repo *store.Repository, cfg ToolConfig) error {
	s := NewMCPServer(repo, cfg)
	return s.Run(ctx, &mcp.StdioTransport{})
}
