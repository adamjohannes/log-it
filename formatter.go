package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Formatter serializes a log entry map into bytes.
type Formatter interface {
	Format(entry map[string]any) ([]byte, error)
}

// JSONFormatter serializes entries as single-line JSON.
// This is the default formatter.
type JSONFormatter struct{}

// Format marshals the entry map to JSON.
func (JSONFormatter) Format(entry map[string]any) ([]byte, error) {
	return json.Marshal(entry)
}

// TextFormatter serializes entries as human-readable text, suitable
// for local development. Optionally includes ANSI color codes.
//
// Output format:
//
//	2026-04-12T14:32:07Z INFO    [main.go:42] hello  user_id=42 status=ok
type TextFormatter struct {
	NoColor bool // disable ANSI color codes
}

// Format renders the entry as a single line of human-readable text.
func (f TextFormatter) Format(entry map[string]any) ([]byte, error) {
	var buf bytes.Buffer

	ts, _ := entry["time"].(string)
	level, _ := entry["level"].(string)
	msg, _ := entry["message"].(string)
	file, _ := entry["file"].(string)

	displayLevel := level
	if !f.NoColor {
		displayLevel = colorize(level)
	}

	fmt.Fprintf(&buf, "%s %-7s [%s] %s", ts, displayLevel, file, msg)

	// Collect extra keys in sorted order for deterministic output
	coreKeys := map[string]struct{}{
		"time": {}, "level": {}, "message": {}, "file": {},
	}
	var extraKeys []string
	for k := range entry {
		if _, ok := coreKeys[k]; !ok {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)

	for _, k := range extraKeys {
		fmt.Fprintf(&buf, "  %s=%v", k, entry[k])
	}

	return buf.Bytes(), nil
}

// colorize wraps a level string with ANSI color codes.
func colorize(level string) string {
	switch level {
	case "TRACE":
		return "\033[90m" + level + "\033[0m" // gray
	case "DEBUG":
		return "\033[36m" + level + "\033[0m" // cyan
	case "INFO":
		return "\033[32m" + level + "\033[0m" // green
	case "WARNING":
		return "\033[33m" + level + "\033[0m" // yellow
	case "ERROR":
		return "\033[31m" + level + "\033[0m" // red
	case "FATAL":
		return "\033[35m" + level + "\033[0m" // magenta
	default:
		return level
	}
}
