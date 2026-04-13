package logger

import (
	"log/slog"
	"os"
	"sync/atomic"
)

var defaultLogger atomic.Pointer[Logger]

// Default returns the global default Logger. If no default has been
// set via SetDefault, it returns a logger writing JSON to stderr at INFO level.
func Default() *Logger {
	if l := defaultLogger.Load(); l != nil {
		return l
	}
	return New(os.Stderr, INFO)
}

// SetDefault sets the global default Logger returned by Default()
// and FromContext() when no logger is in the context. It also sets
// slog.Default() so that code using the standard library's slog
// package is routed through this logger.
func SetDefault(l *Logger) {
	defaultLogger.Store(l)
	slog.SetDefault(slog.New(NewSlogHandler(l)))
}

// ReplaceDefault sets a new default Logger and returns the previous one.
// Returns nil if no default was previously set. Also updates slog.Default().
func ReplaceDefault(l *Logger) *Logger {
	prev := defaultLogger.Swap(l)
	slog.SetDefault(slog.New(NewSlogHandler(l)))
	return prev
}
