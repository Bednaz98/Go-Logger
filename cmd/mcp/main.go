package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joshuabednaz/go-logger/internal/config"
	mcpmod "github.com/joshuabednaz/go-logger/internal/mcp"
	"github.com/joshuabednaz/go-logger/internal/observability"
	"github.com/joshuabednaz/go-logger/internal/store"
)

func main() {
	observability.InitSlog()
	cfg := config.LoadServerFromEnv()

	db, dialect, err := store.OpenDB(cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	if err := store.AutoMigrate(db); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}
	repo := store.NewRepository(db, dialect)
	slog.Info("mcp stdio starting", "dialect", dialect)

	mcpCfg := mcpmod.ToolConfig{EnableDeleteLogs: cfg.MCPEnableDeleteLogs}
	if err := mcpmod.RunStdio(context.Background(), repo, mcpCfg); err != nil {
		slog.Error("mcp", "error", err)
		os.Exit(1)
	}
}
