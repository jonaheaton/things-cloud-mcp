package main

import (
	"math"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestJSONToolResultWithIndentHandlesMarshalError(t *testing.T) {
	t.Parallel()

	result := jsonToolResultWithIndent(map[string]float64{"bad": math.NaN()}, true)
	if !result.IsError {
		t.Fatal("expected marshal failure to return an error result")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one content item, got %d", len(result.Content))
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if !strings.Contains(text.Text, "failed to encode result JSON") {
		t.Fatalf("unexpected error text: %q", text.Text)
	}
}
