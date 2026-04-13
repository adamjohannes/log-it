package logger

import (
	"bytes"
	"testing"
)

func TestFilteredWriterPassesAboveLevel(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFilteredWriter(&buf, ERROR)
	l := New(fw, DEBUG)

	l.Info("should-drop", nil)
	l.Error("should-pass", nil)

	entries := decodeAllEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["message"] != "should-pass" {
		t.Errorf("expected should-pass, got %v", entries[0]["message"])
	}
}

func TestFilteredWriterDropsBelowLevel(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFilteredWriter(&buf, WARNING)
	l := New(fw, DEBUG)

	l.Debug("drop", nil)
	l.Info("drop", nil)

	if buf.Len() != 0 {
		t.Errorf("expected no output, got: %s", buf.String())
	}
}

func TestFilteredWriterWithFanOut(t *testing.T) {
	var infoBuf, errorBuf bytes.Buffer
	infoWriter := NewFilteredWriter(&infoBuf, INFO)
	errorWriter := NewFilteredWriter(&errorBuf, ERROR)
	fan := NewFanOutWriter(infoWriter, errorWriter)
	l := New(fan, DEBUG)

	l.Debug("debug-msg", nil)
	l.Info("info-msg", nil)
	l.Error("error-msg", nil)

	infoEntries := decodeAllEntries(t, &infoBuf)
	errorEntries := decodeAllEntries(t, &errorBuf)

	if len(infoEntries) != 2 { // INFO and ERROR pass INFO filter
		t.Errorf("expected 2 info entries, got %d", len(infoEntries))
	}
	if len(errorEntries) != 1 { // only ERROR passes ERROR filter
		t.Errorf("expected 1 error entry, got %d", len(errorEntries))
	}
}

func TestExtractLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{`{"level":"TRACE","message":"x"}`, TRACE},
		{`{"level":"DEBUG","message":"x"}`, DEBUG},
		{`{"level":"INFO","message":"x"}`, INFO},
		{`{"level":"WARNING","message":"x"}`, WARNING},
		{`{"level":"ERROR","message":"x"}`, ERROR},
		{`{"level":"FATAL","message":"x"}`, FATAL},
		{`{"message":"no level"}`, INFO},     // default
		{`not json at all`, INFO},             // default
	}
	for _, tt := range tests {
		got := extractLevel([]byte(tt.input))
		if got != tt.want {
			t.Errorf("extractLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
