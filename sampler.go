package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

// Sampler decides whether a log entry should be written.
// Return true to allow the entry, false to drop it.
//
// ERROR and FATAL entries bypass the sampler entirely — they are
// never dropped, regardless of what Sample returns.
type Sampler interface {
	Sample(level Level, message string) bool
}

// EveryNSampler allows the first entry and then every Nth entry
// per level. For example, NewEveryNSampler(5) logs the 1st, 6th,
// 11th, etc. entry at each level.
type EveryNSampler struct {
	n      int64
	counts [6]atomic.Int64 // one counter per level (TRACE..FATAL)
}

// NewEveryNSampler creates a sampler that passes every nth entry per level.
func NewEveryNSampler(n int) *EveryNSampler {
	if n <= 0 {
		n = 1
	}
	return &EveryNSampler{n: int64(n)}
}

// Sample returns true for the 1st call and every Nth call at a given level.
func (s *EveryNSampler) Sample(level Level, _ string) bool {
	if int(level) >= len(s.counts) {
		return true
	}
	count := s.counts[level].Add(1)
	return (count-1)%s.n == 0
}

// RateSampler allows at most N entries per second per level.
// When the budget is exhausted, entries are dropped until the
// next second window.
type RateSampler struct {
	perSecond int
	buckets   [6]rateBucket // one per level
}

type rateBucket struct {
	mu      sync.Mutex
	count   int
	resetAt time.Time
}

// NewRateSampler creates a sampler that allows at most perSecond
// entries per second at each level.
func NewRateSampler(perSecond int) *RateSampler {
	if perSecond <= 0 {
		perSecond = 1
	}
	return &RateSampler{perSecond: perSecond}
}

// Sample returns true if the per-second budget for this level has not
// been exhausted.
func (s *RateSampler) Sample(level Level, _ string) bool {
	if int(level) >= len(s.buckets) {
		return true
	}
	b := &s.buckets[level]
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.After(b.resetAt) {
		b.count = 0
		b.resetAt = now.Add(time.Second)
	}
	b.count++
	return b.count <= s.perSecond
}
