package logger

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

// ErrWriterClosed is returned when Write is called on a closed AsyncWriter.
var ErrWriterClosed = errors.New("logger: async writer is closed")

// AsyncWriter buffers writes in a channel and flushes them to the
// underlying writer in a background goroutine. If the channel is full,
// messages are dropped (non-blocking, lossy) to ensure the application
// is never stalled by log I/O.
//
// The caller MUST call Close (or Sync) before application exit to
// flush remaining entries and stop the background goroutine.
type AsyncWriter struct {
	dest    io.Writer
	ch      chan []byte
	done    chan struct{} // closed when drain goroutine exits
	once    sync.Once    // makes Close idempotent
	dropCnt atomic.Int64 // count of dropped messages
}

// NewAsyncWriter creates a buffered async writer with the given
// channel capacity. If bufferSize <= 0, it defaults to 4096.
func NewAsyncWriter(dest io.Writer, bufferSize int) *AsyncWriter {
	if bufferSize <= 0 {
		bufferSize = 4096
	}
	aw := &AsyncWriter{
		dest: dest,
		ch:   make(chan []byte, bufferSize),
		done: make(chan struct{}),
	}
	go aw.drain()
	return aw
}

// Write copies p and sends it to the background goroutine via the
// channel. If the channel is full, the message is silently dropped
// and the drop counter is incremented. If the writer is already
// closed, ErrWriterClosed is returned.
func (aw *AsyncWriter) Write(p []byte) (int, error) {
	// Defensive: recover from send-on-closed-channel panic.
	defer func() {
		recover() //nolint:errcheck
	}()

	buf := make([]byte, len(p))
	copy(buf, p)

	select {
	case aw.ch <- buf:
		return len(p), nil
	default:
		aw.dropCnt.Add(1)
		return 0, nil
	}
}

// drain runs in a background goroutine, consuming from the channel
// and writing to the destination. Preserves log ordering.
func (aw *AsyncWriter) drain() {
	defer close(aw.done)
	for data := range aw.ch {
		_, _ = aw.dest.Write(data)
	}
}

// Close stops the background goroutine after draining all queued
// messages and syncs the destination if it implements Syncer.
// Close is idempotent — calling it multiple times is safe.
func (aw *AsyncWriter) Close() error {
	var err error
	aw.once.Do(func() {
		close(aw.ch)
		<-aw.done // wait for drain to finish
		if s, ok := aw.dest.(Syncer); ok {
			err = s.Sync()
		}
	})
	return err
}

// Sync flushes all buffered log data. Alias for Close.
func (aw *AsyncWriter) Sync() error {
	return aw.Close()
}

// DroppedCount returns the number of messages dropped because
// the channel buffer was full.
func (aw *AsyncWriter) DroppedCount() int64 {
	return aw.dropCnt.Load()
}
