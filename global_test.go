package logger

import (
	"bytes"
	"testing"
)

func TestDefaultReturnsUsableLogger(t *testing.T) {
	l := Default()
	if l == nil {
		t.Fatal("expected non-nil default logger")
	}
	// Should not panic
	l.Info("default-test", nil)
}

func TestSetDefaultChangesGlobal(t *testing.T) {
	var buf bytes.Buffer
	custom := New(&buf, DEBUG)

	prev := ReplaceDefault(custom)
	defer func() {
		// Restore previous default to not affect other tests
		if prev != nil {
			SetDefault(prev)
		} else {
			defaultLogger.Store(nil)
		}
	}()

	got := Default()
	if got != custom {
		t.Error("expected Default() to return the custom logger after SetDefault")
	}

	got.Info("custom-default", nil)
	if buf.Len() == 0 {
		t.Error("expected output from custom default logger")
	}
}

func TestReplaceDefaultReturnsPrevious(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	l1 := New(&buf1, DEBUG)
	l2 := New(&buf2, DEBUG)

	old := ReplaceDefault(l1)
	defer func() {
		if old != nil {
			SetDefault(old)
		} else {
			defaultLogger.Store(nil)
		}
	}()

	prev := ReplaceDefault(l2)
	defer SetDefault(l2) // cleanup

	if prev != l1 {
		t.Error("expected ReplaceDefault to return the previous logger")
	}
}

func TestFromContextFallsBackToDefault(t *testing.T) {
	var buf bytes.Buffer
	custom := New(&buf, DEBUG)

	prev := ReplaceDefault(custom)
	defer func() {
		if prev != nil {
			SetDefault(prev)
		} else {
			defaultLogger.Store(nil)
		}
	}()

	// Empty context should fall back to Default()
	l := FromContext(nil) //nolint:staticcheck // deliberately testing nil context
	if l != custom {
		t.Error("expected FromContext to fall back to custom Default")
	}
}
