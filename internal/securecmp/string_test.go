package securecmp

import "testing"

func TestEqual(t *testing.T) {
	if !Equal("abc", "abc") {
		t.Fatal("equal strings")
	}
	if Equal("abc", "abd") {
		t.Fatal("different strings")
	}
	if Equal("a", "ab") {
		t.Fatal("length mismatch")
	}
}
