package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// captureLog runs fn with a DEBUG-level logger writing to a buffer,
// then returns the first log entry as a map.
func captureLog(fn func(l *Logger)) map[string]any {
	return captureLogWithOpts(nil, fn)
}

// captureLogWithOpts is like captureLog but accepts logger options.
func captureLogWithOpts(opts []Option, fn func(l *Logger)) map[string]any {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, opts...)
	fn(l)
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		panic("captureLog: invalid JSON: " + err.Error())
	}
	return entry
}

// captureLogAll runs fn and returns all log entries.
func captureLogAll(fn func(l *Logger)) []map[string]any {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	fn(l)
	var entries []map[string]any
	dec := json.NewDecoder(&buf)
	for dec.More() {
		var entry map[string]any
		if err := dec.Decode(&entry); err != nil {
			panic("captureLogAll: invalid JSON: " + err.Error())
		}
		entries = append(entries, entry)
	}
	return entries
}

// --- Flatten tests ---

func TestFlattenFieldsAtTopLevel(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("hello", map[string]any{"user_id": 42, "status": "ok"})
	})

	// User fields should be at top level
	if entry["user_id"] != float64(42) {
		t.Errorf("expected user_id=42, got %v", entry["user_id"])
	}
	if entry["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", entry["status"])
	}

	// There should be no nested "fields" key
	if _, exists := entry["fields"]; exists {
		t.Error("expected no nested 'fields' key, but it exists")
	}

	// Core keys should be present
	for _, key := range []string{"time", "level", "message", "file"} {
		if _, exists := entry[key]; !exists {
			t.Errorf("expected core key %q to be present", key)
		}
	}
}

func TestFlattenCollisionWithReservedKey(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("test", map[string]any{"level": "user_value", "message": "user_msg"})
	})

	// Core keys must keep their logger-set values
	if entry["level"] != "INFO" {
		t.Errorf("expected level=INFO, got %v", entry["level"])
	}
	if entry["message"] != "test" {
		t.Errorf("expected message=test, got %v", entry["message"])
	}

	// Colliding user fields should be prefixed with "fields."
	if entry["fields.level"] != "user_value" {
		t.Errorf("expected fields.level=user_value, got %v", entry["fields.level"])
	}
	if entry["fields.message"] != "user_msg" {
		t.Errorf("expected fields.message=user_msg, got %v", entry["fields.message"])
	}
}

func TestFlattenNoFields(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("bare", nil)
	})

	// Should have exactly 4 core keys
	if len(entry) != 4 {
		t.Errorf("expected 4 keys, got %d: %v", len(entry), entry)
	}
}

func TestFlattenEmptyFields(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("bare", map[string]any{})
	})

	if len(entry) != 4 {
		t.Errorf("expected 4 keys, got %d: %v", len(entry), entry)
	}
}

func TestFlattenWithChildLogger(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG)
	child := root.With(map[string]any{"component": "api"})
	child.Info("request", map[string]any{"path": "/health"})

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

func TestFlattenWithContextExtractor(t *testing.T) {
	type ctxKey string
	var buf bytes.Buffer
	root := New(&buf, DEBUG)
	child := root.WithContextExtractor(func(ctx context.Context) map[string]any {
		if v := ctx.Value(ctxKey("trace_id")); v != nil {
			return map[string]any{"trace_id": v}
		}
		return nil
	})

	ctx := context.WithValue(context.Background(), ctxKey("trace_id"), "abc-123")
	child.InfoContext(ctx, "traced", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}

	if entry["trace_id"] != "abc-123" {
		t.Errorf("expected trace_id=abc-123, got %v", entry["trace_id"])
	}
}

func TestCoreKeysPresent(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Debug("dbg", nil)
	})

	if entry["level"] != "DEBUG" {
		t.Errorf("expected level=DEBUG, got %v", entry["level"])
	}
	if entry["message"] != "dbg" {
		t.Errorf("expected message=dbg, got %v", entry["message"])
	}
	if _, ok := entry["time"].(string); !ok {
		t.Error("expected time to be a string")
	}
	if _, ok := entry["file"].(string); !ok {
		t.Error("expected file to be a string")
	}
}

// --- Service identity tests ---

func TestIdentityKeysPresent(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithServiceIdentity("api", "1.2.3", "prod")},
		func(l *Logger) {
			l.Info("boot", nil)
		},
	)

	if entry["service"] != "api" {
		t.Errorf("expected service=api, got %v", entry["service"])
	}
	if entry["version"] != "1.2.3" {
		t.Errorf("expected version=1.2.3, got %v", entry["version"])
	}
	if entry["env"] != "prod" {
		t.Errorf("expected env=prod, got %v", entry["env"])
	}
	if h, ok := entry["host"].(string); !ok || h == "" {
		t.Errorf("expected host to be a non-empty string, got %v", entry["host"])
	}
}

