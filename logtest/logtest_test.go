package logtest

import (
	"sync"
	"testing"

	logger "github.com/adamjohannes/log-it"
)

func TestNewTestLoggerCapturesEntries(t *testing.T) {
	l, h := NewTestLogger(t)
	l.Info("hello", map[string]any{"key": "val"})
	l.Error("boom", nil)

	if h.Len() != 2 {
		t.Fatalf("expected 2 records, got %d", h.Len())
	}
	records := h.Records()
	if records[0].Level != "INFO" || records[0].Message != "hello" {
		t.Errorf("record 0: got %+v", records[0])
	}
	if records[1].Level != "ERROR" || records[1].Message != "boom" {
		t.Errorf("record 1: got %+v", records[1])
	}
}

func TestTestHandlerFieldsAvailable(t *testing.T) {
	l, h := NewTestLogger(t)
	l.Info("req", map[string]any{"status": float64(200)})

	records := h.Records()
	if len(records) != 1 {
		t.Fatal("expected 1 record")
	}
	if records[0].Fields["status"] != float64(200) {
		t.Errorf("expected status=200, got %v", records[0].Fields["status"])
	}
}

func TestTestHandlerReset(t *testing.T) {
	l, h := NewTestLogger(t)
	l.Info("a", nil)
	l.Info("b", nil)
	if h.Len() != 2 {
		t.Fatal("expected 2 before reset")
	}
	h.Reset()
	if h.Len() != 0 {
		t.Errorf("expected 0 after reset, got %d", h.Len())
	}
}

func TestTestHandlerConcurrency(t *testing.T) {
	l, h := NewTestLogger(t)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Info("concurrent", nil)
		}()
	}
	wg.Wait()
	if h.Len() != 100 {
		t.Errorf("expected 100 records, got %d", h.Len())
	}
}

func TestNewTLoggerDoesNotPanic(t *testing.T) {
	l := NewTLogger(t)
	l.Info("via t.Log", nil)
	l.Debug("debug via t.Log", nil)
	// If we got here, it works. Output appears with -v.
}

func TestAssertLogged(t *testing.T) {
	l, h := NewTestLogger(t)
	l.Info("request completed", nil)
	l.Error("db timeout", nil)

	AssertLogged(t, h, "INFO", "request completed")
	AssertLogged(t, h, "ERROR", "timeout")
	AssertLogged(t, h, "INFO", "request") // substring match
}

func TestAssertNotLogged(t *testing.T) {
	l, h := NewTestLogger(t)
	l.Info("ok", nil)

	AssertNotLogged(t, h, "ERROR", "fail")
	AssertNotLogged(t, h, "INFO", "missing")
}

func TestRecordsReturnsACopy(t *testing.T) {
	l, h := NewTestLogger(t)
	l.Info("first", nil)

	records := h.Records()
	l.Info("second", nil)

	// Original slice should not have been modified
	if len(records) != 1 {
		t.Errorf("expected copy to have 1 record, got %d", len(records))
	}
	if h.Len() != 2 {
		t.Errorf("expected handler to have 2 records, got %d", h.Len())
	}
}

func TestNewTestLoggerAtTraceLevel(t *testing.T) {
	l, h := NewTestLogger(t)
	l.SetLevel(logger.TRACE)
	l.Trace("trace-msg", nil)
	l.Debug("debug-msg", nil)

	AssertLogged(t, h, "TRACE", "trace-msg")
	AssertLogged(t, h, "DEBUG", "debug-msg")
}
