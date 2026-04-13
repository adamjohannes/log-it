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

func TestTextFormatterStripsANSIFromValues(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	l.Info("test", map[string]any{"injected": "\033[31mred text\033[0m"})

	output := buf.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI codes to be stripped, got: %s", output)
	}
	if !strings.Contains(output, "red text") {
		t.Errorf("expected text content preserved after ANSI strip: %s", output)
	}
}

func TestTextFormatterStripsANSIFromMessage(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{NoColor: true}))
	l.Info("\033[1mbold message\033[0m", nil)

	output := buf.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI codes to be stripped from message, got: %s", output)
	}
	if !strings.Contains(output, "bold message") {
		t.Errorf("expected message text preserved: %s", output)
	}
}

func TestStripANSINoEscapePassthrough(t *testing.T) {
	input := "no escape here"
	if got := stripANSI(input); got != input {
		t.Errorf("expected passthrough, got %q", got)
	}
}

// --- KeyMap remapping tests ---

func TestJSONFormatterKeyMap(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(JSONFormatter{
		KeyMap: map[string]string{"level": "severity", "message": "msg"},
	}))
	l.Info("remapped", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["severity"] != "INFO" {
		t.Errorf("expected severity=INFO, got %v", entry["severity"])
	}
	if entry["msg"] != "remapped" {
		t.Errorf("expected msg=remapped, got %v", entry["msg"])
	}
	if _, exists := entry["level"]; exists {
		t.Error("expected 'level' key to be remapped away")
	}
}

func TestGCPKeyMapPreset(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(JSONFormatter{KeyMap: GCPKeyMap}))
	l.Info("gcp-test", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["severity"] != "INFO" {
		t.Errorf("expected severity=INFO, got %v", entry["severity"])
	}
	if entry["textPayload"] != "gcp-test" {
		t.Errorf("expected textPayload=gcp-test, got %v", entry["textPayload"])
	}
}

func TestDatadogKeyMapPreset(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(JSONFormatter{KeyMap: DatadogKeyMap}))
	l.Info("dd-test", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["status"] != "INFO" {
		t.Errorf("expected status=INFO, got %v", entry["status"])
	}
	if _, exists := entry["level"]; exists {
		t.Error("expected 'level' key to be remapped to 'status'")
	}
}

func TestELKKeyMapPreset(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(JSONFormatter{KeyMap: ELKKeyMap}))
	l.Info("elk-test", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if _, exists := entry["@timestamp"]; !exists {
		t.Error("expected @timestamp key from ELKKeyMap")
	}
	if entry["log.level"] != "INFO" {
		t.Errorf("expected log.level=INFO, got %v", entry["log.level"])
	}
}

func TestTextFormatterKeyMap(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithFormatter(TextFormatter{
		NoColor: true,
		KeyMap:  map[string]string{"level": "severity"},
	}))
	l.Info("test", nil)

	// TextFormatter with KeyMap should still produce readable output
	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Errorf("expected message in output: %s", output)
	}
}

func TestAutoFormatNonTerminalUsesJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, INFO, WithAutoFormat())
	l.Info("auto", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected JSON output for non-terminal writer, got: %s", buf.String())
	}
	if entry["message"] != "auto" {
		t.Errorf("expected message=auto, got %v", entry["message"])
	}
}

func TestAutoFormatUnwrapsAsyncWriter(t *testing.T) {
	// AsyncWriter around a bytes.Buffer is still not a terminal
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 64)
	l := New(aw, INFO, WithAutoFormat())
	l.Info("unwrapped", nil)
	_ = l.Sync()

	// Should be JSON (Buffer is not a terminal)
	entries := decodeAllEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["message"] != "unwrapped" {
		t.Errorf("expected message=unwrapped, got %v", entries[0]["message"])
	}
}