func TestIdentityNotPresentWithoutOption(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("no-id", nil)
	})

	for _, key := range []string{"service", "version", "env", "host"} {
		if _, exists := entry[key]; exists {
			t.Errorf("expected no %q key without identity option, but it exists", key)
		}
	}
}

func TestIdentityInheritedByChild(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG, WithServiceIdentity("svc", "0.1.0", "staging"))
	child := root.With(map[string]any{"component": "handler"})
	child.Info("child-log", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}

	if entry["service"] != "svc" {
		t.Errorf("expected service=svc on child, got %v", entry["service"])
	}
	if entry["component"] != "handler" {
		t.Errorf("expected component=handler, got %v", entry["component"])
	}
}

func TestIdentityCollisionWithUserField(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithServiceIdentity("api", "1.0.0", "prod")},
		func(l *Logger) {
			l.Info("test", map[string]any{"service": "user-val"})
		},
	)

	// Core identity key should be preserved
	if entry["service"] != "api" {
		t.Errorf("expected service=api, got %v", entry["service"])
	}
	// User's colliding field should be namespaced
	if entry["fields.service"] != "user-val" {
		t.Errorf("expected fields.service=user-val, got %v", entry["fields.service"])
	}
}

// --- Sync tests ---

func TestSyncFlushesAsyncWriter(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 256)
	l := New(aw, DEBUG)

	for i := 0; i < 10; i++ {
		l.Info("msg", map[string]any{"i": i})
	}

	if err := l.Sync(); err != nil {
		t.Fatalf("sync error: %v", err)
	}

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 10 {
		t.Errorf("expected 10 entries, got %d", len(entries))
	}
}

func TestSyncOnPlainWriter(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Info("hello", nil)

	if err := l.Sync(); err != nil {
		t.Fatalf("sync on plain writer should not error: %v", err)
	}
}

func TestSyncIdempotent(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 16)
	l := New(aw, DEBUG)
	l.Info("once", nil)

	if err := l.Sync(); err != nil {
		t.Fatalf("first sync error: %v", err)
	}
	if err := l.Sync(); err != nil {
		t.Fatalf("second sync error: %v", err)
	}
}

func TestWriteAfterSyncIsDropped(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Info("before", nil)

	_ = l.Sync()

	before := buf.Len()
	l.Info("after", nil)

	if buf.Len() != before {
		t.Error("expected no new data after Sync, but buffer grew")
	}
}

func TestSyncOnChildSyncsRoot(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 256)
	root := New(aw, DEBUG)
	child := root.With(map[string]any{"child": true})

	child.Info("from-child", nil)

	if err := child.Sync(); err != nil {
		t.Fatalf("sync from child error: %v", err)
	}

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// --- Full integration test ---

func TestFullComposition(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	fan := NewFanOutWriter(&buf1, &buf2)
	aw := NewAsyncWriter(fan, 4096)
	l := New(aw, INFO, WithServiceIdentity("myapp", "2.0.0", "prod"))

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				l.Info("request", map[string]any{"goroutine": id, "i": i})
			}
		}(g)
	}
	wg.Wait()

	if err := l.Sync(); err != nil {
		t.Fatalf("sync error: %v", err)
	}

	entries1 := decodeAllEntries(t, &buf1)
	entries2 := decodeAllEntries(t, &buf2)

	if len(entries1) != 50 {
		t.Errorf("buf1: expected 50 entries, got %d (dropped=%d)", len(entries1), aw.DroppedCount())
	}
	if len(entries2) != 50 {
		t.Errorf("buf2: expected 50 entries, got %d", len(entries2))
	}

	// Verify identity on first entry
	if len(entries1) > 0 {
		e := entries1[0]
		if e["service"] != "myapp" {
			t.Errorf("expected service=myapp, got %v", e["service"])
		}
		if e["version"] != "2.0.0" {
			t.Errorf("expected version=2.0.0, got %v", e["version"])
		}
		if e["env"] != "prod" {
			t.Errorf("expected env=prod, got %v", e["env"])
		}
	}
}

// decodeAllEntries reads all newline-delimited JSON entries from buf.
func decodeAllEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	for dec.More() {
		var entry map[string]any
		if err := dec.Decode(&entry); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}
