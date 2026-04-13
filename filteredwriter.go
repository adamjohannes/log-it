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
}

// NewFilteredWriter creates a writer that only passes through entries
// at or above the given minimum level.
func NewFilteredWriter(w io.Writer, minLevel Level) *FilteredWriter {
	return &FilteredWriter{inner: w, minLevel: minLevel}
}

// Write inspects the JSON entry for a level field and drops entries
// below the minimum level. Non-JSON or unparseable entries are passed
// through unchanged.
func (fw *FilteredWriter) Write(p []byte) (int, error) {
	level := extractLevel(p)
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

// extractLevel parses the level from a JSON log entry without
// unmarshalling the entire entry. Returns INFO if unparseable.
func extractLevel(p []byte) Level {
	// Look for "level":"VALUE" in the JSON bytes
	key := []byte(`"level":"`)
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
