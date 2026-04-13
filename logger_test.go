package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
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
	file, ok := entry["file"].(string)
	if !ok {
		t.Error("expected file to be a string")
	}
	if !strings.Contains(file, "logger_test.go") {
		t.Errorf("expected file to contain logger_test.go, got %v", file)
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

// noopExit is a test exit function that records the code without exiting.
func noopExit(code *int) func(int) {
	return func(c int) { *code = c }
}

// --- TRACE level tests ---

func TestTraceLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, TRACE)
	l.Trace("trace-msg", map[string]any{"k": "v"})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "TRACE" {
		t.Errorf("expected level=TRACE, got %v", entry["level"])
	}
	if entry["k"] != "v" {
		t.Errorf("expected k=v, got %v", entry["k"])
	}
}

func TestTraceFilteredByDebug(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Trace("should-drop", nil)

	if buf.Len() != 0 {
		t.Error("expected TRACE to be dropped when minLevel is DEBUG")
	}
}

func TestTraceLevelString(t *testing.T) {
	if TRACE.String() != "TRACE" {
		t.Errorf("expected TRACE, got %s", TRACE.String())
	}
}

// --- Full caller path tests ---

func TestDefaultCallerIsBasename(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("test", nil)
	})
	file, _ := entry["file"].(string)
	if strings.Contains(file, "/") {
		t.Errorf("expected basename only, got %v", file)
	}
}

func TestFullCallerPathIncludesDirectory(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithFullCallerPath()},
		func(l *Logger) {
			l.Info("test", nil)
		},
	)
	file, _ := entry["file"].(string)
	if !strings.Contains(file, "/") {
		t.Errorf("expected full path with /, got %v", file)
	}
	if !strings.Contains(file, "logger_test.go") {
		t.Errorf("expected logger_test.go in path, got %v", file)
	}
}

// --- Event ID tests ---

func TestEventIDPresent(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithEventID()},
		func(l *Logger) {
			l.Info("with-id", nil)
		},
	)
	eid, ok := entry["event_id"].(string)
	if !ok || eid == "" {
		t.Errorf("expected non-empty event_id, got %v", entry["event_id"])
	}
}

func TestEventIDNotPresentByDefault(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("no-id", nil)
	})
	if _, exists := entry["event_id"]; exists {
		t.Error("expected no event_id without WithEventID")
	}
}

func TestEventIDsAreUnique(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithEventID())
	l.Info("a", nil)
	l.Info("b", nil)

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	id1, _ := entries[0]["event_id"].(string)
	id2, _ := entries[1]["event_id"].(string)
	if id1 == id2 {
		t.Errorf("expected unique event_ids, both are %q", id1)
	}
}

// --- Level filtering & SetLevel/GetLevel tests ---

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, WARNING)

	l.Debug("should-drop", nil)
	l.Info("should-drop", nil)
	if buf.Len() != 0 {
		t.Error("expected no output for levels below WARNING")
	}

	l.Warning("should-write", nil)
	l.Error("should-also-write", nil)

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSetLevelChangesFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, ERROR)

	l.Info("dropped", nil)
	if buf.Len() != 0 {
		t.Error("expected Info to be dropped at ERROR level")
	}

	l.SetLevel(DEBUG)
	l.Info("written", nil)

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["message"] != "written" {
		t.Errorf("expected message=written, got %v", entries[0]["message"])
	}
}

func TestGetLevel(t *testing.T) {
	l := New(&bytes.Buffer{}, WARNING)
	if l.GetLevel() != WARNING {
		t.Errorf("expected WARNING, got %v", l.GetLevel())
	}
}

func TestSetLevelOnChild(t *testing.T) {
	root := New(&bytes.Buffer{}, INFO)
	child := root.With(map[string]any{"c": true})
	child.SetLevel(ERROR)

	if root.GetLevel() != ERROR {
		t.Errorf("expected root level to be ERROR after child.SetLevel, got %v", root.GetLevel())
	}
}

