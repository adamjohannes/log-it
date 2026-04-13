# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Breaking

- **Caller capture is now opt-in.** The `"file"` field is no longer included in log entries by default. Use `WithCaller()` to re-enable it, or `WithFullCallerPath()` which implies `WithCaller()`. This saves ~300–600ns per log call on the hot path.

### Added

- `WithCaller()` option to opt in to caller file:line capture
- `WithStackTrace()` option for stack trace capture on ERROR/FATAL entries
- `WithFallbackWriter(w)` option for resilient logging when the primary sink fails
- `WithMiddleware(mw...)` option for pre-write entry transformation and filtering
- `WithRedactFields(fields...)` and `WithRedactFieldsFunc(replacement, fields...)` for field-name-based PII redaction
- `WithAutoFormat()` option for terminal-aware formatter selection (colored text for TTYs, JSON otherwise)
- `LogfmtFormatter` for Grafana Loki / logfmt-compatible output; recognized by `WithEnvConfig()` via `LOG_FORMAT=logfmt`
- `FilteredWriter` for per-sink level filtering; composes with `FanOutWriter`
- `WithLevelKey(key)` option on `FilteredWriter` for compatibility with KeyMap remapping
- `Interface` type in `iface.go` for dependency injection and test doubles; `*Logger` satisfies it implicitly
- `logtest` subpackage with `TestHandler`, `NewTestLogger(t)`, `NewTLogger(t)`, `AssertLogged`, `AssertNotLogged`
- `DatadogKeyMap` and `ELKKeyMap` cloud provider presets alongside existing `GCPKeyMap`
- `Unwrap()` method on `AsyncWriter`, `FanOutWriter`, and `FilteredWriter` for terminal detection through wrapper layers
- `Middleware` type for pre-write entry interception
- 7 new `Example*` functions (LogfmtFormatter, FilteredWriter, WithRedactFields, WithMiddleware, WithFallbackWriter, WithStackTrace, OTel trace context)
- `SetDefault()` and `ReplaceDefault()` now automatically call `slog.SetDefault()` for seamless slog interop

### Changed

- `JSONFormatter` uses a hand-rolled encoder (`strconv.Append*`, sorted keys) instead of `encoding/json.Marshal` — significantly faster with fewer allocations
- `TextFormatter` and `writeEntry` use `sync.Pool` buffer pooling to reduce allocations
- `TextFormatter` now strips ANSI escape sequences from field values and messages (security hardening)
- `WithFullCallerPath()` now implies `WithCaller()` — no separate call needed
- Unmarshalable field values (e.g., `func()`) are now encoded as their string representation instead of causing a fallback error entry

### Fixed

- `Sync()` now ignores known benign errors ("invalid argument", "inappropriate ioctl for device") from non-file descriptors
- `FilteredWriter` works correctly with KeyMap remapping via `WithLevelKey`
- `WithAutoFormat()` detects terminals through wrapped writers (`AsyncWriter`, `FanOutWriter`, `FilteredWriter`)

## [0.2.0] - 2026-04-12

### Added
- `FromContext()` / `WithLogger()` for storing and retrieving loggers from `context.Context`
- `Default()` / `SetDefault()` / `ReplaceDefault()` global default logger with lazy initialization
- `FromContext()` falls back to `Default()` when no logger is in the context
- `StdLogger(level)` method for bridging to stdlib's `*log.Logger`
- `WriteErrorCount()` for monitoring failed writes to the output sink
- `Group()` typed field constructor for nested JSON structures
- `KeyMap` field on `JSONFormatter` and `TextFormatter` for remapping core field names
- `GCPKeyMap` preset for Google Cloud Logging compatibility (`level`→`severity`, `message`→`textPayload`)
- `WithEnvConfig()` option to read `LOG_LEVEL` and `LOG_FORMAT` from environment variables
- `SyncWithTimeout(d)` for deadline-aware flush when sinks may be slow or unreachable
- `slog.LogValuer` support — field values implementing `LogValuer` are resolved before serialization, enabling PII redaction and lazy evaluation
- `Nop()` logger that discards all output, safe for tests and defaults
- Testable `Example` functions for pkg.go.dev documentation (14 examples)
- Benchmarks (11) and stress tests (7) with `BENCHMARKS.md` documentation

### Fixed
- `Fatal` now calls `Sync` on the underlying writer before exiting, preventing loss of async-buffered logs
- `TextFormatter` escapes `\n` and `\r` in messages and field values, preventing log injection
- Lint compliance with golangci-lint v2 (errcheck on `os.Setenv`, `os.Remove`, `Sync`)

## [0.1.0] - 2026-04-12

### Added
- Structured, leveled JSON logger with 6 levels: `TRACE`, `DEBUG`, `INFO`, `WARNING`, `ERROR`, `FATAL`
- 4 logging styles: structured (map), formatted (printf), context-aware, typed fields
- Flat JSON output with reserved key collision handling (`"fields."` prefix)
- Child loggers via `With()` with persistent field merging
- Context propagation via `WithContextExtractor()` for automatic trace/request ID injection
- Service identity metadata via `WithServiceIdentity()` (service, version, env, host)
- `FanOutWriter` for broadcasting to multiple destinations simultaneously
- `AsyncWriter` with non-blocking lossy channel and `DroppedCount()` monitoring
- Graceful shutdown via `Sync()` with closed flag to prevent post-shutdown writes
- Pluggable `Formatter` interface with `JSONFormatter` (default) and `TextFormatter` (ANSI colors)
- Hooks/interceptors via `WithHooks()` with per-hook panic recovery
- Automatic error enrichment: `_type` and `_chain` fields for error interface values
- `slog.Handler` adapter via `NewSlogHandler()` for `log/slog` ecosystem integration
- Typed field constructors (`String`, `Int`, `Int64`, `Float64`, `Bool`, `Err`, `Duration`, `Any`) with `*w` methods
- Sampling via `NewEveryNSampler()` and `NewRateSampler()` — ERROR/FATAL never sampled
- `//go:noinline` on internal methods to prevent `runtime.Caller` skip breakage
- `WithEventID()` for unique per-entry IDs (timestamp + atomic counter)
- `WithFullCallerPath()` to include package directory in file field
- `Timed()` and `TimedContext()` helpers for defer-friendly duration logging
- Injectable `exitFunc` for Fatal testability
- Panic recovery for context extractors (matching hooks pattern)
- 138 tests, all passing with `-race`

### Fixed
- JSON injection in marshal-failure fallback path (now uses `json.Marshal`)
- Portuguese fallback error message replaced with English

[Unreleased]: https://github.com/adamjohannes/log-it/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/adamjohannes/log-it/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/adamjohannes/log-it/releases/tag/v0.1.0
