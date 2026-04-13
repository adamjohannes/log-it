package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestWithEnvConfigLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "error")
	defer os.Unsetenv("LOG_LEVEL")

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithEnvConfig())

	if l.GetLevel() != ERROR {
		t.Errorf("expected level=ERROR from env, got %v", l.GetLevel())
	}

	l.Info("should-drop", nil)
	if buf.Len() != 0 {
		t.Error("expected Info to be dropped at ERROR level")
	}
}

func TestWithEnvConfigFormat(t *testing.T) {
	os.Setenv("LOG_FORMAT", "text")
	defer os.Unsetenv("LOG_FORMAT")

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithEnvConfig())
	l.Info("text-test", nil)

	// Should not be JSON
	var entry map[string]any
	if json.Unmarshal(buf.Bytes(), &entry) == nil {
		t.Error("expected non-JSON output when LOG_FORMAT=text")
	}
}

func TestWithEnvConfigUnset(t *testing.T) {
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("LOG_FORMAT")

	var buf bytes.Buffer
	l := New(&buf, INFO, WithEnvConfig())

	// Should keep original level
	if l.GetLevel() != INFO {
		t.Errorf("expected level=INFO (default), got %v", l.GetLevel())
	}

	l.Info("json-test", nil)

	// Should be JSON (default format)
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected JSON output by default: %v", err)
	}
}

func TestWithEnvConfigWarnAlias(t *testing.T) {
	os.Setenv("LOG_LEVEL", "warn")
	defer os.Unsetenv("LOG_LEVEL")

	l := New(&bytes.Buffer{}, DEBUG, WithEnvConfig())
	if l.GetLevel() != WARNING {
		t.Errorf("expected level=WARNING from 'warn', got %v", l.GetLevel())
	}
}

func TestWithEnvConfigCaseInsensitive(t *testing.T) {
	os.Setenv("LOG_LEVEL", "DEBUG")
	defer os.Unsetenv("LOG_LEVEL")

	l := New(&bytes.Buffer{}, ERROR, WithEnvConfig())
	if l.GetLevel() != DEBUG {
		t.Errorf("expected level=DEBUG from 'DEBUG', got %v", l.GetLevel())
	}
}
