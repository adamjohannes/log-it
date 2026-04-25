package logger

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestAsyncWriterDrainsAllMessages(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 256)

	for i := 0; i < 100; i++ {
		_, _ = aw.Write([]byte("line\n"))
	}
	if err := aw.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	lines := strings.Count(buf.String(), "line\n")
	if lines != 100 {
		t.Errorf("expected 100 lines, got %d", lines)
	}
}

func TestAsyncWriterDropsWhenFull(t *testing.T) {
	// Use a slow/blocked writer with buffer size 1.
	// We need the drain goroutine to be blocked so the channel fills up.
	blockCh := make(chan struct{})
	bw := &blockingWriter{blockCh: blockCh}
	aw := NewAsyncWriter(bw, 1)

	// First write goes into the channel (buffer size 1).
	_, _ = aw.Write([]byte("msg1"))

	// The drain goroutine picks up msg1 and blocks on bw.Write.
	// Second write fills the channel buffer.
	// Give drain a moment to pick up msg1.
	// We write enough to guarantee at least one drop.
	for i := 0; i < 10; i++ {
		_, _ = aw.Write([]byte("overflow"))
	}

	// Unblock the writer and close.
	close(blockCh)
	_ = aw.Close()

	if aw.DroppedCount() == 0 {
		t.Error("expected at least one dropped message")
	}
}

// blockingWriter blocks on Write until blockCh is closed.
type blockingWriter struct {
	blockCh chan struct{}
	once    sync.Once
	buf     bytes.Buffer
}

func (b *blockingWriter) Write(p []byte) (int, error) {
	b.once.Do(func() {
		<-b.blockCh // block on the first write only
	})
	return b.buf.Write(p)
}

func TestAsyncWriterCloseIdempotent(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 16)
	_, _ = aw.Write([]byte("test"))

	if err := aw.Close(); err != nil {
		t.Fatalf("first close error: %v", err)
	}
	// Second close should not panic.
	if err := aw.Close(); err != nil {
		t.Fatalf("second close error: %v", err)
	}
}

func TestAsyncWriterWriteAfterClose(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 16)
	_ = aw.Close()

	// Should not panic thanks to recover in Write.
	n, _ := aw.Write([]byte("after-close"))
	if n != 0 {
		t.Errorf("expected n=0 for write-after-close, got %d", n)
	}
}

func TestAsyncWriterSyncCallsDestSync(t *testing.T) {
	sr := &syncRecorderAsync{}
	aw := NewAsyncWriter(sr, 16)
	_, _ = aw.Write([]byte("data"))

	if err := aw.Sync(); err != nil {
		t.Fatalf("sync error: %v", err)
	}
	if !sr.synced {
		t.Error("expected destination Sync() to be called")
	}
}

type syncRecorderAsync struct {
	bytes.Buffer
	synced bool
}

func (s *syncRecorderAsync) Sync() error {
	s.synced = true
	return nil
}

func TestAsyncWriterConcurrency(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 8192)

	var wg sync.WaitGroup
	for g := 0; g < 100; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_, _ = aw.Write([]byte("x"))
			}
		}()
	}
	wg.Wait()
	_ = aw.Close()

	total := int64(buf.Len()) + aw.DroppedCount()
	if total != 10000 {
		t.Errorf("expected 10000 total (written+dropped), got %d (written=%d, dropped=%d)",
			total, buf.Len(), aw.DroppedCount())
	}
}

func TestAsyncWriterDefaultBufferSize(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAsyncWriter(&buf, 0)
	_, _ = aw.Write([]byte("ok"))
	_ = aw.Close()

	if buf.String() != "ok" {
		t.Errorf("expected 'ok', got %q", buf.String())
	}
}
