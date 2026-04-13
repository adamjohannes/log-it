package logger

import (
	"bytes"
	"io"
)

// FilteredWriter wraps an io.Writer and only passes through log entries
// whose level is at or above the configured minimum. It inspects the
// JSON-encoded log entry for the "level" field to make the decision.
//
// Use with FanOutWriter to send different levels to different sinks:
//
//	fan := NewFanOutWriter(
//	    NewFilteredWriter(stdoutWriter, INFO),     // INFO+ to stdout
//	    NewFilteredWriter(errorFileWriter, ERROR), // ERROR+ to file
//	)
type FilteredWriter struct {
	inner    io.Writer
	minLevel Level
	levelKey string
}

// NewFilteredWriter creates a writer that only passes through entries
// at or above the given minimum level. By default it looks for the
// "level" key in JSON output. Use WithLevelKey to match a remapped
// key name (e.g., "severity" for GCPKeyMap, "status" for DatadogKeyMap).
func NewFilteredWriter(w io.Writer, minLevel Level, opts ...FilteredWriterOption) *FilteredWriter {
	fw := &FilteredWriter{inner: w, minLevel: minLevel, levelKey: "level"}
	for _, opt := range opts {
		opt(fw)
	}
	return fw
}

// FilteredWriterOption configures a FilteredWriter.
type FilteredWriterOption func(*FilteredWriter)

// WithLevelKey sets the JSON key name used to extract the log level.
// Use this when the logger is configured with a KeyMap that remaps
// the "level" key (e.g., WithLevelKey("severity") for GCPKeyMap).
func WithLevelKey(key string) FilteredWriterOption {
	return func(fw *FilteredWriter) { fw.levelKey = key }
}

// Write inspects the JSON entry for a level field and drops entries
// below the minimum level. Non-JSON or unparseable entries are passed
// through unchanged.
func (fw *FilteredWriter) Write(p []byte) (int, error) {
	level := extractLevel(p, fw.levelKey)
	if level < fw.minLevel {
		return len(p), nil // silently drop
	}
	return fw.inner.Write(p)
}

// Sync delegates to the inner writer if it implements Syncer.
func (fw *FilteredWriter) Sync() error {
	if s, ok := fw.inner.(Syncer); ok {
		return s.Sync()
	}
	return nil
}

// Unwrap returns the inner writer. This enables WithAutoFormat to
// detect terminal writers through wrapper layers.
func (fw *FilteredWriter) Unwrap() io.Writer {
	return fw.inner
}

// extractLevel parses the level from a JSON log entry without
// unmarshalling the entire entry. Returns INFO if unparseable.
func extractLevel(p []byte, levelKey string) Level {
	// Build search pattern: "key":"
	key := []byte(`"` + levelKey + `":"`)
	idx := bytes.Index(p, key)
	if idx < 0 {
		return INFO
	}
	start := idx + len(key)
	end := bytes.IndexByte(p[start:], '"')
	if end < 0 {
		return INFO
	}
	switch string(p[start : start+end]) {
	case "TRACE":
		return TRACE
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARNING":
		return WARNING
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}
