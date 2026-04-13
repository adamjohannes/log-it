package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestWithLoggerAndFromContext(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	ctx := WithLogger(context.Background(), l)
	got := FromContext(ctx)

	if got != l {
		t.Error("expected FromContext to return the stored logger")
	}

	got.Info("from-context", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["message"] != "from-context" {
		t.Errorf("expected message=from-context, got %v", entry["message"])
	}
}

func TestFromContextEmptyFallsBackToDefault(t *testing.T) {
	got := FromContext(context.Background())
	if got == nil {
		t.Error("expected non-nil default logger from empty context")
	}
}

func TestFromContextWithChildLogger(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG)
	child := root.With(map[string]any{"component": "handler"})

	ctx := WithLogger(context.Background(), child)
	got := FromContext(ctx)

	if got != child {
		t.Error("expected FromContext to return the child logger")
	}
}
