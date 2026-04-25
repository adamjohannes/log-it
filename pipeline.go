package logger

// Syncer is implemented by writers that can flush buffered data.
// *os.File naturally satisfies this interface.
type Syncer interface {
	Sync() error
}

// Hook is called after a log entry is written.
// It receives the level, message, and the merged fields map.
//
// Hooks run synchronously under the logger's mutex — keep them fast.
// For slow operations (HTTP alerts, metrics push), launch a goroutine
// inside the hook.
type Hook func(level Level, message string, fields map[string]any)

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
