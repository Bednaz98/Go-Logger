package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joshuabednaz/go-logger/internal/config"
	mcpmod "github.com/joshuabednaz/go-logger/internal/mcp"
	"github.com/joshuabednaz/go-logger/internal/observability"
	grpcserver "github.com/joshuabednaz/go-logger/internal/server/grpc"
	httpserver "github.com/joshuabednaz/go-logger/internal/server/http"
	"github.com/joshuabednaz/go-logger/internal/store"
	"github.com/joshuabednaz/go-logger/internal/tlsconfig"
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
	slog.Info("database ready", "dialect", dialect)

	tlsRes, err := tlsconfig.Load(cfg.TLS)
	if err != nil {
		slog.Error("tls", "error", err)
		os.Exit(1)
	}
	if tlsRes.AutoGen {
		slog.Warn("tls auto-generated", "fingerprint_sha256", tlsRes.Fingerprint)
	}

	if strings.TrimSpace(cfg.AuthBearerToken) == "" && !cfg.AuthDisabled {
		slog.Warn("LOGGER_AUTH_TOKEN empty: gRPC/HTTPS/MCP HTTP auth is disabled (set LOGGER_AUTH_DISABLED=true to silence)")
	}

	grpcAddr := grpcserver.ListenAddr(cfg.ListenBindAddress, cfg.GRPCPort)
	grpcSrv, err := grpcserver.Serve(grpcAddr, tlsRes.Certificate, cfg, repo)
	if err != nil {
		slog.Error("grpc", "error", err)
		os.Exit(1)
	}

	httpAddr := listenAddr(cfg.ListenBindAddress, cfg.HTTPPort)
	router := httpserver.NewRouter(cfg, repo)
	httpSrv := &http.Server{
		Addr:    httpAddr,
		Handler: router,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsRes.Certificate},
			MinVersion:   tls.VersionTLS12,
		},
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("https listening", "addr", httpAddr)
		if err := httpSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			slog.Error("https", "error", err)
		}
	}()

	var mcpSrv *http.Server
	if cfg.MCPHTTPListen {
		mcpImpl := mcpmod.NewMCPServer(repo, mcpmod.ToolConfig{EnableDeleteLogs: cfg.MCPEnableDeleteLogs})
		h := mcpmod.StreamableHTTPHandler(mcpImpl, cfg)
		mcpAddr := listenAddr(cfg.ListenBindAddress, cfg.MCPHTTPPort)
		mcpSrv = &http.Server{
			Addr:              mcpAddr,
			Handler:           h,
			TLSConfig:         httpSrv.TLSConfig,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("mcp https listening", "addr", mcpAddr)
			if err := mcpSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				slog.Error("mcp https", "error", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	<-ctx.Done()
	stop()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	if mcpSrv != nil {
		_ = mcpSrv.Shutdown(shutdownCtx)
	}
	grpcSrv.GracefulStop()
}

func listenAddr(bind string, port int) string {
	if strings.TrimSpace(bind) == "" {
		bind = "0.0.0.0"
	}
	return net.JoinHostPort(bind, strconv.Itoa(port))
}
