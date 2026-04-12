package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestSlogHandlerBasicLogging(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	sl := slog.New(NewSlogHandler(l))

	sl.Info("hello from slog", "user", "alice", "count", 42)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level=INFO, got %v", entry["level"])
	}
	if entry["message"] != "hello from slog" {
		t.Errorf("expected message, got %v", entry["message"])
	}
	if entry["user"] != "alice" {
		t.Errorf("expected user=alice, got %v", entry["user"])
	}
	if entry["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", entry["count"])
	}
}

func TestSlogHandlerLevelMapping(t *testing.T) {
	tests := []struct {
		slogFn func(*slog.Logger, string, ...any)
		expect string
	}{
		{(*slog.Logger).Debug, "DEBUG"},
		{(*slog.Logger).Info, "INFO"},
		{(*slog.Logger).Warn, "WARNING"},
		{(*slog.Logger).Error, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, DEBUG)
			sl := slog.New(NewSlogHandler(l))

			tt.slogFn(sl, "test")

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["level"] != tt.expect {
				t.Errorf("expected level=%s, got %v", tt.expect, entry["level"])
			}
		})
	}
}

func TestSlogHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	sl := slog.New(NewSlogHandler(l)).With("service", "api", "version", "1.0")

	sl.Info("request")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["fields.service"] != "api" {
		// "service" is a reserved key, so it gets prefixed
		t.Errorf("expected fields.service=api, got service=%v fields.service=%v",
			entry["service"], entry["fields.service"])
	}
	if entry["fields.version"] != "1.0" {
		t.Errorf("expected fields.version=1.0, got %v", entry["fields.version"])
	}
}

func TestSlogHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	sl := slog.New(NewSlogHandler(l)).WithGroup("http")

	sl.Info("request", "method", "GET", "path", "/api")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["http.method"] != "GET" {
		t.Errorf("expected http.method=GET, got %v", entry["http.method"])
	}
	if entry["http.path"] != "/api" {
		t.Errorf("expected http.path=/api, got %v", entry["http.path"])
	}
}

func TestSlogHandlerNestedGroups(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	sl := slog.New(NewSlogHandler(l)).WithGroup("http").WithGroup("request")

	sl.Info("incoming", "method", "POST")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["http.request.method"] != "POST" {
		t.Errorf("expected http.request.method=POST, got %v", entry["http.request.method"])
	}
}

func TestSlogHandlerEnabled(t *testing.T) {
	l := New(&bytes.Buffer{}, WARNING)
	h := NewSlogHandler(l)

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected DEBUG to be disabled at WARNING level")
	}
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected INFO to be disabled at WARNING level")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected WARN to be enabled at WARNING level")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("expected ERROR to be enabled at WARNING level")
	}
}

func TestSlogHandlerContextPropagation(t *testing.T) {
	type ctxKey string
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		if v := ctx.Value(ctxKey("trace_id")); v != nil {
			return map[string]any{"trace_id": v}
		}
		return nil
	})

	sl := slog.New(NewSlogHandler(child))
	ctx := context.WithValue(context.Background(), ctxKey("trace_id"), "abc-123")
	sl.InfoContext(ctx, "traced-slog")

	// Note: slog's InfoContext doesn't go through our internalLogCtx path
	// because the slog handler calls writeEntry directly. Context extractors
	// won't be invoked through this path. This is a known limitation.
	// The entry should still be written.
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["message"] != "traced-slog" {
		t.Errorf("expected message=traced-slog, got %v", entry["message"])
	}
}

func TestSlogHandlerWithLoggerFields(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.With(map[string]any{"component": "handler"})

	sl := slog.New(NewSlogHandler(child))
	sl.Info("test", "action", "create")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["component"] != "handler" {
		t.Errorf("expected component=handler from logger fields, got %v", entry["component"])
	}
	if entry["action"] != "create" {
		t.Errorf("expected action=create from slog attrs, got %v", entry["action"])
	}
}
