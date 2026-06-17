package content

import (
	"sync"
	"sync/atomic"
)

// ProgressDirection indicates whether progress is for an upload or a download.
type ProgressDirection string

const (
	ProgressUpload   ProgressDirection = "upload"
	ProgressDownload ProgressDirection = "download"
)

// ProgressReporter tracks byte-level progress for an upload or download
// operation. It is safe for concurrent use: callers may invoke the record
// methods from multiple goroutines (for example, when streaming chunks in
// parallel) while a separate goroutine polls the snapshot methods for UI
// updates.
type ProgressReporter struct {
	direction ProgressDirection
	total     int64 // total bytes expected (0 = unknown / streaming)
	transferred int64 // bytes processed so far (atomic)
	started   atomic.Bool
	completed atomic.Bool
	mu        sync.RWMutex
	chunks    int // number of chunks processed
}

// NewProgressReporter creates a reporter for the given direction. If total is
// known (e.g. file size on upload), pass it so that percentage can be computed;
// otherwise pass 0 for an indeterminate operation.
func NewProgressReporter(direction ProgressDirection, total int64) *ProgressReporter {
	return &ProgressReporter{
		direction: direction,
		total:     total,
	}
}

// Direction returns the configured direction.
func (p *ProgressReporter) Direction() ProgressDirection { return p.direction }

// Total returns the total number of bytes expected (0 if unknown).
func (p *ProgressReporter) Total() int64 { return atomic.LoadInt64(&p.total) }

// SetTotal updates the total bytes expected. Useful when the size is discovered
// mid-operation (for example, after parsing a multipart header).
func (p *ProgressReporter) SetTotal(total int64) {
	atomic.StoreInt64(&p.total, total)
}

// Transferred returns the number of bytes processed so far.
func (p *ProgressReporter) Transferred() int64 {
	return atomic.LoadInt64(&p.transferred)
}

// Record adds n bytes to the transferred counter and marks the reporter as
// started. Negative values are ignored. n is the chunk size just processed.
func (p *ProgressReporter) Record(n int) {
	if n <= 0 {
		return
	}
	p.started.Store(true)
	atomic.AddInt64(&p.transferred, int64(n))
	p.mu.Lock()
	p.chunks++
	p.mu.Unlock()
}

// ChunksProcessed returns the number of Record calls that advanced the counter.
func (p *ProgressReporter) ChunksProcessed() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.chunks
}

// Percentage returns completion as a float in [0, 100]. If Total is unknown
// (0) the result is 0 to avoid division by zero.
func (p *ProgressReporter) Percentage() float64 {
	total := atomic.LoadInt64(&p.total)
	if total <= 0 {
		return 0
	}
	done := atomic.LoadInt64(&p.transferred)
	if done >= total {
		return 100
	}
	return float64(done) / float64(total) * 100
}

// Complete marks the operation as finished. After Complete, Percentage returns
// 100 (when Total is known) regardless of minor counting drift.
func (p *ProgressReporter) Complete() {
	p.completed.Store(true)
	total := atomic.LoadInt64(&p.total)
	if total > 0 {
		// Pin transferred to total so percentage reads exactly 100.
		atomic.StoreInt64(&p.transferred, total)
	}
}

// IsCompleted reports whether Complete has been called.
func (p *ProgressReporter) IsCompleted() bool { return p.completed.Load() }

// IsStarted reports whether at least one Record call has been made.
func (p *ProgressReporter) IsStarted() bool { return p.started.Load() }

// Snapshot returns an immutable view of the current progress suitable for
// serialising to JSON or pushing to a websocket client.
type ProgressSnapshot struct {
	Direction   ProgressDirection `json:"direction"`
	Total       int64             `json:"total"`
	Transferred int64             `json:"transferred"`
	Percentage  float64           `json:"percentage"`
	Chunks      int               `json:"chunks"`
	Completed   bool              `json:"completed"`
}

// Snapshot captures the current state of the reporter atomically.
func (p *ProgressReporter) Snapshot() ProgressSnapshot {
	return ProgressSnapshot{
		Direction:   p.direction,
		Total:       atomic.LoadInt64(&p.total),
		Transferred: atomic.LoadInt64(&p.transferred),
		Percentage:  p.Percentage(),
		Chunks:      p.ChunksProcessed(),
		Completed:   p.completed.Load(),
	}
}
