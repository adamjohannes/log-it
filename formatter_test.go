package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONFormatterProducesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG) // default is JSONFormatter
	l.Info("json-test", map[string]any{"k": "v"})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if entry["message"] != "json-test" {
		t.Errorf("expected message=json-test, got %v", entry["message"])
	}
}

func TestTextFormatterOutput(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	l.Info("hello world", map[string]any{"user": "alice"})

	line := buf.String()

	// Should NOT be valid JSON
	var js map[string]any
	if json.Unmarshal(buf.Bytes(), &js) == nil {
		t.Error("expected non-JSON output from TextFormatter")
	}

	// Should contain key parts
	if !strings.Contains(line, "INFO") {
		t.Errorf("expected INFO in output: %s", line)
	}
	if !strings.Contains(line, "hello world") {
		t.Errorf("expected message in output: %s", line)
	}
	if !strings.Contains(line, "user=alice") {
		t.Errorf("expected user=alice in output: %s", line)
	}
	// Should contain file:line in brackets
	if !strings.Contains(line, "[") || !strings.Contains(line, "]") {
		t.Errorf("expected [file:line] in output: %s", line)
	}
}

func TestTextFormatterNoColorHasNoANSI(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	l.Error("err", nil)

	if strings.Contains(buf.String(), "\033[") {
		t.Error("expected no ANSI escape codes with NoColor=true")
	}
}

func TestTextFormatterWithColorHasANSI(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: false}))
	l.Error("err", nil)

	if !strings.Contains(buf.String(), "\033[") {
		t.Error("expected ANSI escape codes with NoColor=false")
	}
}

func TestTextFormatterWithIdentity(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG,
		WithFormatter(TextFormatter{NoColor: true}),
		WithServiceIdentity("api", "1.0.0", "prod"),
	)
	l.Info("boot", nil)

	line := buf.String()
	if !strings.Contains(line, "service=api") {
		t.Errorf("expected service=api in output: %s", line)
	}
	if !strings.Contains(line, "env=prod") {
		t.Errorf("expected env=prod in output: %s", line)
	}
}

func TestWithFormatterOption(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	l.Info("opt-test", nil)

	// Verify it's text, not JSON
	if buf.Bytes()[0] == '{' {
		t.Error("expected text output, got JSON")
	}
}

func TestDefaultFormatterIsJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG) // no WithFormatter
	l.Info("default", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("default formatter should be JSON: %v", err)
	}
}

func TestColorizeAllLevels(t *testing.T) {
	levels := []string{"TRACE", "DEBUG", "INFO", "WARNING", "ERROR", "FATAL", "UNKNOWN"}
	for _, lvl := range levels {
		result := colorize(lvl)
		if lvl == "UNKNOWN" {
			if result != lvl {
				t.Errorf("expected no color for UNKNOWN, got %q", result)
			}
		} else {
			if !strings.Contains(result, "\033[") {
				t.Errorf("expected ANSI code for %s, got %q", lvl, result)
			}
			if !strings.Contains(result, lvl) {
				t.Errorf("expected level name in colored output for %s", lvl)
			}
		}
	}
}

func TestTextFormatterSanitizesNewlines(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	l.Info("test", map[string]any{"evil": "line1\nfake-line2\rcarriage"})

	output := buf.String()
	lines := strings.Count(output, "\n")
	// Should be exactly 1 newline (the trailing newline from writeEntry)
	if lines != 1 {
		t.Errorf("expected 1 line, got %d lines — log injection possible:\n%s", lines, output)
	}
	if !strings.Contains(output, `line1\nfake-line2\rcarriage`) {
		t.Errorf("expected escaped control chars in output: %s", output)
	}
}

func TestTextFormatterSanitizesMessage(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	l.Info("msg\ninjected line", nil)

	output := buf.String()
	lines := strings.Count(output, "\n")
	if lines != 1 {
		t.Errorf("expected 1 line for sanitized message, got %d:\n%s", lines, output)
	}
}