func TestGetLevelOnChild(t *testing.T) {
	root := New(&bytes.Buffer{}, WARNING)
	child := root.With(nil)

	if child.GetLevel() != WARNING {
		t.Errorf("expected child.GetLevel()=WARNING, got %v", child.GetLevel())
	}

	root.SetLevel(DEBUG)
	if child.GetLevel() != DEBUG {
		t.Errorf("expected child.GetLevel()=DEBUG after root change, got %v", child.GetLevel())
	}
}

// --- Structured methods: all levels ---

func TestAllStructuredLevels(t *testing.T) {
	type logFunc func(*Logger, string, map[string]any)
	tests := []struct {
		name  string
		fn    logFunc
		level string
	}{
		{"Debug", func(l *Logger, m string, f map[string]any) { l.Debug(m, f) }, "DEBUG"},
		{"Info", func(l *Logger, m string, f map[string]any) { l.Info(m, f) }, "INFO"},
		{"Warning", func(l *Logger, m string, f map[string]any) { l.Warning(m, f) }, "WARNING"},
		{"Error", func(l *Logger, m string, f map[string]any) { l.Error(m, f) }, "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, DEBUG)
			tt.fn(l, "hello", map[string]any{"k": "v"})

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["level"] != tt.level {
				t.Errorf("expected level=%s, got %v", tt.level, entry["level"])
			}
			if entry["message"] != "hello" {
				t.Errorf("expected message=hello, got %v", entry["message"])
			}
			if entry["k"] != "v" {
				t.Errorf("expected k=v, got %v", entry["k"])
			}
		})
	}
}

func TestFatalCallsExitFunc(t *testing.T) {
	var buf bytes.Buffer
	var exitCode int
	l := New(&buf, DEBUG, withExitFunc(noopExit(&exitCode)))

	l.Fatal("boom", map[string]any{"reason": "test"})

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

func TestFatalWritesBeforeExiting(t *testing.T) {
	var buf bytes.Buffer
	var written bool
	l := New(&buf, DEBUG, withExitFunc(func(int) {
		written = buf.Len() > 0
	}))

	l.Fatal("last-words", nil)

	if !written {
		t.Error("expected log entry to be written before exitFunc was called")
	}
}

func TestFatalSyncsAsyncWriter(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 256)
	var exitCode int
	l := New(aw, DEBUG, withExitFunc(noopExit(&exitCode)))

	// Write some entries through async path
	for i := 0; i < 5; i++ {
		l.Info("before-fatal", map[string]any{"i": i})
	}
	l.Fatal("the end", nil)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	// After Fatal + sync, all entries should be in the buffer
	entries := decodeAllEntries(t, &buf)
	if len(entries) != 6 {
		t.Errorf("expected 6 entries (5 info + 1 fatal), got %d", len(entries))
	}

	// Last entry should be FATAL
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		if last["level"] != "FATAL" {
			t.Errorf("expected last entry level=FATAL, got %v", last["level"])
		}
	}
}

// --- Formatted methods ---

func TestAllFormattedLevels(t *testing.T) {
	type logFunc func(*Logger, string, ...any)
	tests := []struct {
		name  string
		fn    logFunc
		level string
	}{
		{"Debugf", func(l *Logger, f string, v ...any) { l.Debugf(f, v...) }, "DEBUG"},
		{"Infof", func(l *Logger, f string, v ...any) { l.Infof(f, v...) }, "INFO"},
		{"Warningf", func(l *Logger, f string, v ...any) { l.Warningf(f, v...) }, "WARNING"},
		{"Errorf", func(l *Logger, f string, v ...any) { l.Errorf(f, v...) }, "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, DEBUG)
			tt.fn(l, "count=%d name=%s", 42, "alice")

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["level"] != tt.level {
				t.Errorf("expected level=%s, got %v", tt.level, entry["level"])
			}
			if entry["message"] != "count=42 name=alice" {
				t.Errorf("expected interpolated message, got %v", entry["message"])
			}
		})
	}
}

