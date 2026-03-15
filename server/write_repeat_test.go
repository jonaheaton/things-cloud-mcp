package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
)

func decodeRepeatConfig(t *testing.T, raw *json.RawMessage) map[string]any {
	t.Helper()
	if raw == nil {
		t.Fatal("expected repeat rule, got nil")
	}
	var cfg map[string]any
	if err := json.Unmarshal(*raw, &cfg); err != nil {
		t.Fatalf("failed to unmarshal repeat config: %v", err)
	}
	return cfg
}

func numberField(t *testing.T, cfg map[string]any, key string) float64 {
	t.Helper()
	v, ok := cfg[key]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("key %q is not numeric: %#v", key, v)
	}
	return n
}

func TestBuildRepeatRule_UntilDate(t *testing.T) {
	ref := time.Date(2026, 2, 17, 15, 30, 0, 0, time.UTC)
	raw, err := buildRepeatRule("daily until 2026-02-24", ref)
	if err != nil {
		t.Fatalf("buildRepeatRule returned error: %v", err)
	}
	cfg := decodeRepeatConfig(t, raw)

	wantStart := float64(time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC).Unix())
	wantEnd := float64(time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC).Unix())

	if got := numberField(t, cfg, "fu"); got != 16 {
		t.Fatalf("expected fu=16 (daily), got %.0f", got)
	}
	if got := numberField(t, cfg, "tp"); got != 0 {
		t.Fatalf("expected tp=0, got %.0f", got)
	}
	if got := numberField(t, cfg, "sr"); got != wantStart {
		t.Fatalf("expected sr=%.0f, got %.0f", wantStart, got)
	}
	if got := numberField(t, cfg, "ed"); got != wantEnd {
		t.Fatalf("expected ed=%.0f, got %.0f", wantEnd, got)
	}
}

func TestBuildRepeatRule_UntilDateWithAfterCompletionEitherOrder(t *testing.T) {
	ref := time.Date(2026, 2, 17, 9, 0, 0, 0, time.UTC)
	wantEnd := float64(time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC).Unix())

	cases := []string{
		"daily after completion until 2026-02-24",
		"daily until 2026-02-24 after completion",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			raw, err := buildRepeatRule(input, ref)
			if err != nil {
				t.Fatalf("buildRepeatRule returned error: %v", err)
			}
			cfg := decodeRepeatConfig(t, raw)
			if got := numberField(t, cfg, "tp"); got != 1 {
				t.Fatalf("expected tp=1, got %.0f", got)
			}
			if got := numberField(t, cfg, "ed"); got != wantEnd {
				t.Fatalf("expected ed=%.0f, got %.0f", wantEnd, got)
			}
		})
	}
}

func TestBuildRepeatRule_InvalidEndDate(t *testing.T) {
	ref := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	_, err := buildRepeatRule("daily until 02-24-2026", ref)
	if err == nil {
		t.Fatal("expected error for invalid end date format")
	}
	if !strings.Contains(err.Error(), "invalid repeat end date") {
		t.Fatalf("expected invalid repeat end date error, got: %v", err)
	}
}

func TestBuildRepeatRule_EndDateBeforeStart(t *testing.T) {
	ref := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	_, err := buildRepeatRule("daily until 2026-02-16", ref)
	if err == nil {
		t.Fatal("expected error for end date before start date")
	}
	if !strings.Contains(err.Error(), "on or after start date") {
		t.Fatalf("expected on-or-after-start-date error, got: %v", err)
	}
}

func TestBuildRepeatRule_DefaultsToNeverendingSentinel(t *testing.T) {
	ref := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	raw, err := buildRepeatRule("daily", ref)
	if err != nil {
		t.Fatalf("buildRepeatRule returned error: %v", err)
	}
	cfg := decodeRepeatConfig(t, raw)

	wantEnd := float64(time.Date(things.NeverendingRepeatYear, time.January, 1, 0, 0, 0, 0, time.UTC).Unix())
	if got := numberField(t, cfg, "ed"); got != wantEnd {
		t.Fatalf("expected ed=%.0f, got %.0f", wantEnd, got)
	}
}
