# log-it

A structured, leveled JSON logger for Go. Pure stdlib, zero dependencies, 138 tests.

```go
import logger "github.com/adamjohannes/log-it"
```

## Install

```bash
go get github.com/adamjohannes/log-it
```

Requires Go 1.21+.

## Quick Start

```go
package main

import (
	"os"

	logger "github.com/adamjohannes/log-it"
)

func main() {
	log := logger.New(os.Stdout, logger.INFO)
	defer log.Sync()

	log.Info("server started", map[string]any{"port": 8080})
	// {"file":"main.go:14","level":"INFO","message":"server started","port":8080,"time":"2026-04-12T..."}
}
```

## Features

- **6 log levels**: `TRACE`, `DEBUG`, `INFO`, `WARNING`, `ERROR`, `FATAL`
- **4 logging styles**: structured (map), formatted (printf), context-aware, typed fields
- **Flat JSON output**: fields at top level for observability platform compatibility
- **Child loggers**: `With()` for persistent fields, shared writer/mutex/level
- **Context propagation**: extract trace IDs, request IDs from `context.Context`
- **Service identity**: auto-attach `service`, `version`, `env`, `host` to every entry
- **Async buffered writes**: non-blocking channel with lossy back-pressure
- **Fan-out writer**: broadcast to multiple destinations simultaneously
- **Pluggable formatters**: JSON (default) or human-readable text with ANSI colors
- **Sampling / rate limiting**: prevent log storms from overwhelming sinks
- **Hooks**: post-write callbacks for alerting, metrics, PII scrubbing
- **Error enrichment**: auto-detect `error` values, add `_type` and `_chain` fields
- **`log/slog` integration**: `slog.Handler` adapter for ecosystem compatibility
- **Timing helpers**: `Timed()` / `TimedContext()` for defer-friendly duration logging
- **Event IDs**: unique per-entry IDs for pipeline deduplication
- **Graceful shutdown**: `Sync()` flushes all buffered entries before exit

## Logging Styles

### Structured (map-based)

```go
log.Info("request handled", map[string]any{
    "method":  "GET",
    "path":    "/api/users",
    "status":  200,
    "latency": 12.5,
})
```

### Formatted (printf-style)

```go
log.Infof("listening on port %d", 8080)
```

### Context-aware

```go
log.InfoContext(ctx, "db query", map[string]any{"table": "users"})
```

### Typed fields (zero-allocation constructors)

```go
log.Infow("request handled",
    logger.String("method", "GET"),
    logger.Int("status", 200),
    logger.Float64("latency_ms", 12.5),
    logger.Duration("elapsed", 350*time.Millisecond),
    logger.Bool("cached", true),
    logger.Err(err),
)
```

Available constructors: `String`, `Int`, `Int64`, `Float64`, `Bool`, `Err`, `Duration`, `Any`.

## Child Loggers

```go
// Persistent fields attached to every entry
reqLog := log.With(map[string]any{
    "request_id": "abc-123",
    "user_id":    42,
})
reqLog.Info("processing", nil)
// Output includes request_id and user_id automatically
```

## Context Extraction

```go
log := logger.New(os.Stdout, logger.INFO)
log = log.WithContextExtractor(func(ctx context.Context) map[string]any {
    if traceID := ctx.Value("trace_id"); traceID != nil {
        return map[string]any{"trace_id": traceID}
    }
    return nil
})

log.InfoContext(ctx, "handled", nil)
// trace_id auto-extracted from context
```

## Configuration Options

```go
log := logger.New(os.Stdout, logger.INFO,
    logger.WithServiceIdentity("myapp", "1.2.3", "prod"),
    logger.WithFormatter(logger.TextFormatter{NoColor: false}),
    logger.WithSampler(logger.NewRateSampler(100)),
    logger.WithHooks(myAlertHook),
    logger.WithEventID(),
    logger.WithFullCallerPath(),
)
```

| Option | Description |
|--------|-------------|
| `WithServiceIdentity(service, version, env)` | Attach `service`, `version`, `env`, `host` to every entry |
| `WithFormatter(f)` | Set output format (`JSONFormatter{}` or `TextFormatter{}`) |
| `WithSampler(s)` | Enable sampling (`NewEveryNSampler(n)` or `NewRateSampler(n)`) |
| `WithHooks(hooks...)` | Register post-write callback functions |
| `WithEventID()` | Generate unique `event_id` per entry |
| `WithFullCallerPath()` | Include full file path instead of basename in `file` field |

