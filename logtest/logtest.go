// Package logtest provides test helpers for the log-it logger.
//
// TestHandler captures log entries for assertion in tests:
//
//	l, h := logtest.NewTestLogger(t)
//	myService := NewService(l)
//	myService.DoSomething()
//	logtest.AssertLogged(t, h, "INFO", "something happened")
//
// NewTLogger routes log output through t.Log() so entries appear
// only when running tests with -v:
//
//	l := logtest.NewTLogger(t)
package logtest

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	logger "github.com/adamjohannes/log-it"
)

// Record holds a parsed log entry captured by TestHandler.
type Record struct {
	Level   string
	Message string
	Fields  map[string]any
}

// TestHandler is an io.Writer that captures JSON log entries for
// inspection in tests. It is safe for concurrent use.
type TestHandler struct {
	mu      sync.Mutex
	records []Record
}

// Write implements io.Writer. It parses each write as a JSON log entry
// and stores the result as a Record.
func (h *TestHandler) Write(p []byte) (int, error) {
	var raw map[string]any
	if err := json.Unmarshal(p, &raw); err != nil {
		// Store as unparsed record
		h.mu.Lock()
		h.records = append(h.records, Record{
			Message: strings.TrimSpace(string(p)),
			Fields:  raw,
		})
		h.mu.Unlock()
		return len(p), nil
	}

	rec := Record{Fields: raw}
	if v, ok := raw["level"].(string); ok {
		rec.Level = v
	}
	if v, ok := raw["message"].(string); ok {
		rec.Message = v
	}

	h.mu.Lock()
	h.records = append(h.records, rec)
	h.mu.Unlock()
	return len(p), nil
}

// Records returns a copy of all captured records.
func (h *TestHandler) Records() []Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Record, len(h.records))
	copy(out, h.records)
	return out
}

// Len returns the number of captured records.
func (h *TestHandler) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.records)
}

// Reset clears all captured records.
func (h *TestHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = h.records[:0]
}

// NewTestLogger creates a TRACE-level logger that writes to a
// TestHandler. Returns both so callers can log via the logger
// and assert via the handler.
func NewTestLogger(t *testing.T) (*logger.Logger, *TestHandler) {
	t.Helper()
	h := &TestHandler{}
	l := logger.New(h, logger.TRACE)
	return l, h
}

// tWriter adapts testing.T into an io.Writer.
type tWriter struct{ t *testing.T }

func (w tWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// NewTLogger creates a TRACE-level logger that routes all output
// through t.Log(). Log entries appear only when running with -v and
// are associated with the test that created the logger.
func NewTLogger(t *testing.T) *logger.Logger {
	t.Helper()
	return logger.New(tWriter{t}, logger.TRACE)
}

// AssertLogged fails the test if no record matches the given level
// and message substring.
func AssertLogged(t *testing.T, h *TestHandler, level, msgSubstring string) {
	t.Helper()
	for _, r := range h.Records() {
		if r.Level == level && strings.Contains(r.Message, msgSubstring) {
			return
		}
	}
	t.Errorf("expected log at level %s containing %q, got %d records: %v",
		level, msgSubstring, h.Len(), summarize(h.Records()))
}

// AssertNotLogged fails the test if any record matches the given level
// and message substring.
func AssertNotLogged(t *testing.T, h *TestHandler, level, msgSubstring string) {
	t.Helper()
	for _, r := range h.Records() {
		if r.Level == level && strings.Contains(r.Message, msgSubstring) {
			t.Errorf("unexpected log at level %s containing %q: %+v", level, msgSubstring, r)
			return
		}
	}
}

func summarize(records []Record) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Level + ": " + r.Message
	}
	return out
}
