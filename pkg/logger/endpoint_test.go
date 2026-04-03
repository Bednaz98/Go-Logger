package logger

import "testing"

func TestGrpcDialTargetRemoteOverridesPrimary(t *testing.T) {
	got, err := grpcDialTarget("primary:1", "grpc://override:7443")
	if err != nil || got != "override:7443" {
		t.Fatalf("grpcDialTarget = %q, %v; want override:7443, nil", got, err)
	}
}

func TestGrpcDialTargetPrimaryOnly(t *testing.T) {
	got, err := grpcDialTarget("only:9", "")
	if err != nil || got != "only:9" {
		t.Fatalf("grpcDialTarget = %q, %v; want only:9, nil", got, err)
	}
}
