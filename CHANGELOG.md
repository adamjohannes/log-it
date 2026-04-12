# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/adamjohannes/log-it/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/adamjohannes/log-it/releases/tag/v0.1.0
