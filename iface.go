package logger

// Interface is a minimal logging interface for dependency injection and
// mocking. *Logger satisfies this interface implicitly.
//
// Libraries and services should accept Interface (or a subset of it)
// rather than the concrete *Logger type so that callers can substitute
// test doubles, no-op loggers, or alternative implementations.
//
//	type OrderService struct {
//	    log logger.Interface
//	}
type Interface interface {
	Trace(message string, fields map[string]any)
	Debug(message string, fields map[string]any)
	Info(message string, fields map[string]any)
	Warning(message string, fields map[string]any)
	Error(message string, fields map[string]any)
	With(fields map[string]any) *Logger
}

// Compile-time check: *Logger must satisfy Interface.
var _ Interface = (*Logger)(nil)
