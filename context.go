package logger

import "context"

type loggerKey struct{}

// WithLogger stores a Logger in the context for later retrieval
// via FromContext. Use this in middleware to propagate a request-scoped
// logger through the call chain.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// FromContext retrieves the Logger stored by WithLogger.
// If no logger is found in the context (or ctx is nil), returns Default().
func FromContext(ctx context.Context) *Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey{}).(*Logger); ok {
			return l
		}
	}
	return Default()
}
