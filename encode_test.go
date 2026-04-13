package logger

import (
	"encoding/json"
	"math"
	"testing"
)

func TestAppendJSONEntryBasic(t *testing.T) {
	entry := map[string]any{
		"level":   "INFO",
		"message": "hello",
		"count":   42,
	}
	got := string(appendJSONEntry(nil, entry))

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\ngot: %s", err, got)
	}
	if parsed["level"] != "INFO" {
		t.Errorf("level: got %v", parsed["level"])
	}
	if parsed["message"] != "hello" {
		t.Errorf("message: got %v", parsed["message"])
	}
	if parsed["count"] != float64(42) {
		t.Errorf("count: got %v", parsed["count"])
	}
}

func TestAppendJSONStringEscaping(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `"hello"`},
		{`say "hi"`, `"say \"hi\""`},
		{"line\nbreak", `"line\nbreak"`},
		{"tab\there", `"tab\there"`},
		{"cr\rreturn", `"cr\rreturn"`},
		{`back\slash`, `"back\\slash"`},
		{"\x00null", `"\u0000null"`},
		{"\x1fcontrol", `"\u001fcontrol"`},
		{"unicode: 日本語", `"unicode: 日本語"`},
	}
	for _, tt := range tests {
		got := string(appendJSONString(nil, tt.input))
		if got != tt.want {
			t.Errorf("appendJSONString(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestAppendJSONValueTypes(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"nil", nil, "null"},
		{"string", "hi", `"hi"`},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", 42, "42"},
		{"int64", int64(-100), "-100"},
		{"uint64", uint64(999), "999"},
		{"float64", 3.14, "3.14"},
		{"float64 int", float64(1), "1"},
		{"string slice", []string{"a", "b"}, `["a","b"]`},
		{"empty string slice", []string{}, "[]"},
		{"nested map", map[string]any{"k": "v"}, `{"k":"v"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(appendJSONValue(nil, tt.val))
			if got != tt.want {
				t.Errorf("appendJSONValue(%v) = %s, want %s", tt.val, got, tt.want)
			}
		})
	}
}

func TestAppendJSONFloatSpecialValues(t *testing.T) {
	// NaN and Inf are encoded as strings since JSON doesn't support them
	got := string(appendJSONValue(nil, math.NaN()))
	var s string
	if err := json.Unmarshal([]byte(got), &s); err != nil {
		t.Fatalf("NaN encoding not valid JSON string: %s", got)
	}

	got = string(appendJSONValue(nil, math.Inf(1)))
	if err := json.Unmarshal([]byte(got), &s); err != nil {
		t.Fatalf("Inf encoding not valid JSON string: %s", got)
	}
}

func TestAppendJSONEntrySortedKeys(t *testing.T) {
	entry := map[string]any{
		"z": 1,
		"a": 2,
		"m": 3,
	}
	got := string(appendJSONEntry(nil, entry))
	// Keys should be sorted alphabetically
	expected := `{"a":2,"m":3,"z":1}`
	if got != expected {
		t.Errorf("got %s, want %s", got, expected)
	}
}

func TestAppendJSONEmptyEntry(t *testing.T) {
	got := string(appendJSONEntry(nil, map[string]any{}))
	if got != "{}" {
		t.Errorf("got %s, want {}", got)
	}
}

func TestCustomEncoderMatchesStdlib(t *testing.T) {
	// Verify our encoder produces output that json.Unmarshal can parse
	// for a realistic log entry
	entry := map[string]any{
		"time":       "2026-04-12T14:32:07.123456789Z",
		"level":      "ERROR",
		"message":    "request failed",
		"status":     500,
		"latency_ms": 42.5,
		"user_id":    "u-123",
		"tags":       []string{"api", "v2"},
		"error":      "connection refused",
		"retry":      true,
		"metadata":   map[string]any{"region": "us-east-1", "az": "1a"},
	}

	got := appendJSONEntry(nil, entry)
	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("custom encoder output not valid JSON: %v\ngot: %s", err, got)
	}

	// Spot check values
	if parsed["status"] != float64(500) {
		t.Errorf("status: got %v", parsed["status"])
	}
	if parsed["retry"] != true {
		t.Errorf("retry: got %v", parsed["retry"])
	}
	meta, ok := parsed["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a nested map")
	}
	if meta["region"] != "us-east-1" {
		t.Errorf("metadata.region: got %v", meta["region"])
	}
}
