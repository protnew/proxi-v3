package content

import (
	"sync"
	"testing"
)

func TestProgressReporter_UploadTracking(t *testing.T) {
	pr := NewProgressReporter(ProgressUpload, 1000)

	if pr.Direction() != ProgressUpload {
		t.Errorf("Direction = %q, want %q", pr.Direction(), ProgressUpload)
	}
	if pr.Total() != 1000 {
		t.Errorf("Total = %d, want 1000", pr.Total())
	}
	if pr.Transferred() != 0 {
		t.Errorf("Transferred = %d, want 0", pr.Transferred())
	}
	if pr.IsStarted() {
		t.Error("should not be started before Record")
	}
	if pr.IsCompleted() {
		t.Error("should not be completed")
	}

	// Record 5 chunks of 200 bytes each = 1000 bytes total.
	for i := 0; i < 5; i++ {
		pr.Record(200)
	}

	if pr.Transferred() != 1000 {
		t.Errorf("Transferred = %d, want 1000", pr.Transferred())
	}
	if pr.ChunksProcessed() != 5 {
		t.Errorf("ChunksProcessed = %d, want 5", pr.ChunksProcessed())
	}
	if !pr.IsStarted() {
		t.Error("should be started after Record")
	}
	if got := pr.Percentage(); got != 100 {
		t.Errorf("Percentage = %f, want 100", got)
	}

	pr.Complete()
	if !pr.IsCompleted() {
		t.Error("should be completed after Complete")
	}
}

func TestProgressReporter_PercentagePartial(t *testing.T) {
	pr := NewProgressReporter(ProgressDownload, 400)

	if got := pr.Percentage(); got != 0 {
		t.Errorf("initial Percentage = %f, want 0", got)
	}

	pr.Record(100)
	if got := pr.Percentage(); got != 25 {
		t.Errorf("Percentage after 100/400 = %f, want 25", got)
	}

	pr.Record(100)
	if got := pr.Percentage(); got != 50 {
		t.Errorf("Percentage after 200/400 = %f, want 50", got)
	}

	pr.Record(50)
	if got := pr.Percentage(); got != 62.5 {
		t.Errorf("Percentage after 250/400 = %f, want 62.5", got)
	}
}

func TestProgressReporter_UnknownTotal(t *testing.T) {
	pr := NewProgressReporter(ProgressUpload, 0)

	pr.Record(500)
	pr.Record(500)

	if pr.Total() != 0 {
		t.Errorf("Total = %d, want 0", pr.Total())
	}
	// With unknown total, percentage must be 0 (no division by zero).
	if got := pr.Percentage(); got != 0 {
		t.Errorf("Percentage with unknown total = %f, want 0", got)
	}
	if pr.Transferred() != 1000 {
		t.Errorf("Transferred = %d, want 1000", pr.Transferred())
	}
}

func TestProgressReporter_OverRecord(t *testing.T) {
	pr := NewProgressReporter(ProgressDownload, 100)
	pr.Record(150) // record more than total

	// Percentage clamped to 100.
	if got := pr.Percentage(); got != 100 {
		t.Errorf("Percentage after over-record = %f, want 100", got)
	}
}

func TestProgressReporter_NegativeIgnored(t *testing.T) {
	pr := NewProgressReporter(ProgressUpload, 1000)
	pr.Record(-100)
	pr.Record(0)

	if pr.Transferred() != 0 {
		t.Errorf("Transferred = %d, want 0 (negative/zero ignored)", pr.Transferred())
	}
	if pr.ChunksProcessed() != 0 {
		t.Errorf("ChunksProcessed = %d, want 0", pr.ChunksProcessed())
	}
}

func TestProgressReporter_SetTotal(t *testing.T) {
	pr := NewProgressReporter(ProgressUpload, 0)
	if pr.Percentage() != 0 {
		t.Error("percentage should be 0 with unknown total")
	}

	pr.SetTotal(1000)
	pr.Record(500)
	if got := pr.Percentage(); got != 50 {
		t.Errorf("Percentage after SetTotal+Record = %f, want 50", got)
	}
}

func TestProgressReporter_Snapshot(t *testing.T) {
	pr := NewProgressReporter(ProgressDownload, 800)
	pr.Record(200)
	pr.Record(200)

	snap := pr.Snapshot()
	if snap.Direction != ProgressDownload {
		t.Errorf("snapshot Direction = %q, want %q", snap.Direction, ProgressDownload)
	}
	if snap.Total != 800 {
		t.Errorf("snapshot Total = %d, want 800", snap.Total)
	}
	if snap.Transferred != 400 {
		t.Errorf("snapshot Transferred = %d, want 400", snap.Transferred)
	}
	if snap.Percentage != 50 {
		t.Errorf("snapshot Percentage = %f, want 50", snap.Percentage)
	}
	if snap.Chunks != 2 {
		t.Errorf("snapshot Chunks = %d, want 2", snap.Chunks)
	}
	if snap.Completed {
		t.Error("snapshot should not be completed")
	}

	pr.Complete()
	snap = pr.Snapshot()
	if !snap.Completed {
		t.Error("snapshot should be completed after Complete")
	}
}

func TestProgressReporter_Concurrent(t *testing.T) {
	pr := NewProgressReporter(ProgressUpload, 10000)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pr.Record(100)
		}()
	}
	wg.Wait()

	if pr.Transferred() != 10000 {
		t.Errorf("Transferred after concurrent = %d, want 10000", pr.Transferred())
	}
	if pr.ChunksProcessed() != 100 {
		t.Errorf("ChunksProcessed = %d, want 100", pr.ChunksProcessed())
	}
	if got := pr.Percentage(); got != 100 {
		t.Errorf("Percentage = %f, want 100", got)
	}
}
