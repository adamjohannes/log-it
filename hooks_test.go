package logger

import (
	"bytes"
	"testing"
)

func TestHookCalledWithCorrectArgs(t *testing.T) {
	var gotLevel Level
	var gotMsg string
	var gotFields map[string]any

	hook := func(level Level, message string, fields map[string]any) {
		gotLevel = level
		gotMsg = message
		gotFields = fields
	}

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(hook))
	l.Info("hello", map[string]any{"k": "v"})

	if gotLevel != INFO {
		t.Errorf("expected level=INFO, got %v", gotLevel)
	}
	if gotMsg != "hello" {
		t.Errorf("expected message=hello, got %v", gotMsg)
	}
	if gotFields["k"] != "v" {
		t.Errorf("expected fields[k]=v, got %v", gotFields["k"])
	}
}

func TestMultipleHooksCalledInOrder(t *testing.T) {
	var order []int

	h1 := func(Level, string, map[string]any) { order = append(order, 1) }
	h2 := func(Level, string, map[string]any) { order = append(order, 2) }
	h3 := func(Level, string, map[string]any) { order = append(order, 3) }

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(h1, h2, h3))
	l.Info("test", nil)

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("expected hooks called in order [1,2,3], got %v", order)
	}
}

func TestHookFilterByLevel(t *testing.T) {
	var errorCount int

	hook := func(level Level, _ string, _ map[string]any) {
		if level >= ERROR {
			errorCount++
		}
	}

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(hook))
	l.Debug("d", nil)
	l.Info("i", nil)
	l.Warning("w", nil)
	l.Error("e", nil)

	if errorCount != 1 {
		t.Errorf("expected 1 error-level hook call, got %d", errorCount)
	}
}

func TestHookInheritedByChild(t *testing.T) {
	var called bool
	hook := func(Level, string, map[string]any) { called = true }

	var buf bytes.Buffer
	root := New(&buf, DEBUG, WithHooks(hook))
	child := root.With(map[string]any{"child": true})
	child.Info("from-child", nil)

	if !called {
		t.Error("expected hook to be called from child logger")
	}
}

func TestHookPanicDoesNotCrash(t *testing.T) {
	var secondCalled bool

	panicHook := func(Level, string, map[string]any) { panic("boom") }
	safeHook := func(Level, string, map[string]any) { secondCalled = true }

	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithHooks(panicHook, safeHook))
	l.Info("test", nil)

	if !secondCalled {
		t.Error("expected second hook to still be called after first panicked")
	}

	// Verify the log entry was still written (hooks run after write)
	if buf.Len() == 0 {
		t.Error("expected log entry to be written despite hook panic")
	}
}

func TestNoHooksIsNoOp(t *testing.T) {
	// Just verify no panic when no hooks are configured
	var buf bytes.Buffer
	l := New(&buf, DEBUG)
	l.Info("no-hooks", nil)

	if buf.Len() == 0 {
		t.Error("expected log entry to be written")
	}
}
