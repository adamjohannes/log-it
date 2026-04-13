package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Benchmarks — run with: go test -bench=. -benchmem ./...
// =============================================================================

func BenchmarkInfoSync(b *testing.B) {
	l := New(io.Discard, INFO)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("request handled", map[string]any{"status": 200})
	}
}

func BenchmarkInfoAsync(b *testing.B) {
	aw := NewAsyncWriter(io.Discard, 8192)
	l := New(aw, INFO)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("request handled", map[string]any{"status": 200})
	}
	b.StopTimer()
	_ = l.Sync()
}

func BenchmarkInfow(b *testing.B) {
	l := New(io.Discard, INFO)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Infow("request handled", Int("status", 200))
	}
}

func BenchmarkInfoWithFields(b *testing.B) {
	l := New(io.Discard, INFO)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("request", map[string]any{
			"method":  "GET",
			"path":    "/api/users",
			"status":  200,
			"latency": 12.5,
			"user_id": 42,
		})
	}
}

func BenchmarkInfowWithFields(b *testing.B) {
	l := New(io.Discard, INFO)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Infow("request",
			String("method", "GET"),
			String("path", "/api/users"),
			Int("status", 200),
			Float64("latency", 12.5),
			Int("user_id", 42),
		)
	}
}

func BenchmarkChildLogger(b *testing.B) {
	l := New(io.Discard, INFO)
	child := l.With(map[string]any{"service": "api", "version": "1.0.0"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		child.Info("request", map[string]any{"status": 200})
	}
}

func BenchmarkContextExtractor(b *testing.B) {
	type ctxKey string
	l := New(io.Discard, INFO)
	child := l.WithContextExtractor(func(ctx context.Context) map[string]any {
		if v := ctx.Value(ctxKey("rid")); v != nil {
			return map[string]any{"request_id": v}
		}
		return nil
	})
	ctx := context.WithValue(context.Background(), ctxKey("rid"), "req-123")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		child.InfoContext(ctx, "request", nil)
	}
}

func BenchmarkFormatterJSON(b *testing.B) {
	f := JSONFormatter{}
	entry := map[string]any{
		"time":    "2026-04-12T14:32:07.123456789Z",
		"level":   "INFO",
		"message": "request handled",
		"file":    "main.go:42",
		"status":  200,
		"method":  "GET",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(entry)
	}
}

func BenchmarkFormatterText(b *testing.B) {
	f := TextFormatter{NoColor: true}
	entry := map[string]any{
		"time":    "2026-04-12T14:32:07.123456789Z",
		"level":   "INFO",
		"message": "request handled",
		"file":    "main.go:42",
		"status":  200,
		"method":  "GET",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(entry)
	}
}

func BenchmarkParallel(b *testing.B) {
	l := New(io.Discard, INFO)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info("parallel", map[string]any{"goroutine": true})
		}
	})
}

func BenchmarkLevelFiltering(b *testing.B) {
	l := New(io.Discard, WARNING) // DEBUG and INFO are filtered out
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Debug("should be dropped", nil) // should be near-zero cost
	}
}

// =============================================================================
// Stress Tests — run with: go test -run TestStress -count=1 -race -v ./...
// Skipped during normal `go test -short`
// =============================================================================

func TestStressAsyncDropBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Tiny buffer to force drops
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 10)
	l := New(aw, INFO)

	const total = 100_000
	for i := 0; i < total; i++ {
		l.Info("flood", map[string]any{"i": i})
	}
	_ = l.Sync()

	written := countJSONEntries(t, &buf)
	dropped := aw.DroppedCount()

	t.Logf("total=%d written=%d dropped=%d", total, written, dropped)

	if int64(written)+dropped != int64(total) {
		t.Errorf("written(%d) + dropped(%d) != total(%d)", written, dropped, total)
	}
	if dropped == 0 {
		t.Error("expected drops with buffer size 10 and 100k entries")
	}
}

func TestStressConcurrentWriters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var buf bytes.Buffer
	l := New(&buf, INFO)

	const goroutines = 100
	const perGoroutine = 10_000

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				l.Info("concurrent", map[string]any{"g": id, "i": i})
			}
		}(g)
	}
	wg.Wait()

	written := countJSONEntries(t, &buf)
	expected := goroutines * perGoroutine

	t.Logf("expected=%d written=%d", expected, written)

	if written != expected {
		t.Errorf("expected %d entries, got %d (lost %d)", expected, written, expected-written)
	}
}

func TestStressAsyncConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 4096)
	l := New(aw, INFO)

	const goroutines = 100
	const perGoroutine = 10_000

	start := time.Now()

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				l.Info("async-concurrent", map[string]any{"g": id, "i": i})
			}
		}(g)
	}
	wg.Wait()
	_ = l.Sync()

	elapsed := time.Since(start)
	written := countJSONEntries(t, &buf)
	dropped := aw.DroppedCount()
	total := goroutines * perGoroutine

	t.Logf("total=%d written=%d dropped=%d elapsed=%v entries/sec=%.0f",
		total, written, dropped, elapsed, float64(total)/elapsed.Seconds())

	if int64(written)+dropped != int64(total) {
		t.Errorf("written(%d) + dropped(%d) != total(%d)", written, dropped, total)
	}
}

func TestStressChildLoggerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	l := New(io.Discard, INFO)

	const goroutines = 100
	const perGoroutine = 100

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				child := l.With(map[string]any{"g": id, "i": i})
				child.Info("from-child", nil)
			}
		}(g)
	}
	wg.Wait()

	t.Logf("created and logged from %d child loggers concurrently", goroutines*perGoroutine)
}

func TestStressSamplerUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var buf bytes.Buffer
	sampler := NewEveryNSampler(100)
	l := New(&buf, INFO, WithSampler(sampler))

	const goroutines = 100
	const perGoroutine = 10_000
	total := goroutines * perGoroutine

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				l.Info("sampled", nil)
			}
		}()
	}
	wg.Wait()

	written := countJSONEntries(t, &buf)
	expected := total / 100 // EveryN(100) passes 1 in 100

	t.Logf("total=%d written=%d expected=~%d", total, written, expected)

	// Allow 1% tolerance due to concurrent counter increments
	if written < expected-expected/100 || written > expected+expected/100 {
		t.Errorf("expected ~%d sampled entries, got %d", expected, written)
	}
}

func TestStressAllFeaturesEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var hookCount atomic.Int64
	hook := func(_ Level, _ string, _ map[string]any) {
		hookCount.Add(1)
	}

	var buf1, buf2 bytes.Buffer
	fan := NewFanOutWriter(&buf1, &buf2)
	aw := NewAsyncWriter(fan, 8192)
	l := New(aw, INFO,
		WithServiceIdentity("stress", "0.1.0", "test"),
		WithEventID(),
		WithFullCallerPath(),
		WithHooks(hook),
		WithSampler(NewEveryNSampler(10)),
	)

	child := l.With(map[string]any{"component": "stress"})
	child = child.WithContextExtractor(func(ctx context.Context) map[string]any {
		return map[string]any{"request_id": "req-stress"}
	})

	const goroutines = 50
	const perGoroutine = 1_000

	start := time.Now()

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				child.InfoContext(context.Background(), "all-features", map[string]any{
					"g": id, "i": i,
				})
			}
		}(g)
	}
	wg.Wait()
	_ = l.Sync()

	elapsed := time.Since(start)
	total := goroutines * perGoroutine
	written1 := countJSONEntries(t, &buf1)
	written2 := countJSONEntries(t, &buf2)
	dropped := aw.DroppedCount()

	t.Logf("total=%d written_buf1=%d written_buf2=%d dropped=%d hooks=%d elapsed=%v",
		total, written1, written2, dropped, hookCount.Load(), elapsed)

	// Both fan-out destinations should have the same count
	if written1 != written2 {
		t.Errorf("fan-out mismatch: buf1=%d buf2=%d", written1, written2)
	}

	// written + dropped should equal sampled total (total/10 for EveryN(10))
	sampledTotal := total / 10
	if int64(written1)+dropped != int64(sampledTotal) {
		// Allow small tolerance due to concurrent sampling
		diff := int64(written1) + dropped - int64(sampledTotal)
		if diff < -10 || diff > 10 {
			t.Errorf("written(%d) + dropped(%d) != sampled_total(%d), diff=%d",
				written1, dropped, sampledTotal, diff)
		}
	}

	// Hooks should have been called for every written entry
	if hookCount.Load() != int64(written1) {
		t.Errorf("hook count (%d) != written count (%d)", hookCount.Load(), written1)
	}
}

func TestStressMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	l := New(io.Discard, INFO)

	// Force GC and measure baseline
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	const entries = 1_000_000
	for i := 0; i < entries; i++ {
		l.Info("memory test", map[string]any{"i": i})
	}

	// Force GC and measure after
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	heapDelta := int64(after.HeapInuse) - int64(before.HeapInuse)
	allocsDelta := after.TotalAlloc - before.TotalAlloc

	t.Logf("entries=%d heap_delta=%s total_alloc=%s alloc_per_entry=%d bytes",
		entries,
		formatBytes(heapDelta),
		formatBytes(int64(allocsDelta)),
		allocsDelta/entries,
	)

	// Heap should not grow unboundedly — if writing to Discard, heap delta
	// should be minimal after GC (no retained references).
	// Allow up to 10MB of heap growth for 1M entries (generous).
	if heapDelta > 10*1024*1024 {
		t.Errorf("heap grew by %s after 1M entries — possible memory leak", formatBytes(heapDelta))
	}
}

// =============================================================================
// Helpers
// =============================================================================

// countJSONEntries counts valid newline-delimited JSON objects in buf.
func countJSONEntries(t *testing.T, buf *bytes.Buffer) int {
	t.Helper()
	count := 0
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	for dec.More() {
		var entry map[string]any
		if err := dec.Decode(&entry); err != nil {
			t.Fatalf("invalid JSON at entry %d: %v", count, err)
		}
		count++
	}
	return count
}

func formatBytes(b int64) string {
	if b < 0 {
		return fmt.Sprintf("-%s", formatBytes(-b))
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMG"[exp])
}
