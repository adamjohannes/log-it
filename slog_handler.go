package logger

import (
	"context"
	"log"
	"log/slog"
	"runtime"
)

// SlogHandler implements slog.Handler, bridging Go's standard
// structured logging into this logger. Use it with:
//
//	slogLogger := slog.New(logger.NewSlogHandler(myLogger))
type SlogHandler struct {
	logger *Logger
	attrs  []slog.Attr
	group  string
}

// NewSlogHandler creates a slog.Handler that routes log entries
// through the provided Logger.
func NewSlogHandler(l *Logger) *SlogHandler {
	return &SlogHandler{logger: l}
}

// Enabled reports whether the logger accepts records at the given slog level
// by mapping it to the underlying Logger's minimum level.
func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return slogLevelToLevel(level) >= h.logger.GetLevel()
}

// Handle converts a slog.Record into a map-based log entry, merging
// pre-set attributes and record attributes, and writes it through
// the underlying Logger. Source location is extracted from the record's PC.
func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	fields := make(map[string]any, record.NumAttrs()+len(h.attrs))

	// Pre-set attrs from WithAttrs
	for _, a := range h.attrs {
		fields[h.prefixKey(a.Key)] = a.Value.Any()
	}

	// Record attrs
	record.Attrs(func(a slog.Attr) bool {
		fields[h.prefixKey(a.Key)] = a.Value.Any()
		return true
	})

	// Extract source from record's PC if available
	if record.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{record.PC})
		f, _ := fs.Next()
		if f.File != "" {
			fields["slog_source"] = f.File + ":" + itoa(f.Line)
		}
	}

	level := slogLevelToLevel(record.Level)
	r := h.logger.root()

	if r.closed.Load() {
		return nil
	}

	allFields := mergeFields(h.logger.fields, fields)
	h.logger.writeEntry(r, level, record.Message, allFields)
	return nil
}

// WithAttrs returns a new handler that includes the given attributes
// in every subsequent Handle call.
func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &SlogHandler{logger: h.logger, attrs: newAttrs, group: h.group}
}

// WithGroup returns a new handler with the given group name.
// All subsequent attributes will be prefixed with "group.".
func (h *SlogHandler) WithGroup(name string) slog.Handler {
	prefix := name
	if h.group != "" {
		prefix = h.group + "." + name
	}
	return &SlogHandler{logger: h.logger, attrs: h.attrs, group: prefix}
}

// prefixKey prepends the group name to a key.
func (h *SlogHandler) prefixKey(key string) string {
	if h.group == "" {
		return key
	}
	return h.group + "." + key
}

// slogLevelToLevel maps slog levels to our Level type.
// slog.LevelDebug (-4) maps to DEBUG. Custom levels below that map to TRACE.
func slogLevelToLevel(l slog.Level) Level {
	switch {
	case l < slog.LevelDebug:
		return TRACE
	case l < slog.LevelInfo:
		return DEBUG
	case l < slog.LevelWarn:
		return INFO
	case l < slog.LevelError:
		return WARNING
	default:
		return ERROR
	}
}

// levelToSlogLevel maps our Level type to slog levels.
func levelToSlogLevel(l Level) slog.Level {
	switch l {
	case TRACE:
		return slog.LevelDebug - 4
	case DEBUG:
		return slog.LevelDebug
	case INFO:
		return slog.LevelInfo
	case WARNING:
		return slog.LevelWarn
	case ERROR:
		return slog.LevelError
	case FATAL:
		return slog.LevelError + 4
	default:
		return slog.LevelInfo
	}
}

// itoa is a simple int-to-string without importing strconv.
func itoa(i int) string {
	if i < 0 {
		return "-" + itoa(-i)
	}
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

// StdLogger returns a *log.Logger that routes writes through this
// logger at the specified level. Useful for bridging libraries that
// accept the standard library's *log.Logger.
func (l *Logger) StdLogger(level Level) *log.Logger {
	return slog.NewLogLogger(NewSlogHandler(l), levelToSlogLevel(level))
}
