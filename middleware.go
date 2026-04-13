package logger

// Middleware transforms or filters a log entry before it is written.
// It receives the fully assembled entry map (including core keys like
// "time", "level", "message") and returns the (possibly modified) entry.
//
// Return nil to drop the entry entirely (it will not be written or
// trigger hooks).
//
// Middleware functions run in order under the logger's mutex, so they
// must be fast and must not call back into the logger.
type Middleware func(entry map[string]any) map[string]any
