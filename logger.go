package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	TRACE Level = iota
	DEBUG
	INFO
	WARNING
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case TRACE:
		return "TRACE"
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
	identity   *serviceIdentity
	closed     atomic.Bool
	exitFunc   func(code int)
	hooks          []Hook
	formatter      Formatter
	sampler        Sampler
	middleware     []Middleware
	caller         bool
	fullCallerPath bool
	eventID        bool
	writeErrors    atomic.Int64
	fallbackWriter io.Writer
}

// New creates a root Logger that writes to out and discards
// entries below minLevel. Options configure additional behavior
// such as service identity metadata.
func New(out io.Writer, minLevel Level, opts ...Option) *Logger {
	l := &Logger{
		out:       out,
		exitFunc:  os.Exit,
		formatter: JSONFormatter{},
	}
	l.minLevel.Store(int32(minLevel))
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Nop returns a logger that discards all output. Useful as a safe
// default in tests or when a logger is required but no output is wanted.
func Nop() *Logger {
	return New(io.Discard, FATAL+1)
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

// WriteErrorCount returns the number of times the underlying writer
// returned an error. Useful for monitoring sink health.
func (l *Logger) WriteErrorCount() int64 {
	return l.root().writeErrors.Load()
}

// Sync flushes any buffered log data to the underlying writer and
// prevents further log entries from being accepted. Call Sync before
// application exit to avoid losing log entries.
//
// For AsyncWriter destinations, this blocks until all queued entries
// are written. For FanOutWriter, each underlying writer is synced.
// For plain io.Writers (like os.Stdout), this is a no-op.
//
// Sync is safe to call on child loggers — it always syncs the root.
func (l *Logger) Sync() error {
	r := l.root()
	r.closed.Store(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.out.(Syncer); ok {
		err := s.Sync()
		if isBenignSyncError(err) {
			return nil
		}
		return err
	}
	return nil
}

// isBenignSyncError returns true for known harmless errors that occur
// when syncing non-file descriptors (e.g., stdout piped to a socket).
func isBenignSyncError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// "invalid argument" on Linux/macOS when stdout is not a file
	// "inappropriate ioctl for device" on some Unix systems
	return strings.Contains(msg, "invalid argument") ||
		strings.Contains(msg, "inappropriate ioctl for device")
}

// SyncWithTimeout is like Sync but returns an error if the flush
// doesn't complete within the given duration. Useful when the
// underlying sink may be slow or unreachable.
func (l *Logger) SyncWithTimeout(d time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- l.Sync() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		return errors.New("logger: sync timed out")
	}
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

// reservedKeys are core entry keys that cannot be overwritten by user fields.
// If a user field collides, it is prefixed with "fields." (e.g., "fields.level").
var reservedKeys = map[string]struct{}{
	"time": {}, "level": {}, "message": {}, "file": {},
	"service": {}, "version": {}, "env": {}, "host": {},
	"event_id": {},
}

// eventCounter provides unique sequence numbers for event IDs.
var eventCounter atomic.Uint64

// generateEventID creates a lightweight unique ID from timestamp + counter.
func generateEventID() string {
	ts := time.Now().UnixNano()
	seq := eventCounter.Add(1)
	return fmt.Sprintf("%x-%x", ts, seq)
}

// resolveLogValuers scans fields for values implementing slog.LogValuer
// and replaces them with their resolved values. This enables PII types
// that control their own log representation. Resolves recursively up to
// a depth of 10 to prevent infinite loops.
func resolveLogValuers(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return fields
	}
	var resolved map[string]any
	for k, v := range fields {
		if lv, ok := v.(slog.LogValuer); ok {
			if resolved == nil {
				resolved = make(map[string]any, len(fields))
				for fk, fv := range fields {
					resolved[fk] = fv
				}
			}
			val := lv.LogValue()
			for i := 0; i < 10; i++ {
				if inner, ok := val.Any().(slog.LogValuer); ok {
					val = inner.LogValue()
				} else {
					break
				}
			}
			resolved[k] = val.Any()
		}
	}
	if resolved != nil {
		return resolved
	}
	return fields
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
//
//go:noinline
func (l *Logger) internalLog(level Level, message string, fields map[string]any) {
	r := l.root()
	if r.closed.Load() {
		return
	}
	if level < Level(r.minLevel.Load()) {
		return
	}
	if s := r.sampler; s != nil && level < ERROR {
		if !s.Sample(level, message) {
			return
		}
	}

	allFields := mergeFields(l.fields, fields)
	l.writeEntry(r, level, message, allFields)
}

// internalLogCtx is like internalLog but also runs context extractors.
// Field priority: per-call > context-extracted > persistent (.With()).
//
//go:noinline
func (l *Logger) internalLogCtx(ctx context.Context, level Level, message string, fields map[string]any) {
	r := l.root()
	if r.closed.Load() {
		return
	}
	if level < Level(r.minLevel.Load()) {
		return
	}
	if s := r.sampler; s != nil && level < ERROR {
		if !s.Sample(level, message) {
			return
		}
	}

	var ctxFields map[string]any
	if len(l.extractors) > 0 && ctx != nil {
		for _, extract := range l.extractors {
			func() {
				defer func() { _ = recover() }()
				ctxFields = mergeFields(ctxFields, extract(ctx))
			}()
		}
	}

	allFields := mergeFields(l.fields, mergeFields(ctxFields, fields))
	l.writeEntry(r, level, message, allFields)
}

// writeEntry collects caller info, serializes the entry as flat JSON,
// and writes it to the root logger's output under its mutex.
//
// runtime.Caller skip=3: writeEntry -> internalLog/internalLogCtx -> public method -> caller
//
//go:noinline
func (l *Logger) writeEntry(r *Logger, level Level, message string, fields map[string]any) {
	entry := make(map[string]any, 4+len(fields))
	entry["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	entry["level"] = level.String()
	entry["message"] = message

	if r.caller {
		_, file, line, ok := runtime.Caller(3)
		if !ok {
			file = "???"
			line = 0
		} else if !r.fullCallerPath {
			if slash := strings.LastIndex(file, "/"); slash >= 0 {
				file = file[slash+1:]
			}
		}
		entry["file"] = fmt.Sprintf("%s:%d", file, line)
	}

	if id := r.identity; id != nil {
		entry["service"] = id.Service
		entry["version"] = id.Version
		entry["env"] = id.Env
		entry["host"] = id.Host
	}

	if r.eventID {
		entry["event_id"] = generateEventID()
	}

	fields = resolveLogValuers(fields)
	fields = enrichErrors(fields)

	for k, v := range fields {
		if _, ok := reservedKeys[k]; ok {
			entry["fields."+k] = v
		} else {
			entry[k] = v
		}
	}

	// Run middleware chain; nil return means drop the entry
	for _, mw := range r.middleware {
		entry = mw(entry)
		if entry == nil {
			return
		}
	}

	data, err := r.formatter.Format(entry)
	if err != nil {
		fallback := map[string]string{
			"time":    time.Now().UTC().Format(time.RFC3339Nano),
			"level":   "ERROR",
			"message": "failed to marshal log entry to json: " + err.Error(),
		}
		data, _ = json.Marshal(fallback)
	}

	// Use pooled buffer to append newline and write atomically
	writeBuf := bufPool.Get().(*bytes.Buffer)
	writeBuf.Reset()
	writeBuf.Write(data)
	writeBuf.WriteByte('\n')

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.out.Write(writeBuf.Bytes()); err != nil {
		r.writeErrors.Add(1)
		if r.fallbackWriter != nil {
			_, _ = r.fallbackWriter.Write(writeBuf.Bytes())
		}
	}
	bufPool.Put(writeBuf)

	if hooks := r.hooks; len(hooks) > 0 {
		for _, hook := range hooks {
			func() {
				defer func() { _ = recover() }()
				hook(level, message, fields)
			}()
		}
	}

	if level == FATAL {
		// Flush async-buffered logs before exiting. We're already holding
		// the mutex so call Sync on the writer directly (not l.Sync()).
		if s, ok := r.out.(Syncer); ok {
			_ = s.Sync()
		}
		r.exitFunc(1)
	}
}

// --- Structured log methods ---

func (l *Logger) Trace(message string, fields map[string]any)   { l.internalLog(TRACE, message, fields) }
func (l *Logger) Debug(message string, fields map[string]any)   { l.internalLog(DEBUG, message, fields) }
func (l *Logger) Info(message string, fields map[string]any)    { l.internalLog(INFO, message, fields) }
func (l *Logger) Warning(message string, fields map[string]any) { l.internalLog(WARNING, message, fields) }
func (l *Logger) Error(message string, fields map[string]any)   { l.internalLog(ERROR, message, fields) }
func (l *Logger) Fatal(message string, fields map[string]any)   { l.internalLog(FATAL, message, fields) }

// --- Formatted log methods ---

func (l *Logger) Tracef(format string, v ...any)   { l.internalLog(TRACE, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Debugf(format string, v ...any)   { l.internalLog(DEBUG, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Infof(format string, v ...any)    { l.internalLog(INFO, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Warningf(format string, v ...any) { l.internalLog(WARNING, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Errorf(format string, v ...any)   { l.internalLog(ERROR, fmt.Sprintf(format, v...), nil) }
func (l *Logger) Fatalf(format string, v ...any)   { l.internalLog(FATAL, fmt.Sprintf(format, v...), nil) }

// --- Context-aware log methods ---

func (l *Logger) TraceContext(ctx context.Context, message string, fields map[string]any)   { l.internalLogCtx(ctx, TRACE, message, fields) }
func (l *Logger) DebugContext(ctx context.Context, message string, fields map[string]any)   { l.internalLogCtx(ctx, DEBUG, message, fields) }
func (l *Logger) InfoContext(ctx context.Context, message string, fields map[string]any)    { l.internalLogCtx(ctx, INFO, message, fields) }
func (l *Logger) WarningContext(ctx context.Context, message string, fields map[string]any) { l.internalLogCtx(ctx, WARNING, message, fields) }
func (l *Logger) ErrorContext(ctx context.Context, message string, fields map[string]any)   { l.internalLogCtx(ctx, ERROR, message, fields) }
func (l *Logger) FatalContext(ctx context.Context, message string, fields map[string]any)   { l.internalLogCtx(ctx, FATAL, message, fields) }

// --- Typed field log methods (zero-allocation constructors) ---

func (l *Logger) Tracew(message string, fields ...Field)   { l.internalLog(TRACE, message, fieldsToMap(fields)) }
func (l *Logger) Debugw(message string, fields ...Field)   { l.internalLog(DEBUG, message, fieldsToMap(fields)) }
func (l *Logger) Infow(message string, fields ...Field)    { l.internalLog(INFO, message, fieldsToMap(fields)) }
func (l *Logger) Warningw(message string, fields ...Field) { l.internalLog(WARNING, message, fieldsToMap(fields)) }
func (l *Logger) Errorw(message string, fields ...Field)   { l.internalLog(ERROR, message, fieldsToMap(fields)) }
func (l *Logger) Fatalw(message string, fields ...Field)   { l.internalLog(FATAL, message, fieldsToMap(fields)) }
