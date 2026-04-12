package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestFieldConstructors(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		wantKey  string
		wantVal  any
		wantType FieldType
	}{
		{"String", String("name", "alice"), "name", "alice", FieldTypeString},
		{"Int", Int("count", 42), "count", int64(42), FieldTypeInt},
		{"Int64", Int64("big", 9999999999), "big", int64(9999999999), FieldTypeInt64},
		{"Float64", Float64("rate", 3.14), "rate", 3.14, FieldTypeFloat64},
		{"Bool/true", Bool("active", true), "active", true, FieldTypeBool},
		{"Bool/false", Bool("active", false), "active", false, FieldTypeBool},
		{"Duration", Duration("elapsed", 5*time.Second), "elapsed", "5s", FieldTypeDuration},
		{"Any", Any("data", []int{1, 2}), "data", []int{1, 2}, FieldTypeAny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Key != tt.wantKey {
				t.Errorf("key: expected %s, got %s", tt.wantKey, tt.field.Key)
			}
			if tt.field.Type != tt.wantType {
				t.Errorf("type: expected %d, got %d", tt.wantType, tt.field.Type)
			}
			got := tt.field.Value()
			// Compare with type assertion for slices
			switch want := tt.wantVal.(type) {
			case []int:
				gotSlice, ok := got.([]int)
				if !ok || len(gotSlice) != len(want) {
					t.Errorf("value: expected %v, got %v", want, got)
				}
			default:
				if got != want {
					t.Errorf("value: expected %v (%T), got %v (%T)", want, want, got, got)
				}
			}
		})
	}
}

func TestErrFieldConstructor(t *testing.T) {
	err := errors.New("fail")
	f := Err(err)
	if f.Key != "error" {
		t.Errorf("expected key=error, got %s", f.Key)
	}
	if f.Value() != err {
		t.Errorf("expected error value, got %v", f.Value())
	}
}

func TestErrFieldNil(t *testing.T) {
	f := Err(nil)
	if f.Value() != nil {
		t.Errorf("expected nil for Err(nil), got %v", f.Value())
	}
}

func TestFieldsToMap(t *testing.T) {
	m := fieldsToMap([]Field{
		String("name", "bob"),
		Int("age", 30),
		Bool("active", true),
	})

	if m["name"] != "bob" {
		t.Errorf("expected name=bob, got %v", m["name"])
	}
	if m["age"] != int64(30) {
		t.Errorf("expected age=30, got %v", m["age"])
	}
	if m["active"] != true {
		t.Errorf("expected active=true, got %v", m["active"])
	}
}

func TestFieldsToMapEmpty(t *testing.T) {
	m := fieldsToMap(nil)
	if m != nil {
		t.Error("expected nil for empty fields")
	}
	m = fieldsToMap([]Field{})
	if m != nil {
		t.Error("expected nil for zero-length fields")
	}
}

func TestInfowProducesCorrectJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Infow("request",
		String("method", "GET"),
		Int("status", 200),
		Float64("latency", 1.23),
		Bool("cached", true),
	)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level=INFO, got %v", entry["level"])
	}
	if entry["method"] != "GET" {
		t.Errorf("expected method=GET, got %v", entry["method"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("expected status=200, got %v", entry["status"])
	}
	if entry["latency"] != 1.23 {
		t.Errorf("expected latency=1.23, got %v", entry["latency"])
	}
	if entry["cached"] != true {
		t.Errorf("expected cached=true, got %v", entry["cached"])
	}
}

func TestAllTypedMethods(t *testing.T) {
	type logFunc func(*Logger, string, ...Field)
	tests := []struct {
		name  string
		fn    logFunc
		level string
	}{
		{"Debugw", (*Logger).Debugw, "DEBUG"},
		{"Infow", (*Logger).Infow, "INFO"},
		{"Warningw", (*Logger).Warningw, "WARNING"},
		{"Errorw", (*Logger).Errorw, "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, DEBUG)
			tt.fn(l, "test", String("k", "v"))

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["level"] != tt.level {
				t.Errorf("expected level=%s, got %v", tt.level, entry["level"])
			}
			if entry["k"] != "v" {
				t.Errorf("expected k=v, got %v", entry["k"])
			}
		})
	}
}

func TestFatalwCallsExitFunc(t *testing.T) {
	var buf bytes.Buffer
	var exitCode int
	l := New(&buf, DEBUG, withExitFunc(noopExit(&exitCode)))

	l.Fatalw("fatal-typed", String("reason", "test"))

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "FATAL" {
		t.Errorf("expected level=FATAL, got %v", entry["level"])
	}
	if entry["reason"] != "test" {
		t.Errorf("expected reason=test, got %v", entry["reason"])
	}
}

func TestTypedFieldsWithChildLogger(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG)
	child := root.With(map[string]any{"component": "api"})

	child.Infow("request", String("path", "/health"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["component"] != "api" {
		t.Errorf("expected component=api, got %v", entry["component"])
	}
	if entry["path"] != "/health" {
		t.Errorf("expected path=/health, got %v", entry["path"])
	}
}
