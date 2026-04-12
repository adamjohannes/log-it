package logger

import "os"

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
