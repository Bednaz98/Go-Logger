package config

import (
	"testing"
)

func TestGetenvPort(t *testing.T) {
	t.Setenv("TEST_PORT_X", "")
	n, err := getenvPort("TEST_PORT_X", 5000)
	if err != nil || n != 5000 {
		t.Fatalf("empty env: got %d, %v; want 5000, nil", n, err)
	}

	t.Setenv("TEST_PORT_X", "8080")
	n, err = getenvPort("TEST_PORT_X", 5000)
	if err != nil || n != 8080 {
		t.Fatalf("8080: got %d, %v; want 8080, nil", n, err)
	}

	t.Setenv("TEST_PORT_X", "0")
	_, err = getenvPort("TEST_PORT_X", 5000)
	if err == nil {
		t.Fatal("port 0: want error")
	}

	t.Setenv("TEST_PORT_X", "70000")
	_, err = getenvPort("TEST_PORT_X", 5000)
	if err == nil {
		t.Fatal("port 70000: want error")
	}

	t.Setenv("TEST_PORT_X", "nope")
	_, err = getenvPort("TEST_PORT_X", 5000)
	if err == nil {
		t.Fatal("non-numeric: want error")
	}
}
