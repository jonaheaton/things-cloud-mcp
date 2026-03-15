package thingscloud

import (
	"strings"
	"testing"
)

func TestFormatDebugLogBody(t *testing.T) {
	t.Parallel()

	t.Run("keeps short payloads unchanged", func(t *testing.T) {
		t.Parallel()

		got := formatDebugLogBody([]byte("hello"))
		if got != "hello" {
			t.Fatalf("formatDebugLogBody = %q, want %q", got, "hello")
		}
	})

	t.Run("truncates oversized payloads", func(t *testing.T) {
		t.Parallel()

		input := strings.Repeat("a", maxDebugLogBytes+25)
		got := formatDebugLogBody([]byte(input))
		if !strings.HasPrefix(got, strings.Repeat("a", maxDebugLogBytes)) {
			t.Fatal("expected truncated output to keep the leading bytes")
		}
		if !strings.Contains(got, "...[truncated 25 bytes]") {
			t.Fatalf("expected truncation suffix, got %q", got)
		}
		if len(got) <= maxDebugLogBytes {
			t.Fatal("expected truncation notice to extend output")
		}
	})
}
