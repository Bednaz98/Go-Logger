package grpcutil

import "testing"

func TestParseDialTarget(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"localhost:7443", "localhost:7443"},
		{"grpc://example.com:7443", "example.com:7443"},
		{"GRPC://10.0.0.1:443", "10.0.0.1:443"},
		{"grpcs://api.example:7443", "api.example:7443"},
	}
	for _, tc := range tests {
		got, err := ParseDialTarget(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("ParseDialTarget(%q) = %q, %v; want %q, nil", tc.in, got, err, tc.want)
		}
	}
	if _, err := ParseDialTarget("https://x:1"); err == nil {
		t.Fatal("expected error for https scheme")
	}
}
