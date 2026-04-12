package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestEnrichSimpleError(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	err := errors.New("connection refused")
	l.Error("failed", map[string]any{"err": err})

	var entry map[string]any
	if e := json.Unmarshal(buf.Bytes(), &entry); e != nil {
		t.Fatal(e)
	}

	if entry["err"] != "connection refused" {
		t.Errorf("expected err string, got %v", entry["err"])
	}
	if entry["err_type"] != "*errors.errorString" {
		t.Errorf("expected err_type=*errors.errorString, got %v", entry["err_type"])
	}
	// No chain for unwrapped error
	if _, exists := entry["err_chain"]; exists {
		t.Error("expected no err_chain for simple error")
	}
}

func TestEnrichWrappedError(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	inner := errors.New("timeout")
	wrapped := fmt.Errorf("query failed: %w", inner)
	l.Error("db error", map[string]any{"err": wrapped})

	var entry map[string]any
	if e := json.Unmarshal(buf.Bytes(), &entry); e != nil {
		t.Fatal(e)
	}

	if entry["err"] != "query failed: timeout" {
		t.Errorf("expected wrapped message, got %v", entry["err"])
	}

	chain, ok := entry["err_chain"].([]any)
	if !ok {
		t.Fatalf("expected err_chain to be array, got %T: %v", entry["err_chain"], entry["err_chain"])
	}
	if len(chain) != 2 {
		t.Errorf("expected chain length 2, got %d", len(chain))
	}
	if chain[0] != "query failed: timeout" {
		t.Errorf("chain[0]: expected outer message, got %v", chain[0])
	}
	if chain[1] != "timeout" {
		t.Errorf("chain[1]: expected inner message, got %v", chain[1])
	}
}

func TestEnrichNonErrorFieldsUntouched(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Info("ok", map[string]any{"status": 200, "name": "test"})

	var entry map[string]any
	if e := json.Unmarshal(buf.Bytes(), &entry); e != nil {
		t.Fatal(e)
	}

	if entry["status"] != float64(200) {
		t.Errorf("expected status=200, got %v", entry["status"])
	}
	if entry["name"] != "test" {
		t.Errorf("expected name=test, got %v", entry["name"])
	}
	// No _type or _chain keys for non-error fields
	if _, exists := entry["status_type"]; exists {
		t.Error("unexpected status_type key")
	}
}

func TestEnrichNilErrorField(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	var nilErr error
	l.Info("maybe-err", map[string]any{"err": nilErr})

	var entry map[string]any
	if e := json.Unmarshal(buf.Bytes(), &entry); e != nil {
		t.Fatal(e)
	}

	// nil error is not an error interface value — should not be enriched
	if _, exists := entry["err_type"]; exists {
		t.Error("expected no err_type for nil error")
	}
}

func TestEnrichMultipleErrors(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	l.Error("multi", map[string]any{
		"db_err":  errors.New("db down"),
		"net_err": errors.New("no route"),
		"count":   42,
	})

	var entry map[string]any
	if e := json.Unmarshal(buf.Bytes(), &entry); e != nil {
		t.Fatal(e)
	}

	if entry["db_err"] != "db down" {
		t.Errorf("expected db_err string, got %v", entry["db_err"])
	}
	if entry["net_err"] != "no route" {
		t.Errorf("expected net_err string, got %v", entry["net_err"])
	}
	if entry["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", entry["count"])
	}
	if _, exists := entry["db_err_type"]; !exists {
		t.Error("expected db_err_type to exist")
	}
	if _, exists := entry["net_err_type"]; !exists {
		t.Error("expected net_err_type to exist")
	}
}

func TestEnrichStringFieldNotError(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	// "err" is a string, not an error — should NOT be enriched
	l.Info("test", map[string]any{"err": "just a string"})

	var entry map[string]any
	if e := json.Unmarshal(buf.Bytes(), &entry); e != nil {
		t.Fatal(e)
	}

	if entry["err"] != "just a string" {
		t.Errorf("expected string value, got %v", entry["err"])
	}
	if _, exists := entry["err_type"]; exists {
		t.Error("expected no err_type for string field")
	}
}

func TestEnrichEmptyFields(t *testing.T) {
	result := enrichErrors(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}

	result = enrichErrors(map[string]any{})
	if len(result) != 0 {
		t.Error("expected empty map for empty input")
	}
}