func TestFatalfCallsExitFunc(t *testing.T) {
	var buf bytes.Buffer
	var exitCode int
	l := New(&buf, DEBUG, withExitFunc(noopExit(&exitCode)))

	l.Fatalf("fatal: %s", "reason")

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["message"] != "fatal: reason" {
		t.Errorf("expected interpolated message, got %v", entry["message"])
	}
}

func TestFormattedMethodsHaveNoFields(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Infof("hello %s", "world")
	})

	// Should have exactly 4 core keys, no user fields
	if len(entry) != 4 {
		t.Errorf("expected 4 keys, got %d: %v", len(entry), entry)
	}
}

// --- Context methods ---

func TestAllContextLevels(t *testing.T) {
	type ctxKey string
	type logFunc func(*Logger, context.Context, string, map[string]any)
	tests := []struct {
		name  string
		fn    logFunc
		level string
	}{
		{"DebugContext", func(l *Logger, ctx context.Context, m string, f map[string]any) { l.DebugContext(ctx, m, f) }, "DEBUG"},
		{"InfoContext", func(l *Logger, ctx context.Context, m string, f map[string]any) { l.InfoContext(ctx, m, f) }, "INFO"},
		{"WarningContext", func(l *Logger, ctx context.Context, m string, f map[string]any) { l.WarningContext(ctx, m, f) }, "WARNING"},
		{"ErrorContext", func(l *Logger, ctx context.Context, m string, f map[string]any) { l.ErrorContext(ctx, m, f) }, "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, DEBUG)
			child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
				if v := ctx.Value(ctxKey("rid")); v != nil {
					return map[string]any{"request_id": v}
				}
				return nil
			})

			ctx := context.WithValue(context.Background(), ctxKey("rid"), "req-1")
			tt.fn(child, ctx, "ctx-log", nil)

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["level"] != tt.level {
				t.Errorf("expected level=%s, got %v", tt.level, entry["level"])
			}
			if entry["request_id"] != "req-1" {
				t.Errorf("expected request_id=req-1, got %v", entry["request_id"])
			}
		})
	}
}

func TestFatalContextCallsExitFunc(t *testing.T) {
	var buf bytes.Buffer
	var exitCode int
	l := New(&buf, DEBUG, withExitFunc(noopExit(&exitCode)))

	l.FatalContext(context.Background(), "fatal-ctx", map[string]any{"x": 1})

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
}

func TestContextMethodsWithNilContext(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"should_not": "appear"}
	})

	// nil context — extractors should be skipped, no panic
	child.InfoContext(nil, "nil-ctx", nil) //nolint:staticcheck // deliberately testing nil context behavior

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["message"] != "nil-ctx" {
		t.Errorf("expected message=nil-ctx, got %v", entry["message"])
	}
	if _, exists := entry["should_not"]; exists {
		t.Error("expected extractor to be skipped with nil context")
	}
}

func TestContextMethodsWithNilExtractorResult(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		return nil // extractor returns nil
	})

	child.InfoContext(context.Background(), "nil-result", map[string]any{"a": 1})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["a"] != float64(1) {
		t.Errorf("expected a=1, got %v", entry["a"])
	}
}

// --- Field priority & merge precedence ---

func TestFieldPriority_PerCallOverridesContext(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.With(map[string]any{"x": "persistent"})
	child = child.WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"x": "context"}
	})

	child.InfoContext(context.Background(), "priority", map[string]any{"x": "per-call"})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["x"] != "per-call" {
		t.Errorf("expected x=per-call (highest priority), got %v", entry["x"])
	}
}

func TestFieldPriority_ContextOverridesPersistent(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.With(map[string]any{"x": "persistent"})
	child = child.WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"x": "context"}
	})

	child.InfoContext(context.Background(), "priority", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["x"] != "context" {
		t.Errorf("expected x=context (overrides persistent), got %v", entry["x"])
	}
}

func TestFieldPriority_PersistentIsDefault(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		child := l.With(map[string]any{"x": "persistent"})
		child.Info("priority", nil)
	})

	if entry["x"] != "persistent" {
		t.Errorf("expected x=persistent, got %v", entry["x"])
	}
}

