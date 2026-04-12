package logger

import (
	"errors"
	"fmt"
	"io"
)

// Syncer is implemented by writers that can flush buffered data.
// *os.File naturally satisfies this interface.
type Syncer interface {
	Sync() error
}

// FanOutWriter duplicates each Write call to all underlying writers.
// It implements io.Writer and Syncer.
type FanOutWriter struct {
	writers []io.Writer
}

// NewFanOutWriter creates a writer that broadcasts writes to all
// provided writers. Thread safety is not needed here because the
// logger's mutex serializes all Write calls.
func NewFanOutWriter(writers ...io.Writer) *FanOutWriter {
	return &FanOutWriter{writers: writers}
}

// Write sends p to each underlying writer. Fails fast on the first error.
func (f *FanOutWriter) Write(p []byte) (int, error) {
	for _, w := range f.writers {
		n, err := w.Write(p)
		if err != nil {
			return n, fmt.Errorf("fanout write failed: %w", err)
		}
	}
	return len(p), nil
}

// Sync calls Sync on every underlying writer that implements Syncer.
// Errors from all writers are collected and joined.
func (f *FanOutWriter) Sync() error {
	var errs []error
	for _, w := range f.writers {
		if s, ok := w.(Syncer); ok {
			if err := s.Sync(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
