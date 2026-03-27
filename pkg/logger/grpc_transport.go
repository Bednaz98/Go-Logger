package logger

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type dialTLSConfig struct {
	CAPEM              []byte
	InsecureSkipVerify bool
}

type grpcTransport struct {
	conn   *grpc.ClientConn
	client loggerv1.LoggerServiceClient
	token  string
}

func newGRPCTransport(target, bearer string, tlsCfg dialTLSConfig) (*grpcTransport, error) {
	var creds credentials.TransportCredentials
	if tlsCfg.InsecureSkipVerify {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	} else if len(tlsCfg.CAPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(tlsCfg.CAPEM) {
			return nil, fmt.Errorf("logger: invalid tls CA PEM")
		}
		creds = credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
	} else {
		return nil, fmt.Errorf("logger: provide TLSCAPEM or enable InsecureSkipVerify for TLS servers")
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &grpcTransport{
		conn:   conn,
		client: loggerv1.NewLoggerServiceClient(conn),
		token:  bearer,
	}, nil
}

func (g *grpcTransport) Close() error {
	if g == nil || g.conn == nil {
		return nil
	}
	return g.conn.Close()
}

func (g *grpcTransport) IngestBatch(ctx context.Context, applicationName string, recs []LocalRecord) (uint32, error) {
	ctx = g.withAuth(ctx)
	records := make([]*loggerv1.LogRecord, 0, len(recs))
	for i := range recs {
		records = append(records, localToProto(&recs[i]))
	}
	resp, err := g.client.IngestBatch(ctx, &loggerv1.IngestBatchRequest{
		ApplicationName: applicationName,
		Records:         records,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetAcceptedCount(), nil
}

func (g *grpcTransport) withAuth(ctx context.Context) context.Context {
	tok := strings.TrimSpace(g.token)
	if tok == "" {
		return ctx
	}
	md := metadata.Pairs("authorization", "Bearer "+tok)
	return metadata.NewOutgoingContext(ctx, md)
}

func localToProto(l *LocalRecord) *loggerv1.LogRecord {
	kind := loggerv1.RecordKind_RECORD_KIND_UNSPECIFIED
	switch strings.ToLower(strings.TrimSpace(l.RecordKind)) {
	case "operational":
		kind = loggerv1.RecordKind_RECORD_KIND_OPERATIONAL
	case "analytics":
		kind = loggerv1.RecordKind_RECORD_KIND_ANALYTICS
	}
	lvl := loggerv1.LogLevel_LOG_LEVEL_INFO
	switch strings.ToLower(strings.TrimSpace(l.LogLevel)) {
	case "trace":
		lvl = loggerv1.LogLevel_LOG_LEVEL_TRACE
	case "debug":
		lvl = loggerv1.LogLevel_LOG_LEVEL_DEBUG
	case "info", "":
		lvl = loggerv1.LogLevel_LOG_LEVEL_INFO
	case "warn", "warning":
		lvl = loggerv1.LogLevel_LOG_LEVEL_WARN
	case "error":
		lvl = loggerv1.LogLevel_LOG_LEVEL_ERROR
	case "fatal":
		lvl = loggerv1.LogLevel_LOG_LEVEL_FATAL
	}
	ts := l.EventTimestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return &loggerv1.LogRecord{
		LogId:              l.LogID,
		RecordKind:         kind,
		AnalyticsEventName: l.AnalyticsEventName,
		UserActorId:        l.UserActorID,
		Source:             l.Source,
		SourceEnvironment:  l.SourceEnvironment,
		SessionId:          l.SessionID,
		ApplicationName:    l.ApplicationName,
		LogMessage:         l.LogMessage,
		MetadataJson:       append([]byte(nil), l.MetadataJSON...),
		EventTimestamp:     ts.UTC().Format(time.RFC3339Nano),
		LogLevel:           lvl,
		TraceId:            l.TraceID,
		SpanId:             l.SpanID,
	}
}
