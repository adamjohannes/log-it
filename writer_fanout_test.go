package logger

import (
	"bytes"
	"errors"
	"testing"
)

// errWriter is a test writer that always returns an error.
type errWriter struct{ err error }

func (e *errWriter) Write([]byte) (int, error) { return 0, e.err }

// syncRecorder records whether Sync was called and can return an error.
type syncRecorder struct {
	bytes.Buffer
	synced bool
	err    error
}

func (s *syncRecorder) Sync() error {
	s.synced = true
	return s.err
}

func TestFanOutWritesToAllWriters(t *testing.T) {
	var a, b bytes.Buffer
	fan := NewFanOutWriter(&a, &b)

	msg := []byte("hello\n")
	n, err := fan.Write(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("expected n=%d, got %d", len(msg), n)
	}
	if a.String() != "hello\n" {
		t.Errorf("writer a: expected %q, got %q", "hello\n", a.String())
	}
	if b.String() != "hello\n" {
		t.Errorf("writer b: expected %q, got %q", "hello\n", b.String())
	}
}

func TestFanOutFailsFastOnError(t *testing.T) {
	var a bytes.Buffer
	e := &errWriter{err: errors.New("disk full")}
	fan := NewFanOutWriter(&a, e)

	_, err := fan.Write([]byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, e.err) {
		t.Errorf("expected wrapped disk full error, got: %v", err)
	}
}

func TestFanOutSyncCallsSyncers(t *testing.T) {
	s1 := &syncRecorder{}
	s2 := &syncRecorder{}
	var plain bytes.Buffer // does not implement Syncer
	fan := NewFanOutWriter(s1, &plain, s2)

	err := fan.Sync()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s1.synced {
		t.Error("expected s1.Sync() to be called")
	}
	if !s2.synced {
		t.Error("expected s2.Sync() to be called")
	}
}

func TestFanOutSyncCollectsErrors(t *testing.T) {
	s1 := &syncRecorder{err: errors.New("sync1 failed")}
	s2 := &syncRecorder{err: errors.New("sync2 failed")}
	fan := NewFanOutWriter(s1, s2)

	err := fan.Sync()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, s1.err) {
		t.Errorf("expected sync1 error in result: %v", err)
	}
	if !errors.Is(err, s2.err) {
		t.Errorf("expected sync2 error in result: %v", err)
	}
}

func TestFanOutZeroWriters(t *testing.T) {
	fan := NewFanOutWriter()
	n, err := fan.Write([]byte("nothing"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len("nothing") {
		t.Errorf("expected n=%d, got %d", len("nothing"), n)
	}

	if err := fan.Sync(); err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}
}