func TestMultipleExtractors(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"a": "from-ext1"}
	}).WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"b": "from-ext2"}
	})

	child.InfoContext(context.Background(), "multi", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["a"] != "from-ext1" {
		t.Errorf("expected a=from-ext1, got %v", entry["a"])
	}
	if entry["b"] != "from-ext2" {
		t.Errorf("expected b=from-ext2, got %v", entry["b"])
	}
}

func TestMultipleExtractorsCollision(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"x": "first"}
	}).WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"x": "second"}
	})

	child.InfoContext(context.Background(), "collision", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	// Second extractor merges over first
	if entry["x"] != "second" {
		t.Errorf("expected x=second (last extractor wins), got %v", entry["x"])
	}
}

// --- Edge cases ---

func TestJsonMarshalFailure(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	// func() is not JSON-marshalable
	l.Info("bad-field", map[string]any{"fn": func() {}})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "ERROR" {
		t.Errorf("expected fallback level=ERROR, got %v", entry["level"])
	}
	msg, _ := entry["message"].(string)
	if !strings.Contains(msg, "failed to marshal log entry to json") {
		t.Errorf("expected English fallback message, got %v", msg)
	}
}

func TestJsonMarshalFallbackNoInjection(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	// func() triggers marshal failure; the error message from json.Marshal
	// may contain quotes. Verify the fallback is still valid JSON.
	l.Info("inject-test", map[string]any{"fn": func() {}})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("fallback JSON is invalid (possible injection): %v\nraw: %s", err, buf.String())
	}
}

func TestLevelStringUnknown(t *testing.T) {
	if Level(99).String() != "UNKNOWN" {
		t.Errorf("expected UNKNOWN for invalid level, got %s", Level(99).String())
	}
}

func TestTimestampFormat(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("ts-test", nil)
	})

	ts, ok := entry["time"].(string)
	if !ok {
		t.Fatal("time is not a string")
	}
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("time %q is not valid RFC3339Nano: %v", ts, err)
	}
}

func TestEmptyMessage(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		l.Info("", nil)
	})

	if entry["message"] != "" {
		t.Errorf("expected empty message, got %v", entry["message"])
	}
}

func TestWriteAfterSyncContextMethod(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.InfoContext(context.Background(), "before", nil)

	_ = l.Sync()
	before := buf.Len()

	l.InfoContext(context.Background(), "after", nil)
	if buf.Len() != before {
		t.Error("expected context log after Sync to be dropped")
	}
}

// --- Concurrency tests ---

func TestConcurrentSetLevelAndLogging(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	var wg sync.WaitGroup

	// 10 goroutines logging
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				l.Info("concurrent", map[string]any{"j": j})
			}
		}()
	}

	// 1 goroutine flipping level
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			if j%2 == 0 {
				l.SetLevel(ERROR)
			} else {
				l.SetLevel(DEBUG)
			}
		}
	}()

	wg.Wait()
	// If we got here without -race failing, the test passes.
}

func TestConcurrentSyncAndLogging(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	var wg sync.WaitGroup

	// Goroutines logging
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				l.Info("log", nil)
			}
		}()
	}

	// Another goroutine calling Sync
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = l.Sync()
	}()

	wg.Wait()
	// No panics or races = pass.
}

// --- Child logger isolation ---

func TestChildSiblingIsolation(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG)
	child1 := root.With(map[string]any{"a": 1})
	child2 := root.With(map[string]any{"b": 2})

	child1.Info("c1", nil)
	child2.Info("c2", nil)

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// child1's entry should have "a" but not "b"
	e1 := entries[0]
	if e1["a"] != float64(1) {
		t.Errorf("child1: expected a=1, got %v", e1["a"])
	}
	if _, exists := e1["b"]; exists {
		t.Error("child1: should not have field 'b' from sibling")
	}

	// child2's entry should have "b" but not "a"
	e2 := entries[1]
	if e2["b"] != float64(2) {
		t.Errorf("child2: expected b=2, got %v", e2["b"])
	}
	if _, exists := e2["a"]; exists {
		t.Error("child2: should not have field 'a' from sibling")
	}
}

