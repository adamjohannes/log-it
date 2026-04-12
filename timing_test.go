package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestTimedLogsDuration(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	done := l.Timed("db_query")
	time.Sleep(5 * time.Millisecond)
	done()

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["message"] != "db_query" {
		t.Errorf("expected message=db_query, got %v", entry["message"])
	}
	dur, ok := entry["duration_ms"].(float64)
	if !ok {
		t.Fatalf("expected duration_ms as float64, got %T", entry["duration_ms"])
	}
	if dur <= 0 {
		t.Errorf("expected positive duration, got %v", dur)
	}
}

func TestTimedContextLogsDuration(t *testing.T) {
	type ctxKey string
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		if v := ctx.Value(ctxKey("rid")); v != nil {
			return map[string]any{"request_id": v}
		}
		return nil
	})

	ctx := context.WithValue(context.Background(), ctxKey("rid"), "req-1")
	done := child.TimedContext(ctx, "http_request")
	time.Sleep(2 * time.Millisecond)
	done()

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["message"] != "http_request" {
		t.Errorf("expected message=http_request, got %v", entry["message"])
	}
	if entry["request_id"] != "req-1" {
		t.Errorf("expected request_id=req-1, got %v", entry["request_id"])
	}
	if _, ok := entry["duration_ms"].(float64); !ok {
		t.Error("expected duration_ms field")
	}
}

func TestTimedWithChildLogger(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG)
	child := root.With(map[string]any{"component": "db"})

	done := child.Timed("query")
	done()

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["component"] != "db" {
		t.Errorf("expected component=db, got %v", entry["component"])
	}
	if entry["message"] != "query" {
		t.Errorf("expected message=query, got %v", entry["message"])
	}
}
