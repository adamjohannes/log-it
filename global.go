package logger

import (
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
// and FromContext() when no logger is in the context.
func SetDefault(l *Logger) {
	defaultLogger.Store(l)
}

// ReplaceDefault sets a new default Logger and returns the previous one.
// Returns nil if no default was previously set.
func ReplaceDefault(l *Logger) *Logger {
	return defaultLogger.Swap(l)
}
