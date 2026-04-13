package logger

import (
	"io"
	"os"
)

// Option configures a root Logger.
type Option func(*Logger)

// serviceIdentity holds metadata attached to every log entry.
type serviceIdentity struct {
	Service string
	Version string
	Env     string
	Host    string
}

// WithServiceIdentity sets service metadata that is automatically
// included in every log entry. The host is resolved via os.Hostname
// at construction time.
func WithServiceIdentity(service, version, env string) Option {
	return func(l *Logger) {
		l.identity = &serviceIdentity{
			Service: service,
			Version: version,
			Env:     env,
			Host:    hostname(),
		}
	}
}

// hostname returns the machine hostname, or "unknown" on error.
// Called once at logger construction, not per log entry.
func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// withExitFunc overrides the function called on FATAL (default: os.Exit).
// Unexported — intended for testing only.
func withExitFunc(fn func(int)) Option {
	return func(l *Logger) { l.exitFunc = fn }
}

// WithHooks registers hook functions that are called after every log
// entry is written. Hooks receive the level, message, and merged fields.
// They run synchronously under the logger's mutex — keep them fast.
func WithHooks(hooks ...Hook) Option {
	return func(l *Logger) { l.hooks = hooks }
}

// WithFormatter sets the output format for log entries.
// Defaults to JSONFormatter if not specified.
// Use TextFormatter{} for human-readable output in development.
func WithFormatter(f Formatter) Option {
	return func(l *Logger) { l.formatter = f }
}

// WithSampler sets a sampler that controls which log entries are written.
// ERROR and FATAL entries always pass through regardless of the sampler.
// Use NewEveryNSampler or NewRateSampler, or implement the Sampler interface.
func WithSampler(s Sampler) Option {
	return func(l *Logger) { l.sampler = s }
}

// WithCaller enables caller information (file and line number) in the
// "file" field of every log entry. Disabled by default because
// runtime.Caller has a measurable cost (~300–600ns per call).
func WithCaller() Option {
	return func(l *Logger) { l.caller = true }
}

// WithFullCallerPath includes the full file path (including package
// directory) in the "file" field instead of just the basename.
// Implies WithCaller().
func WithFullCallerPath() Option {
	return func(l *Logger) {
		l.caller = true
		l.fullCallerPath = true
	}
}

// WithEventID enables automatic generation of a unique event_id
// for every log entry, useful for deduplication in log pipelines.
func WithEventID() Option {
	return func(l *Logger) { l.eventID = true }
}

// WithFallbackWriter sets a fallback destination for log entries when
// the primary writer fails. This prevents log loss during incidents
// where the primary sink (file, network) is unavailable. The write
// error counter is still incremented on primary failure.
func WithFallbackWriter(w io.Writer) Option {
	return func(l *Logger) { l.fallbackWriter = w }
}

// WithMiddleware registers middleware functions that transform or filter
// log entries before they are written. Middleware runs in order after
// the entry is fully assembled (core keys, identity, fields, error
// enrichment). Return nil from a middleware to drop the entry.
func WithMiddleware(mw ...Middleware) Option {
	return func(l *Logger) { l.middleware = append(l.middleware, mw...) }
}

// WithAutoFormat selects the formatter automatically based on the
// output writer. If the writer is a terminal (e.g., os.Stderr in a
// local shell), TextFormatter with colors is used. Otherwise,
// JSONFormatter is used. This only takes effect if no explicit
// formatter has been set via WithFormatter.
func WithAutoFormat() Option {
	return func(l *Logger) {
		if f, ok := l.out.(*os.File); ok && isTerminal(f.Fd()) {
			l.formatter = TextFormatter{}
		} else {
			l.formatter = JSONFormatter{}
		}
	}
}
