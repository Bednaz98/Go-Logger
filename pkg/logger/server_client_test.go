package logger

import (
	"context"
	"errors"
	"testing"
)

func TestNewServerClientDisableRemote(t *testing.T) {
	_, err := NewServerClient(Options{
		ApplicationName: "app",
		DisableRemote:     true,
		GRPCAddress:       "127.0.0.1:5000",
		InsecureSkipVerify: true,
	})
	if !errors.Is(err, ErrServerClientDisableRemote) {
		t.Fatalf("got %v, want ErrServerClientDisableRemote", err)
	}
}

func TestNewServerClientRequiresTarget(t *testing.T) {
	_, err := NewServerClient(Options{ApplicationName: "app"})
	if !errors.Is(err, ErrNoRemoteTarget) {
		t.Fatalf("got %v, want ErrNoRemoteTarget", err)
	}
}

func TestServerClientFlushNoOp(t *testing.T) {
	c, err := NewServerClient(Options{
		ApplicationName:    "app",
		GRPCAddress:        "127.0.0.1:5000",
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
}
