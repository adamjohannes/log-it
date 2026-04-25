# log-it

A structured, leveled JSON logger for Go. Pure stdlib, zero dependencies.

```go
import logger "github.com/adamjohannes/log-it"
```

## Install

```bash
go get github.com/adamjohannes/log-it
```

Requires Go 1.21+.

Full API documentation: [pkg.go.dev/github.com/adamjohannes/log-it](https://pkg.go.dev/github.com/adamjohannes/log-it)

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
	// {"level":"INFO","message":"server started","port":8080,"time":"2026-04-12T..."}
}
```

## Features

**Logging** — 6 levels (`TRACE` through `FATAL`), 4 styles (map, printf, context-aware, typed fields), child loggers with persistent fields, context extraction for trace/request IDs

**Output** — 3 formatters (JSON, text with ANSI colors, logfmt), auto-format terminal detection, cloud key remapping (`GCPKeyMap`, `DatadogKeyMap`, `ELKKeyMap`), env variable config (`LOG_LEVEL`, `LOG_FORMAT`)

**Writers** — async buffered (non-blocking, lossy back-pressure), fan-out (multi-destination), filtered (per-sink level routing), fallback (resilient writes), composable

**Pipeline** — pre-write middleware, post-write hooks, field redaction, sampling/rate limiting, error enrichment (`_type`, `_chain`)

**Integrations & Utilities** — `slog.Handler` adapter, `*log.Logger` bridge, `Interface` for DI, timing helpers, event IDs, stack traces, caller info, nop logger, `logtest` subpackage, graceful shutdown, context-based logger passing, global default, write error monitoring

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
    logger.Group("user",
        logger.String("id", "u-42"),
        logger.String("role", "admin"),
    ),
)
```

Available constructors: `String`, `Int`, `Int64`, `Float64`, `Bool`, `Err`, `Duration`, `Any`, `Group`.

## Child Loggers & Context

```go
// Persistent fields attached to every entry
reqLog := log.With(map[string]any{
    "request_id": "abc-123",
    "user_id":    42,
})
reqLog.Info("processing", nil)
```

Extract fields from `context.Context` automatically:

```go
log = log.WithContextExtractor(func(ctx context.Context) map[string]any {
    if traceID := ctx.Value("trace_id"); traceID != nil {
        return map[string]any{"trace_id": traceID}
    }
    return nil
})

log.InfoContext(ctx, "handled", nil)
// trace_id auto-extracted from context
```

Store and retrieve loggers via context with `WithLogger()` / `FromContext()`.

## Configuration Options

```go
log := logger.New(os.Stdout, logger.INFO,
    logger.WithServiceIdentity("myapp", "1.2.3", "prod"),
    logger.WithFormatter(logger.TextFormatter{NoColor: false}),
    logger.WithSampler(logger.NewRateSampler(100)),
    logger.WithHooks(myAlertHook),
    logger.WithCaller(),
    logger.WithEventID(),
)
```

| Option | Description |
|--------|-------------|
| `WithServiceIdentity(service, version, env)` | Attach `service`, `version`, `env`, `host` to every entry |
| `WithFormatter(f)` | Set output format (`JSONFormatter{}`, `TextFormatter{}`, or `LogfmtFormatter{}`) |
| `WithSampler(s)` | Enable sampling (`NewEveryNSampler(n)` or `NewRateSampler(n)`) |
| `WithHooks(hooks...)` | Register post-write callback functions |
| `WithMiddleware(mw...)` | Register pre-write middleware for entry transformation or filtering |
| `WithCaller()` | Include `file` field with caller file:line (off by default for performance) |
| `WithFullCallerPath()` | Full file path instead of basename; implies `WithCaller()` |
| `WithStackTrace()` | Capture stack traces for ERROR/FATAL entries |
| `WithEventID()` | Generate unique `event_id` per entry |
| `WithFallbackWriter(w)` | Try fallback writer when primary fails |
| `WithRedactFields(fields...)` | Replace named field values with `[REDACTED]` |
| `WithRedactFieldsFunc(replacement, fields...)` | Same with custom replacement string |
| `WithAutoFormat()` | Auto-select JSON or colored text based on terminal detection |
| `WithEnvConfig()` | Read `LOG_LEVEL` and `LOG_FORMAT` from environment variables |

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

### Filtered Writer

Route different log levels to different destinations. Use with `FanOutWriter` for per-sink filtering.

```go
fan := logger.NewFanOutWriter(
    logger.NewFilteredWriter(os.Stdout, logger.INFO),     // INFO+ to stdout
    logger.NewFilteredWriter(errorFile, logger.ERROR),     // ERROR+ to file
)
log := logger.New(fan, logger.DEBUG)
```

When using a `KeyMap` that renames the `"level"` key, pass `WithLevelKey`:

