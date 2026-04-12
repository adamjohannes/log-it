package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ContextExtractor extracts structured fields from a context.Context.
// Register extractors via WithContextExtractor to automatically inject
// fields like trace_id or request_id into every log entry.
type ContextExtractor func(ctx context.Context) map[string]any

// Logger is a structured, leveled JSON logger.
// It supports child loggers via With(), runtime level changes,
// and automatic field extraction from context.Context.
type Logger struct {
	out        io.Writer
	minLevel   atomic.Int32
	mu         sync.Mutex
	fields     map[string]any
	parent     *Logger
	extractors []ContextExtractor
}

// New creates a root Logger that writes to out and discards
// entries below minLevel.
func New(out io.Writer, minLevel Level) *Logger {
	l := &Logger{
		out: out,
	}
	l.minLevel.Store(int32(minLevel))
	return l
}

// SetLevel atomically updates the minimum log level.
// On child loggers, this changes the root logger's level.
func (l *Logger) SetLevel(level Level) {
	l.root().minLevel.Store(int32(level))
}

// GetLevel atomically returns the current minimum log level.
func (l *Logger) GetLevel() Level {
	return Level(l.root().minLevel.Load())
}

// root returns the root logger. Since With() flattens the chain,
// this is at most one hop.
func (l *Logger) root() *Logger {
	if l.parent != nil {
		return l.parent
	}
	return l
}

// With creates a child logger that carries persistent fields.
// The child shares the root's writer, mutex, and level.
// Persistent fields are merged into every log entry automatically.
func (l *Logger) With(fields map[string]any) *Logger {
	r := l.root()
	return &Logger{
		parent:     r,
		fields:     mergeFields(l.fields, fields),
		extractors: l.extractors,
	}
}

// WithContextExtractor creates a child logger that runs fn on every
// *Context log call to extract fields from the provided context.
// Multiple extractors can be registered by chaining calls.
func (l *Logger) WithContextExtractor(fn ContextExtractor) *Logger {
	child := l.With(nil)
	child.extractors = make([]ContextExtractor, len(l.extractors)+1)
	copy(child.extractors, l.extractors)
	child.extractors[len(l.extractors)] = fn
	return child
}

// mergeFields combines two field maps. Overlay keys take precedence.
func mergeFields(base, overlay map[string]any) map[string]any {
	if len(base) == 0 {
		return overlay
	}
	if len(overlay) == 0 {
		return base
	}
	merged := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overlay {
		merged[k] = v
	}
	return merged
}

// internalLog checks the level, merges persistent fields, and writes the entry.
func (l *Logger) internalLog(level Level, message string, fields map[string]interface{}) {
	r := l.root()
	if level < Level(r.minLevel.Load()) {
		return
	}

	allFields := mergeFields(l.fields, fields)
	l.writeEntry(r, level, message, allFields)
}

// internalLogCtx is like internalLog but also runs context extractors.
// Field priority: per-call > context-extracted > persistent (.With()).
func (l *Logger) internalLogCtx(ctx context.Context, level Level, message string, fields map[string]any) {
	r := l.root()
	if level < Level(r.minLevel.Load()) {
		return
	}

	var ctxFields map[string]any
	if len(l.extractors) > 0 && ctx != nil {
		for _, extract := range l.extractors {
			ctxFields = mergeFields(ctxFields, extract(ctx))
		}
	}

	allFields := mergeFields(l.fields, mergeFields(ctxFields, fields))
	l.writeEntry(r, level, message, allFields)
}

// writeEntry collects caller info, serializes the entry as JSON,
// and writes it to the root logger's output under its mutex.
//
// runtime.Caller skip=3: writeEntry -> internalLog/internalLogCtx -> public method -> caller
func (l *Logger) writeEntry(r *Logger, level Level, message string, fields map[string]any) {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		file = "???"
		line = 0
	} else {
		if slash := strings.LastIndex(file, "/"); slash >= 0 {
			file = file[slash+1:]
		}
	}

	entry := struct {
		Time    string         `json:"time"`
		Level   string         `json:"level"`
		Message string         `json:"message"`
		File    string         `json:"file,omitempty"`
		Fields  map[string]any `json:"fields,omitempty"`
	}{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Level:   level.String(),
		Message: message,
		File:    fmt.Sprintf("%s:%d", file, line),
		Fields:  fields,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		data = []byte(fmt.Sprintf(
			`{"time":"%s","level":"ERROR","message":"falha ao converter entrada de log para json: %v"}`,
			time.Now().UTC().Format(time.RFC3339Nano),
			err,
		))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	data = append(data, '\n')
	_, _ = r.out.Write(data)

	if level == FATAL {
		os.Exit(1)
	}
}

// --- Structured log methods ---

func (l *Logger) Debug(message string, fields map[string]interface{})   { l.internalLog(DEBUG, message, fields) }
func (l *Logger) Info(message string, fields map[string]interface{})    { l.internalLog(INFO, message, fields) }
func (l *Logger) Warning(message string, fields map[string]interface{}) { l.internalLog(WARNING, message, fields) }
func (l *Logger) Error(message string, fields map[string]interface{})   { l.internalLog(ERROR, message, fields) }
func (l *Logger) Fatal(message string, fields map[string]interface{})   { l.internalLog(FATAL, message, fields) }

// --- Formatted log methods ---

func (l *Logger) Debugf(format string, v ...interface{})   { l.internalLog(DEBUG, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Infof(format string, v ...interface{})    { l.internalLog(INFO, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Warningf(format string, v ...interface{}) { l.internalLog(WARNING, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Errorf(format string, v ...interface{})   { l.internalLog(ERROR, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Fatalf(format string, v ...interface{})   { l.internalLog(FATAL, fmt.Sprintf(format, v...), nil) }

// --- Context-aware log methods ---

func (l *Logger) DebugContext(ctx context.Context, message string, fields map[string]any)   { l.internalLogCtx(ctx, DEBUG, message, fields) }
func (l *Logger) InfoContext(ctx context.Context, message string, fields map[string]any)    { l.internalLogCtx(ctx, INFO, message, fields) }
func (l *Logger) WarningContext(ctx context.Context, message string, fields map[string]any) { l.internalLogCtx(ctx, WARNING, message, fields) }
func (l *Logger) ErrorContext(ctx context.Context, message string, fields map[string]any)   { l.internalLogCtx(ctx, ERROR, message, fields) }
func (l *Logger) FatalContext(ctx context.Context, message string, fields map[string]any)   { l.internalLogCtx(ctx, FATAL, message, fields) }
