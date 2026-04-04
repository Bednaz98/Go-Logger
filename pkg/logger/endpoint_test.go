package logger

import "testing"

func TestGrpcDialTargetRemoteOverridesPrimary(t *testing.T) {
	got, err := grpcDialTarget("primary:1", "grpc://override:5000")
	if err != nil || got != "override:5000" {
		t.Fatalf("grpcDialTarget = %q, %v; want override:5000, nil", got, err)
	}
}

func TestGrpcDialTargetPrimaryOnly(t *testing.T) {
	got, err := grpcDialTarget("only:9", "")
	if err != nil || got != "only:9" {
		t.Fatalf("grpcDialTarget = %q, %v; want only:9, nil", got, err)
	}
}
