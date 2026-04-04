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
	cfg, err := config.LoadServerFromEnv()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

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

	remote, err := mcpmod.OpenRemoteIngest(cfg)
	if err != nil {
		if cfg.MCPRemoteStrict {
			slog.Error("mcp remote ingest", "error", err)
			os.Exit(1)
		}
		slog.Warn("mcp remote forward disabled due to config error", "error", err)
		remote = nil
	}
	if remote != nil {
		defer func() { _ = remote.Close() }()
		slog.Info("mcp remote forward enabled", "grpc", cfg.MCPRemoteGRPCAddress)
	}

	mcpCfg := mcpmod.ToolConfig{
		EnableDeleteLogs:     cfg.MCPEnableDeleteLogs,
		MaxMetadataBytes:     cfg.MaxMetadataBytes,
		EnforceMetadataLimit: cfg.EnforceMetadataLimit,
		RemoteIngest:         remote,
	}
	if err := mcpmod.RunStdio(context.Background(), repo, mcpCfg); err != nil {
		slog.Error("mcp", "error", err)
		os.Exit(1)
	}
}
