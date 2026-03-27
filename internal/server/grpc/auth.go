package grpcserver

import (
	"context"
	"strings"

	"github.com/joshuabednaz/go-logger/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const authHeader = "authorization"

func AuthUnaryInterceptor(cfg config.Server) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if cfg.AuthDisabled || strings.TrimSpace(cfg.AuthBearerToken) == "" {
			return handler(ctx, req)
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get(authHeader)
		if len(vals) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization")
		}
		raw := strings.TrimSpace(vals[0])
		const prefix = "Bearer "
		if !strings.HasPrefix(strings.ToLower(raw), strings.ToLower(prefix)) {
			return nil, status.Error(codes.Unauthenticated, "authorization must be Bearer token")
		}
		tok := strings.TrimSpace(raw[len(prefix):])
		if tok != cfg.AuthBearerToken {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		return handler(ctx, req)
	}
}
