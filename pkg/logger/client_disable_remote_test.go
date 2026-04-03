package logger

import (
	"context"
	"errors"
	"testing"
)

func TestNewClientDisableRemoteNoDial(t *testing.T) {
	c, err := NewClient(nopStore{}, Options{
		ApplicationName: "app",
		DisableRemote:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	ctx := context.Background()
	if _, err := c.Log(ctx, "info", "local only", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Flush(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientRemoteRequiresTarget(t *testing.T) {
	_, err := NewClient(nopStore{}, Options{ApplicationName: "app"})
	if !errors.Is(err, ErrNoRemoteTarget) {
		t.Fatalf("got %v, want ErrNoRemoteTarget", err)
	}
}