```go
logger.NewFilteredWriter(w, logger.ERROR, logger.WithLevelKey("severity"))
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

Use `WithFallbackWriter(os.Stderr)` for resilient logging when the primary sink fails.

## Formatters

### JSON (default)

```json
{"level":"INFO","message":"request","method":"GET","status":200,"time":"2026-04-12T14:32:07.123Z"}
```

Remap field names for cloud platforms with built-in presets:

```go
logger.WithFormatter(logger.JSONFormatter{KeyMap: logger.GCPKeyMap})     // "level" -> "severity"
logger.WithFormatter(logger.JSONFormatter{KeyMap: logger.DatadogKeyMap}) // "level" -> "status"
logger.WithFormatter(logger.JSONFormatter{KeyMap: logger.ELKKeyMap})     // "time" -> "@timestamp"
```

All formatters (`JSONFormatter`, `TextFormatter`, `LogfmtFormatter`) support `KeyMap`.

### Text (for development)

```
2026-04-12T14:32:07Z INFO    request  method=GET status=200
```

```go
log := logger.New(os.Stdout, logger.DEBUG,
    logger.WithFormatter(logger.TextFormatter{NoColor: false}),
)
```

Set `NoColor: true` to disable ANSI color codes. ANSI escape sequences in field values are automatically stripped.

### Logfmt (for Loki)

```
time=2026-04-12T14:32:07Z level=INFO message="request handled" method=GET status=200
```

```go
log := logger.New(os.Stdout, logger.INFO,
    logger.WithFormatter(logger.LogfmtFormatter{}),
)
```

### Auto-Format Detection

Automatically selects colored text for terminals, JSON otherwise:

```go
log := logger.New(os.Stderr, logger.INFO, logger.WithAutoFormat())
```

Works through writer wrappers — they implement `Unwrap()` so terminal detection sees through to the underlying `*os.File`.

## Middleware & Hooks

Middleware runs before writing. Return `nil` to drop the entry.

```go
dropHealth := func(entry map[string]any) map[string]any {
    if msg, _ := entry["message"].(string); msg == "health check" {
        return nil
    }
    return entry
}

log := logger.New(os.Stdout, logger.INFO,
    logger.WithMiddleware(dropHealth),
)
```

Hooks run after writing. They receive the level, message, and fields. Each hook is panic-safe.

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

## Filtering & Redaction

Redact sensitive fields by name. Applies recursively to nested maps.

```go
log := logger.New(os.Stdout, logger.INFO,
    logger.WithRedactFields("password", "token", "secret"),
)
log.Info("login", map[string]any{"user": "alice", "password": "s3cret"})
// Output: "password":"[REDACTED]", "user":"alice"
```

For type-level control, implement `slog.LogValuer` on your types.

Prevent log storms with sampling. ERROR and FATAL are **never** sampled.

```go
logger.WithSampler(logger.NewEveryNSampler(10))  // every 10th entry per level
logger.WithSampler(logger.NewRateSampler(100))    // max 100/sec per level
```

## slog & Stdlib Integration

Use as a backend for Go's standard `log/slog`:

```go
log := logger.New(os.Stdout, logger.DEBUG)
slogLogger := slog.New(logger.NewSlogHandler(log))

slogLogger.Info("from slog", "user", "alice", "count", 42)
// Routed through log-it with full feature support
```

`SetDefault()` automatically syncs with `slog.SetDefault()`. Bridge to `*log.Logger` with `StdLogger()`:

```go
httpServer.ErrorLog = log.StdLogger(logger.WARNING)
```

## Test Helpers

The `logtest` subpackage provides helpers for asserting on log output in tests:

```go
import "github.com/adamjohannes/log-it/logtest"

func TestOrderProcessing(t *testing.T) {
    log, handler := logtest.NewTestLogger(t)
    svc := NewOrderService(log)

    svc.Process(ctx, order)

    logtest.AssertLogged(t, handler, "INFO", "order processed")
    logtest.AssertNotLogged(t, handler, "ERROR", "")
}
```

`NewTLogger(t)` routes logs through `t.Log()` so they appear only with `-v`.

## Performance

Level filtering costs ~2ns with zero allocations. A full JSON entry to `io.Discard` runs ~1.9μs. Parallel throughput reaches ~1.8M entries/sec on 12 cores.

See [BENCHMARKS.md](BENCHMARKS.md) for full results and stress tests.

## Testing

```bash
go test -race -v -count=1 ./...
```

A pre-flight script mirrors the full CI pipeline locally:

```bash
./scripts/pre-flight.sh          # full run
./scripts/pre-flight.sh --quick  # skip stress tests and lint
```

## Full API Reference

Complete documentation for all types, methods, and runnable examples:
[pkg.go.dev/github.com/adamjohannes/log-it](https://pkg.go.dev/github.com/adamjohannes/log-it)