func TestChainedWithPrecedence(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		child := l.With(map[string]any{"x": 1}).With(map[string]any{"x": 2})
		child.Info("chained", nil)
	})

	if entry["x"] != float64(2) {
		t.Errorf("expected x=2 (second With wins), got %v", entry["x"])
	}
}

func TestWithNilFields(t *testing.T) {
	entry := captureLog(func(l *Logger) {
		child := l.With(nil)
		child.Info("nil-fields", nil)
	})

	if entry["message"] != "nil-fields" {
		t.Errorf("expected message=nil-fields, got %v", entry["message"])
	}
}

// --- Extractor panic recovery ---

func TestExtractorPanicDoesNotCrash(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		panic("extractor boom")
	}).WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"safe": true}
	})

	child.InfoContext(context.Background(), "survived", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["message"] != "survived" {
		t.Errorf("expected message=survived, got %v", entry["message"])
	}
	if entry["safe"] != true {
		t.Errorf("expected safe=true from second extractor, got %v", entry["safe"])
	}
}

// --- TRACE method variants ---

func TestTracef(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, TRACE)
	l.Tracef("trace %d", 42)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "TRACE" {
		t.Errorf("expected level=TRACE, got %v", entry["level"])
	}
	if entry["message"] != "trace 42" {
		t.Errorf("expected interpolated message, got %v", entry["message"])
	}
}

func TestTracew(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, TRACE)
	l.Tracew("typed-trace", String("k", "v"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "TRACE" {
		t.Errorf("expected level=TRACE, got %v", entry["level"])
	}
	if entry["k"] != "v" {
		t.Errorf("expected k=v, got %v", entry["k"])
	}
}

func TestTraceContext(t *testing.T) {
	type ctxKey string
	var buf bytes.Buffer
	l := New(&buf, TRACE)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		if v := ctx.Value(ctxKey("tid")); v != nil {
			return map[string]any{"trace_id": v}
		}
		return nil
	})

	ctx := context.WithValue(context.Background(), ctxKey("tid"), "t-1")
	child.TraceContext(ctx, "ctx-trace", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "TRACE" {
		t.Errorf("expected level=TRACE, got %v", entry["level"])
	}
	if entry["trace_id"] != "t-1" {
		t.Errorf("expected trace_id=t-1, got %v", entry["trace_id"])
	}
}

// --- Hooks with formatted/typed/context methods ---

func TestHooksCalledWithFormattedMethods(t *testing.T) {
	var called bool
	hook := func(level Level, msg string, _ map[string]any) {
		called = true
		if level != INFO || msg != "hello world" {
			panic("unexpected hook args")
		}
	}

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(hook))
	l.Infof("hello %s", "world")

	if !called {
		t.Error("expected hook to be called with Infof")
	}
}

func TestHooksCalledWithTypedMethods(t *testing.T) {
	var gotFields map[string]any
	hook := func(_ Level, _ string, fields map[string]any) {
		gotFields = fields
	}

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(hook))
	l.Infow("typed", String("k", "v"))

	if gotFields["k"] != "v" {
		t.Errorf("expected hook to receive typed field k=v, got %v", gotFields["k"])
	}
}

func TestHooksCalledWithContextMethods(t *testing.T) {
	var called bool
	hook := func(_ Level, _ string, _ map[string]any) { called = true }

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(hook))
	l.InfoContext(context.Background(), "ctx", nil)

	if !called {
		t.Error("expected hook to be called with InfoContext")
	}
}

// --- Sampler with formatted/typed methods ---

func TestSamplerWithFormattedMethods(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithSampler(NewEveryNSampler(5)))

	for i := 0; i < 10; i++ {
		l.Infof("msg %d", i)
	}

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 2 {
		t.Errorf("expected 2 sampled entries from Infof, got %d", len(entries))
	}
}

