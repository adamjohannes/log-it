// Package logger provides a structured, leveled JSON logger for Go with
// zero external dependencies.
//
// Create a logger with [New], attach persistent fields with [Logger.With],
// and call level methods ([Logger.Info], [Logger.Error], etc.) to emit
// structured entries:
//
//	log := logger.New(os.Stdout, logger.INFO)
//	defer log.Sync()
//
//	log.Info("request handled", map[string]any{"status": 200, "path": "/api/health"})
//
// Three output formats are built in: [JSONFormatter] (default),
// [TextFormatter] for local development, and [LogfmtFormatter] for
// logfmt-aware systems. Format and level can also be driven by
// environment variables via [WithEnvConfig].
//
// For stdlib interop, [NewSlogHandler] bridges log/slog to this logger.
package logger
