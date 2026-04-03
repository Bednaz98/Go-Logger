package mcpmod

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// RemoteLoggerClient is a gRPC LoggerService client used to forward MCP ingest batches to a remote server.
type RemoteLoggerClient struct {
	conn   *grpc.ClientConn
	client loggerv1.LoggerServiceClient
	token  string
}

// NewRemoteLoggerClient dials target (host:port or grpc:// / grpcs://) with TLS.
// grpc/grpc schemes are parsed to host:port only; this client always uses TLS credentials (CA or insecure).
func NewRemoteLoggerClient(target, bearer string, caPEM []byte, insecureSkipVerify bool) (*RemoteLoggerClient, error) {
	target, err := grpcutil.ParseDialTarget(target)
	if err != nil {
		return nil, err
	}
	var creds credentials.TransportCredentials
	if insecureSkipVerify {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	} else if len(caPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("mcp remote: invalid tls CA PEM")
		}
		creds = credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
	} else {
		return nil, fmt.Errorf("mcp remote: set MCP_REMOTE_TLS_CA_PATH or MCP_REMOTE_INSECURE_SKIP_VERIFY=true")
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &RemoteLoggerClient{
		conn:   conn,
		client: loggerv1.NewLoggerServiceClient(conn),
		token:  bearer,
	}, nil
}

// IngestBatch forwards records to the remote LoggerService (same contract as gRPC ingest).
func (r *RemoteLoggerClient) IngestBatch(ctx context.Context, applicationName string, records []*loggerv1.LogRecord) (uint32, error) {
	if r == nil {
		return 0, fmt.Errorf("mcp remote: nil client")
	}
	ctx = r.withAuth(ctx)
	resp, err := r.client.IngestBatch(ctx, &loggerv1.IngestBatchRequest{
		ApplicationName: applicationName,
		Records:         records,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetAcceptedCount(), nil
}

func (r *RemoteLoggerClient) withAuth(ctx context.Context) context.Context {
	tok := strings.TrimSpace(r.token)
	if tok == "" {
		return ctx
	}
	md := metadata.Pairs("authorization", "Bearer "+tok)
	return metadata.NewOutgoingContext(ctx, md)
}

// Close closes the gRPC connection.
func (r *RemoteLoggerClient) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}
	return r.conn.Close()
}
