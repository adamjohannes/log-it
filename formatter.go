package logger

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// bufPool provides reusable byte buffers for formatters and the write path.
var bufPool = sync.Pool{
	New: func() any { return bytes.NewBuffer(make([]byte, 0, 256)) },
}

// Formatter serializes a log entry map into bytes.
type Formatter interface {
	Format(entry map[string]any) ([]byte, error)
}

// JSONFormatter serializes entries as single-line JSON.
// This is the default formatter.
type JSONFormatter struct {
	// KeyMap remaps core field names before serialization.
	// Example: {"level": "severity"} renames the "level" key to "severity".
	KeyMap map[string]string
}

// Format marshals the entry map to JSON, applying key remapping if configured.
// Uses a hand-rolled encoder for performance; falls back to encoding/json
// for types it cannot handle directly.
func (f JSONFormatter) Format(entry map[string]any) ([]byte, error) {
	if len(f.KeyMap) > 0 {
		entry = applyKeyMap(entry, f.KeyMap)
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	result := appendJSONEntry(buf.Bytes(), entry)
	// result may have grown past buf's backing array, so copy
	out := make([]byte, len(result))
	copy(out, result)
	buf.Reset()
	bufPool.Put(buf)
	return out, nil
}

// TextFormatter serializes entries as human-readable text, suitable
// for local development. Optionally includes ANSI color codes.
//
// Output format:
//
//	2026-04-12T14:32:07Z INFO    [main.go:42] hello  user_id=42 status=ok
type TextFormatter struct {
	NoColor bool              // disable ANSI color codes
	KeyMap  map[string]string // remap core field names
}

// Format renders the entry as a single line of human-readable text.
func (f TextFormatter) Format(entry map[string]any) ([]byte, error) {
	if len(f.KeyMap) > 0 {
		entry = applyKeyMap(entry, f.KeyMap)
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	ts, _ := entry["time"].(string)
	level, _ := entry["level"].(string)
	msg, _ := entry["message"].(string)
	file, _ := entry["file"].(string)

	displayLevel := level
	if !f.NoColor {
		displayLevel = colorize(level)
	}

	fmt.Fprintf(buf, "%s %-7s [%s] %s", ts, displayLevel, file, sanitizeText(msg))

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
		fmt.Fprintf(buf, "  %s=%s", k, sanitizeText(fmt.Sprintf("%v", entry[k])))
	}

	// Copy result before returning buffer to pool
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
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

// sanitizeText escapes control characters that could create fake log
// lines or corrupt text output (log injection prevention), and strips
// ANSI escape sequences that could alter terminal rendering.
func sanitizeText(s string) string {
	s = stripANSI(s)
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// stripANSI removes ANSI escape sequences (ESC[...X) from s.
// Uses a simple state machine rather than regexp to avoid the import.
func stripANSI(s string) string {
	// Fast path: no ESC byte means no ANSI sequences
	if !strings.Contains(s, "\033") {
		return s
	}
	b := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip ESC[ and everything up to the terminating letter
			i += 2
			for i < len(s) && !isANSITerminator(s[i]) {
				i++
			}
			if i < len(s) {
				i++ // skip the terminator
			}
		} else {
			b = append(b, s[i])
			i++
		}
	}
	return string(b)
}

// isANSITerminator returns true if b is a valid ANSI CSI final byte.
func isANSITerminator(b byte) bool {
	return b >= 0x40 && b <= 0x7E
}

// applyKeyMap renames keys in the entry map according to the provided mapping.
func applyKeyMap(entry map[string]any, keyMap map[string]string) map[string]any {
	remapped := make(map[string]any, len(entry))
	for k, v := range entry {
		if newKey, ok := keyMap[k]; ok {
			remapped[newKey] = v
		} else {
			remapped[k] = v
		}
	}
	return remapped
}

// GCPKeyMap is a key remapping preset for Google Cloud Logging compatibility.
var GCPKeyMap = map[string]string{
	"level":   "severity",
	"message": "textPayload",
}

// DatadogKeyMap is a key remapping preset for Datadog Log Management.
// Datadog expects "status" for the severity level.
var DatadogKeyMap = map[string]string{
	"level": "status",
}

// ELKKeyMap is a key remapping preset for Elastic/ELK stack compatibility.
// Elasticsearch expects "@timestamp" and "log.level".
var ELKKeyMap = map[string]string{
	"time":  "@timestamp",
	"level": "log.level",
}
