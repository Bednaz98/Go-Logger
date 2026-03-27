package grpcserver

import (
	"context"
	"strings"

	"github.com/google/uuid"
	loggerv1 "github.com/joshuabednaz/go-logger/gen/go/logger/v1"
	"github.com/joshuabednaz/go-logger/internal/config"
	"github.com/joshuabednaz/go-logger/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoggerService struct {
	loggerv1.UnimplementedLoggerServiceServer
	Repo   *store.Repository
	Config config.Server
}

func (s *LoggerService) IngestBatch(ctx context.Context, req *loggerv1.IngestBatchRequest) (*loggerv1.IngestBatchResponse, error) {
	app := strings.TrimSpace(req.GetApplicationName())
	maxMeta := s.Config.MaxMetadataBytes
	if maxMeta <= 0 {
		maxMeta = 256 * 1024
	}
	if err := ValidateIngestBatch(app, req.GetRecords(), maxMeta, s.Config.EnforceMetadataLimit); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	inputs := make([]store.LogInput, 0, len(req.GetRecords()))
	for _, r := range req.GetRecords() {
		in, err := ProtoToLogInput(r)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		inputs = append(inputs, in)
	}
	n, err := s.Repo.IngestBatch(ctx, app, inputs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
	}
	return &loggerv1.IngestBatchResponse{
		AcceptedCount: uint32(n),
		BatchId:       uuid.NewString(),
	}, nil
}

func (s *LoggerService) ListLogs(ctx context.Context, req *loggerv1.ListLogsRequest) (*loggerv1.ListLogsResponse, error) {
	f, err := listReqToFilter(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if strings.TrimSpace(f.ApplicationName) == "" {
		return nil, status.Error(codes.InvalidArgument, "application_name required")
	}
	rows, next, err := s.Repo.ListLogs(ctx, f)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list failed: %v", err)
	}
	out := make([]*loggerv1.LogRecord, 0, len(rows))
	for i := range rows {
		out = append(out, LogToProto(&rows[i]))
	}
	return &loggerv1.ListLogsResponse{Records: out, NextPageToken: next}, nil
}

func (s *LoggerService) DeleteLogs(ctx context.Context, req *loggerv1.DeleteLogsRequest) (*loggerv1.DeleteLogsResponse, error) {
	f, err := deleteReqToFilter(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if strings.TrimSpace(f.ApplicationName) == "" {
		return nil, status.Error(codes.InvalidArgument, "application_name required")
	}
	n, err := s.Repo.DeleteLogs(ctx, f)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete failed: %v", err)
	}
	return &loggerv1.DeleteLogsResponse{DeletedCount: uint64(n)}, nil
}