func TestSamplerWithTypedMethods(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithSampler(NewEveryNSampler(5)))

	for i := 0; i < 10; i++ {
		l.Infow("msg", Int("i", i))
	}

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 2 {
		t.Errorf("expected 2 sampled entries from Infow, got %d", len(entries))
	}
}

// --- Error enrichment through *w methods ---

func TestErrorEnrichmentWithTypedFields(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	err := errors.New("connection refused")
	l.Errorw("failed", Err(err))

	var entry map[string]any
	if e := json.Unmarshal(buf.Bytes(), &entry); e != nil {
		t.Fatal(e)
	}
	if entry["error"] != "connection refused" {
		t.Errorf("expected error string, got %v", entry["error"])
	}
	if entry["error_type"] != "*errors.errorString" {
		t.Errorf("expected error_type, got %v", entry["error_type"])
	}
}

// --- Slog handler + hooks ---

func TestSlogHandlerTriggersHooks(t *testing.T) {
	var hookCalled bool
	var hookLevel Level
	hook := func(level Level, _ string, _ map[string]any) {
		hookCalled = true
		hookLevel = level
	}

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(hook))
	sl := slog.New(NewSlogHandler(l))
	sl.Warn("slog-warn")

	if !hookCalled {
		t.Error("expected hook to be called from slog entry")
	}
	if hookLevel != WARNING {
		t.Errorf("expected hook level=WARNING, got %v", hookLevel)
	}
}

// --- Slog handler after close ---

func TestSlogHandlerAfterClose(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	sl := slog.New(NewSlogHandler(l))

	sl.Info("before")
	_ = l.Sync()
	before := buf.Len()

	sl.Info("after")
	if buf.Len() != before {
		t.Error("expected slog entry after Sync to be dropped")
	}
}

// --- Timing after Sync ---

func TestTimedAfterSync(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)

	done := l.Timed("op")
	_ = l.Sync()
	before := buf.Len()

	done() // should be silently dropped
	if buf.Len() != before {
		t.Error("expected Timed done() after Sync to be dropped")
	}
}

// --- Full caller path with child logger ---

func TestFullCallerPathWithChild(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG, WithFullCallerPath())
	child := root.With(map[string]any{"component": "handler"})
	child.Info("child-full-path", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	file, _ := entry["file"].(string)
	if !strings.Contains(file, "/") {
		t.Errorf("expected full path on child, got %v", file)
	}
}

// --- TextFormatter with error-enriched fields ---

func TestTextFormatterWithErrorEnrichment(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))

	err := errors.New("timeout")
	l.Error("failed", map[string]any{"err": err})

	line := buf.String()
	if !strings.Contains(line, "err=timeout") {
		t.Errorf("expected err=timeout in text output: %s", line)
	}
	if !strings.Contains(line, "err_type=") {
		t.Errorf("expected err_type in text output: %s", line)
	}
}

// --- Event ID format and concurrent uniqueness ---

func TestEventIDFormat(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithEventID()},
		func(l *Logger) {
			l.Info("id-format", nil)
		},
	)
	eid, _ := entry["event_id"].(string)
	if !strings.Contains(eid, "-") {
		t.Errorf("expected hex-hex format, got %q", eid)
	}
}

func TestEventIDConcurrentUniqueness(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 8192)
	l := New(aw, DEBUG, WithEventID())

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				l.Info("concurrent", nil)
			}
		}()
	}
	wg.Wait()
	_ = l.Sync()

	entries := decodeAllEntries(t, &buf)
	ids := make(map[string]bool, len(entries))
	for _, e := range entries {
		eid, _ := e["event_id"].(string)
		if eid == "" {
			t.Fatal("missing event_id in concurrent entry")
		}
		if ids[eid] {
			t.Errorf("duplicate event_id: %s", eid)
		}
		ids[eid] = true
	}
}

// --- Formatter inherited by child ---

func TestFormatterInheritedByChild(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	child := root.With(map[string]any{"child": true})
	child.Info("from-child", nil)

	// Should be text, not JSON
	if buf.Bytes()[0] == '{' {
		t.Error("expected text format from child, got JSON")
	}
	if !strings.Contains(buf.String(), "from-child") {
		t.Error("expected message in text output")
	}
}

