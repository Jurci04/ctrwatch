package commands

import "testing"

func TestShortID(t *testing.T) {
	if got := shortID("123456789012345"); got != "123456789012" {
		t.Fatalf("shortID = %q, want 123456789012", got)
	}
	if got := shortID("123"); got != "123" {
		t.Fatalf("shortID = %q, want 123", got)
	}
}
