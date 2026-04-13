package logger

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
)

func TestEveryNSamplerBasic(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithSampler(NewEveryNSampler(5)))

	for i := 0; i < 10; i++ {
		l.Info("msg", map[string]any{"i": i})
	}

	entries := decodeAllEntriesSampler(t, &buf)
	// Should pass 1st and 6th (indices 0 and 5)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries with EveryN(5) over 10, got %d", len(entries))
	}
}

func TestEveryNSamplerPreservesErrorAndFatal(t *testing.T) {
	var buf bytes.Buffer
	var exitCode int
	l := New(&buf, DEBUG, WithSampler(NewEveryNSampler(100)), withExitFunc(noopExit(&exitCode)))

	// These should ALL pass through despite aggressive sampling
	for i := 0; i < 5; i++ {
		l.Error("err", map[string]any{"i": i})
	}
	l.Fatal("fatal", nil)

	entries := decodeAllEntriesSampler(t, &buf)
	if len(entries) != 6 {
		t.Errorf("expected 6 entries (5 ERROR + 1 FATAL), got %d", len(entries))
	}
}

func TestRateSamplerBasic(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithSampler(NewRateSampler(2)))

	// All within same second
	for i := 0; i < 5; i++ {
		l.Info("msg", map[string]any{"i": i})
	}

	entries := decodeAllEntriesSampler(t, &buf)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries with RateSampler(2), got %d", len(entries))
	}
}

func TestRateSamplerPreservesErrorAndFatal(t *testing.T) {
	var buf bytes.Buffer
	var exitCode int
	l := New(&buf, DEBUG, WithSampler(NewRateSampler(1)), withExitFunc(noopExit(&exitCode)))

	// Log many errors — all should pass
	for i := 0; i < 5; i++ {
		l.Error("err", nil)
	}
	l.Fatal("fatal", nil)

	entries := decodeAllEntriesSampler(t, &buf)
	if len(entries) != 6 {
		t.Errorf("expected 6 entries (all ERROR/FATAL pass), got %d", len(entries))
	}
}

func TestNoSamplerAllPass(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG) // no sampler

	for i := 0; i < 10; i++ {
		l.Info("msg", nil)
	}

	entries := decodeAllEntriesSampler(t, &buf)
	if len(entries) != 10 {
		t.Errorf("expected all 10 without sampler, got %d", len(entries))
	}
}

func TestSamplerOnChildLogger(t *testing.T) {
	var buf bytes.Buffer
	root := New(&buf, DEBUG, WithSampler(NewEveryNSampler(3)))
	child := root.With(map[string]any{"child": true})

	for i := 0; i < 9; i++ {
		child.Info("msg", nil)
	}

	entries := decodeAllEntriesSampler(t, &buf)
	// Every 3rd: 1st, 4th, 7th = 3 entries
	if len(entries) != 3 {
		t.Errorf("expected 3 entries from child with EveryN(3) over 9, got %d", len(entries))
	}
}

func TestSamplerContextMethodsRespected(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithSampler(NewEveryNSampler(5)))

	for i := 0; i < 10; i++ {
		l.InfoContext(nil, "ctx-msg", nil) //nolint:staticcheck // deliberately testing nil context
	}

	entries := decodeAllEntriesSampler(t, &buf)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries via context method with EveryN(5), got %d", len(entries))
	}
}

func TestEveryNSamplerConcurrency(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, DEBUG, WithSampler(NewEveryNSampler(10)))

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				l.Info("concurrent", nil)
			}
		}()
	}
	wg.Wait()

	entries := decodeAllEntriesSampler(t, &buf)
	// 1000 total, every 10th = 100 expected
	if len(entries) != 100 {
		t.Errorf("expected 100 sampled entries, got %d", len(entries))
	}
}

func TestEveryNSamplerDefaultN(t *testing.T) {
	s := NewEveryNSampler(0) // should default to 1
	// Every call should pass
	for i := 0; i < 5; i++ {
		if !s.Sample(INFO, "msg") {
			t.Errorf("expected Sample to return true with n=0 (defaulting to 1)")
		}
	}
}

func TestRateSamplerDefaultPerSecond(t *testing.T) {
	s := NewRateSampler(0) // should default to 1
	if !s.Sample(INFO, "first") {
		t.Error("expected first call to pass")
	}
	if s.Sample(INFO, "second") {
		t.Error("expected second call to be dropped with perSecond=0 (default 1)")
	}
}

// decodeAllEntriesSampler is a local helper to avoid import cycles.
func decodeAllEntriesSampler(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	for dec.More() {
		var entry map[string]any
		if err := dec.Decode(&entry); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}
