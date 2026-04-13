package logger

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRedactFieldsBasic(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithRedactFields("password", "token")},
		func(l *Logger) {
			l.Info("login", map[string]any{
				"user":     "alice",
				"password": "secret123",
				"token":    "abc-xyz",
			})
		},
	)
	if entry["password"] != "[REDACTED]" {
		t.Errorf("expected password=[REDACTED], got %v", entry["password"])
	}
	if entry["token"] != "[REDACTED]" {
		t.Errorf("expected token=[REDACTED], got %v", entry["token"])
	}
	if entry["user"] != "alice" {
		t.Errorf("expected user=alice, got %v", entry["user"])
	}
}

func TestRedactFieldsNested(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithRedactFields("secret")},
		func(l *Logger) {
			l.Infow("nested", Group("auth",
				String("user", "bob"),
				String("secret", "hidden"),
			))
		},
	)
	auth, ok := entry["auth"].(map[string]any)
	if !ok {
		t.Fatalf("expected auth to be a nested map, got %T", entry["auth"])
	}
	if auth["secret"] != "[REDACTED]" {
		t.Errorf("expected nested secret=[REDACTED], got %v", auth["secret"])
	}
	if auth["user"] != "bob" {
		t.Errorf("expected nested user=bob, got %v", auth["user"])
	}
}

func TestRedactFieldsCustomReplacement(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithRedactFieldsFunc("***", "password")},
		func(l *Logger) {
			l.Info("custom", map[string]any{"password": "secret"})
		},
	)
	if entry["password"] != "***" {
		t.Errorf("expected password=***, got %v", entry["password"])
	}
}

func TestRedactFieldsEmptyList(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithRedactFields()},
		func(l *Logger) {
			l.Info("noop", map[string]any{"password": "visible"})
		},
	)
	if entry["password"] != "visible" {
		t.Errorf("expected password=visible (no redaction), got %v", entry["password"])
	}
}

func TestRedactFieldsNonMatchingUnchanged(t *testing.T) {
	entry := captureLogWithOpts(
		[]Option{WithRedactFields("secret")},
		func(l *Logger) {
			l.Info("safe", map[string]any{"user": "alice", "status": float64(200)})
		},
	)
	if entry["user"] != "alice" {
		t.Errorf("user should be unchanged, got %v", entry["user"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("status should be unchanged, got %v", entry["status"])
	}
}

func TestRedactFieldsWithChild(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG, WithRedactFields("token"))
	child := root.With(map[string]any{"component": "auth"})
	child.Info("request", map[string]any{"token": "abc"})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["token"] != "[REDACTED]" {
		t.Errorf("expected token=[REDACTED] via child, got %v", entry["token"])
	}
}
