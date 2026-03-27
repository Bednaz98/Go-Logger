package grpcserver

import (
	"crypto/tls"
	"net"
	"strconv"

	"log/slog"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/config"
	"github.com/joshuabednaz/go-logger/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Serve starts a TLS gRPC listener. Caller should invoke srv.GracefulStop() for shutdown.
func Serve(addr string, tlsCert tls.Certificate, cfg config.Server, repo *store.Repository) (*grpc.Server, error) {
	c := tlsCert
	creds := credentials.NewServerTLSFromCert(&c)

	opts := []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.MaxRecvMsgSize(cfg.MaxGRPCRecvBytes),
		grpc.MaxSendMsgSize(cfg.MaxGRPCSendBytes),
		grpc.ChainUnaryInterceptor(AuthUnaryInterceptor(cfg)),
	}

	srv := grpc.NewServer(opts...)
	svc := &LoggerService{Repo: repo, Config: cfg}
	loggerv1.RegisterLoggerServiceServer(srv, svc)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	slog.Info("grpc listening", "addr", addr)
	go func() {
		if err := srv.Serve(ln); err != nil {
			slog.Error("grpc serve", "error", err)
		}
	}()
	return srv, nil
}

func ListenAddr(bind string, port int) string {
	return net.JoinHostPort(bind, strconv.Itoa(port))
}