// --- Max feature composition ---

func TestMaxFeatureComposition(t *testing.T) {
	var buf bytes.Buffer
	var hookLevel Level
	var hookCalled bool
	hook := func(level Level, _ string, _ map[string]any) {
		hookCalled = true
		hookLevel = level
	}

	aw := NewAsyncWriter(&buf, 4096)
	l := New(aw, TRACE,
		WithServiceIdentity("myapp", "3.0.0", "prod"),
		WithEventID(),
		WithFullCallerPath(),
		WithHooks(hook),
		WithSampler(NewEveryNSampler(1)), // pass all
	)

	child := l.With(map[string]any{"component": "api"})
	child = child.WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"request_id": "req-42"}
	})

	child.ErrorContext(context.Background(), "all-features", map[string]any{
		"err":    errors.New("db timeout"),
		"status": 500,
	})

	_ = l.Sync()

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]

	// Core keys
	if e["level"] != "ERROR" {
		t.Errorf("level: got %v", e["level"])
	}
	if e["message"] != "all-features" {
		t.Errorf("message: got %v", e["message"])
	}

	// Identity
	if e["fields.service"] != "myapp" {
		// "service" is reserved, user field "service" would be prefixed,
		// but identity sets it directly
		if e["service"] != "myapp" {
			t.Errorf("service: got %v", e["service"])
		}
	}

	// Event ID
	eid, _ := e["event_id"].(string)
	if eid == "" {
		t.Error("expected event_id")
	}

	// Full caller path
	file, _ := e["file"].(string)
	if !strings.Contains(file, "/") {
		t.Errorf("expected full caller path, got %v", file)
	}

	// Context extractor
	if e["request_id"] != "req-42" {
		t.Errorf("request_id: got %v", e["request_id"])
	}

	// Child logger fields
	if e["component"] != "api" {
		t.Errorf("component: got %v", e["component"])
	}

	// Error enrichment
	if e["err"] != "db timeout" {
		t.Errorf("err: got %v", e["err"])
	}
	if _, exists := e["err_type"]; !exists {
		t.Error("expected err_type from error enrichment")
	}

	// Per-call field
	if e["status"] != float64(500) {
		t.Errorf("status: got %v", e["status"])
	}

	// Hook was called
	if !hookCalled {
		t.Error("expected hook to be called")
	}
	if hookLevel != ERROR {
		t.Errorf("hook level: got %v", hookLevel)
	}
}

// --- Write error counter ---

type testErrWriter struct{}

func (testErrWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestWriteErrorCounter(t *testing.T) {
	l := New(testErrWriter{}, DEBUG)
	l.Info("a", nil)
	l.Info("b", nil)
	l.Info("c", nil)

	if l.WriteErrorCount() != 3 {
		t.Errorf("expected 3 write errors, got %d", l.WriteErrorCount())
	}
}

func TestWriteErrorCounterZeroOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Info("ok", nil)

	if l.WriteErrorCount() != 0 {
		t.Errorf("expected 0 write errors, got %d", l.WriteErrorCount())
	}
}

// --- SyncWithTimeout ---

func TestSyncWithTimeoutReturnsImmediately(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Info("test", nil)

	err := l.SyncWithTimeout(time.Second)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSyncWithTimeoutExpires(t *testing.T) {
	// Use a writer that blocks forever
	blockCh := make(chan struct{})
	bw := &slowSyncWriter{blockCh: blockCh}
	aw := NewAsyncWriter(bw, 16)
	l := New(aw, DEBUG)

	l.Info("test", nil)

	err := l.SyncWithTimeout(50 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message, got %v", err)
	}

	// Clean up: unblock to prevent goroutine leak
	close(blockCh)
}

// slowSyncWriter implements Syncer but blocks on Sync until unblocked.
type slowSyncWriter struct {
	blockCh chan struct{}
}

func (w *slowSyncWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *slowSyncWriter) Sync() error {
	<-w.blockCh
	return nil
}