## Writers

### Async Buffered Writer

Non-blocking writes via a channel. If the buffer is full, messages are dropped (lossy) to ensure the application is never stalled by log I/O.

```go
async := logger.NewAsyncWriter(os.Stdout, 4096) // buffer size
log := logger.New(async, logger.INFO)
defer log.Sync() // flush before exit

log.Info("fast", nil) // non-blocking
async.DroppedCount()  // monitor drops
```

### Fan-Out Writer

Broadcast every entry to multiple destinations.

```go
file, _ := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
fan := logger.NewFanOutWriter(os.Stdout, file)
log := logger.New(fan, logger.INFO)
```

### Composing Writers

```go
fan := logger.NewFanOutWriter(os.Stdout, file)
async := logger.NewAsyncWriter(fan, 8192)
log := logger.New(async, logger.INFO,
    logger.WithServiceIdentity("api", "2.0.0", "prod"),
)
defer log.Sync() // drains async -> syncs fan-out -> fsyncs file
```

## Formatters

### JSON (default)

```json
{"env":"prod","file":"main.go:42","host":"server-1","level":"INFO","message":"request","method":"GET","service":"api","status":200,"time":"2026-04-12T14:32:07.123Z","version":"2.0.0"}
```

### Text (for development)

```
2026-04-12T14:32:07Z INFO    [main.go:42] request  method=GET status=200
```

```go
log := logger.New(os.Stdout, logger.DEBUG,
    logger.WithFormatter(logger.TextFormatter{NoColor: false}),
)
```

Set `NoColor: true` to disable ANSI color codes (e.g., for file output).

## Sampling

Prevent log storms from overwhelming sinks. ERROR and FATAL are **never** sampled.

```go
// Log every 10th entry per level
log := logger.New(os.Stdout, logger.DEBUG,
    logger.WithSampler(logger.NewEveryNSampler(10)),
)

// Or: max 100 entries per second per level
log := logger.New(os.Stdout, logger.DEBUG,
    logger.WithSampler(logger.NewRateSampler(100)),
)
```

## slog Integration

Use as a backend for Go's standard `log/slog`:

```go
log := logger.New(os.Stdout, logger.DEBUG)
slogLogger := slog.New(logger.NewSlogHandler(log))

slogLogger.Info("from slog", "user", "alice", "count", 42)
// Routed through log-it with full feature support (hooks, formatting, etc.)
```

## Timing Helpers

```go
done := log.Timed("db_query")
defer done()
// Logs: {"level":"INFO","message":"db_query","duration_ms":12.345,...}
```

With context extraction:

```go
done := log.TimedContext(ctx, "http_request")
defer done()
```

## Error Enrichment

When a field value implements the `error` interface, the logger automatically adds:
- `<key>`: the error message string
- `<key>_type`: the concrete error type (e.g., `*os.PathError`)
- `<key>_chain`: the unwrap chain (only if the error is wrapped)

```go
err := fmt.Errorf("query failed: %w", sql.ErrNoRows)
log.Error("db error", map[string]any{"err": err})
// Output includes: "err":"query failed: no rows", "err_type":"*fmt.wrapError", "err_chain":["query failed: no rows","no rows"]
```

## Hooks

Hooks run after each entry is written. They receive the level, message, and merged fields. Each hook is panic-safe.

```go
alertHook := func(level logger.Level, msg string, fields map[string]any) {
    if level >= logger.ERROR {
        // send alert, increment counter, etc.
    }
}

log := logger.New(os.Stdout, logger.INFO,
    logger.WithHooks(alertHook),
)
```

## Runtime Level Changes

```go
log.SetLevel(logger.DEBUG)   // enable debug logging
log.GetLevel()               // read current level
```

Level changes are atomic and propagate to all child loggers.

## Graceful Shutdown

Always call `Sync()` before application exit:

```go
log := logger.New(asyncWriter, logger.INFO)
defer log.Sync()
```

`Sync()` flushes the async buffer, syncs fan-out writers, and prevents further entries from being accepted.

## Testing

138 tests covering all public API, error paths, concurrency, and feature combinations. All passing with `-race`:

```bash
go test -race -v -count=1 ./...
```
