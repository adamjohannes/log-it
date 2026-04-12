package logger

// Hook is called after a log entry is written.
// It receives the level, message, and the merged fields map.
//
// Hooks run synchronously under the logger's mutex — keep them fast.
// For slow operations (HTTP alerts, metrics push), launch a goroutine
// inside the hook.
type Hook func(level Level, message string, fields map[string]any)
