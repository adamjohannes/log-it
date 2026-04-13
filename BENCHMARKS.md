# Benchmarks & Stress Tests

This document describes the performance benchmarks and stress tests for log-it, how to run them, and reference results.

## Running

```bash
# Benchmarks (fast, measures per-operation cost)
go test -bench=. -benchmem -count=3 -run='^$' ./...

# Stress tests (longer, pushes limits)
go test -run TestStress -count=1 -race -v ./...

# Regular tests only (stress tests skipped)
go test -short -race -count=1 ./...
```

## Benchmarks

Standard Go benchmarks measuring per-operation cost and allocations.

| Benchmark | What it measures |
|-----------|-----------------|
| `BenchmarkInfoSync` | Synchronous `Info()` to `io.Discard` — baseline throughput |
| `BenchmarkInfoAsync` | `Info()` through `AsyncWriter` — non-blocking path overhead |
| `BenchmarkInfow` | Typed field `Infow()` — allocation comparison vs map-based |
| `BenchmarkInfoWithFields` | `Info()` with 5 map fields — map allocation cost |
| `BenchmarkInfowWithFields` | `Infow()` with 5 typed fields — typed allocation cost |
| `BenchmarkChildLogger` | Child logger with persistent fields — field merge overhead |
| `BenchmarkContextExtractor` | `InfoContext()` with extractor — extraction overhead |
| `BenchmarkFormatterJSON` | `JSONFormatter.Format()` in isolation |
| `BenchmarkFormatterText` | `TextFormatter.Format()` in isolation |
| `BenchmarkParallel` | `b.RunParallel` — concurrent throughput under mutex contention |
| `BenchmarkLevelFiltering` | Logging below min level — measures early-exit cost (should be ~0) |

### Reference Results

Measured on Apple M3 Pro, Go 1.24, 12 cores:

```
BenchmarkInfoSync-12              574474    1896 ns/op    1545 B/op    25 allocs/op
BenchmarkInfoAsync-12             626748    1968 ns/op    1674 B/op    26 allocs/op
BenchmarkInfow-12                 624187    1942 ns/op    1545 B/op    25 allocs/op
BenchmarkInfoWithFields-12        416515    2947 ns/op    2226 B/op    35 allocs/op
BenchmarkInfowWithFields-12       425551    2815 ns/op    2266 B/op    38 allocs/op
BenchmarkChildLogger-12           481339    2505 ns/op    2379 B/op    33 allocs/op
BenchmarkContextExtractor-12      702357    1716 ns/op    1545 B/op    25 allocs/op
BenchmarkFormatterJSON-12        1362030     887 ns/op     576 B/op    14 allocs/op
BenchmarkFormatterText-12        1587921     757 ns/op     432 B/op    11 allocs/op
BenchmarkParallel-12             1815426     677 ns/op    1556 B/op    25 allocs/op
BenchmarkLevelFiltering-12     669924172    1.79 ns/op       0 B/op     0 allocs/op
```

### Key Takeaways

- **Baseline throughput**: ~527K entries/sec (sync), ~1.8M entries/sec (parallel across 12 cores)
- **Level filtering is free**: 1.79 ns/op, zero allocations — disabled levels cost nothing
- **Async overhead is minimal**: +72 ns/op over sync (channel send + copy)
- **Typed fields (Infow) match map-based (Info)**: same allocation count for single field; slight overhead with many fields due to `fieldsToMap` conversion
- **TextFormatter is faster than JSON**: 757 ns vs 887 ns (no reflection-heavy marshal)
- **Context extraction adds no overhead**: extractors are only called when present, and the result is a simple map merge

## Stress Tests

Longer-running tests that push the logger to its limits. Skipped during normal `go test -short`. All tests run with `-race` to detect data races.

### TestStressAsyncDropBehavior

Floods an `AsyncWriter` with buffer size 10 with 100,000 entries. Verifies:
- Drops are counted correctly (`written + dropped == total`)
- No panics or deadlocks under back-pressure
- The lossy strategy works as designed

**Reference**: 97,775 written, 2,225 dropped.

### TestStressConcurrentWriters

100 goroutines each writing 10,000 entries through a synchronous writer (mutex-protected). Verifies:
- All 1,000,000 entries are written (zero loss)
- Every entry is valid JSON (no corruption from interleaved writes)
- No data races

**Reference**: 1,000,000/1,000,000 written. Zero loss.

### TestStressAsyncConcurrent

100 goroutines each writing 10,000 entries through `AsyncWriter` (buffer=4096). Verifies:
- `written + dropped == total`
- No deadlocks under high concurrency
- Reports throughput (entries/sec)

**Reference**: 956,854 written, 43,146 dropped. 175K entries/sec.

### TestStressChildLoggerCreation

100 goroutines each creating 100 child loggers and logging from each. Verifies:
- No data races during concurrent `With()` + logging
- Child logger field isolation under concurrency

**Reference**: 10,000 child loggers created and used concurrently.

### TestStressSamplerUnderLoad

100 goroutines producing 1,000,000 total entries with `EveryNSampler(100)`. Verifies:
- Exactly 10,000 entries pass through (1 in 100)
- Atomic counter accuracy under high contention

**Reference**: 10,000/10,000 expected. Perfect accuracy.

### TestStressAllFeaturesEnabled

All options enabled simultaneously: `AsyncWriter` + `FanOutWriter` + `WithServiceIdentity` + `WithEventID` + `WithFullCallerPath` + `WithHooks` + `WithSampler` + `WithContextExtractor`. 50 goroutines x 1,000 entries. Verifies:
- Fan-out destinations receive identical counts
- `written + dropped == sampled_total`
- Hook count matches written count
- No crashes or races with maximum feature density

**Reference**: 5,000 written to both destinations, 0 dropped, 5,000 hook calls. 76ms elapsed.

### TestStressMemoryStability

Logs 1,000,000 entries to `io.Discard`, measures heap before/after with forced GC. Verifies:
- No memory leaks (heap should return to near-baseline after GC)
- Reports per-entry allocation cost

**Reference**: 64 KiB heap delta after 1M entries (no leak). 1,774 bytes total allocated per entry.
