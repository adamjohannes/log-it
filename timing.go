package logger

import (
	"context"
	"time"
)

// Timed starts a timer and returns a function that, when called,
// logs the elapsed duration. Use with defer for automatic timing:
//
//	done := logger.Timed("db_query")
//	defer done()
func (l *Logger) Timed(operation string) func() {
	start := time.Now()
	return func() {
		l.Info(operation, map[string]any{
			"duration_ms": float64(time.Since(start).Microseconds()) / 1000.0,
		})
	}
}

// TimedContext is like Timed but uses context-aware logging,
// allowing context extractors to inject fields.
func (l *Logger) TimedContext(ctx context.Context, operation string) func() {
	start := time.Now()
	return func() {
		l.InfoContext(ctx, operation, map[string]any{
			"duration_ms": float64(time.Since(start).Microseconds()) / 1000.0,
		})
	}
}
