package urlsegmentenricher

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestModuleMetrics_RecordRequest(t *testing.T) {
	m := &ModuleMetrics{}

	m.RecordRequest()
	m.RecordRequest()
	m.RecordRequest()

	snap := m.Snapshot()
	assert.Equal(t, int64(3), snap.TotalRequests)
}

func TestModuleMetrics_RecordFetchSuccess(t *testing.T) {
	m := &ModuleMetrics{}

	m.RecordFetchSuccess(100*time.Millisecond, 5)
	m.RecordFetchSuccess(200*time.Millisecond, 3)

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.SuccessfulFetches)
	assert.Equal(t, int64(8), snap.TotalSegmentsAdded)
	// Average should be (100 + 200) / 2 = 150ms
	assert.Equal(t, float64(150), snap.AvgFetchDurationMs)
}

func TestModuleMetrics_RecordFetchFailure_NonTimeout(t *testing.T) {
	m := &ModuleMetrics{}

	m.RecordFetchFailure(false)
	m.RecordFetchFailure(false)

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.FailedFetches)
	assert.Equal(t, int64(0), snap.TimeoutFetches)
}

func TestModuleMetrics_RecordFetchFailure_Timeout(t *testing.T) {
	m := &ModuleMetrics{}

	m.RecordFetchFailure(true)
	m.RecordFetchFailure(false)
	m.RecordFetchFailure(true)

	snap := m.Snapshot()
	assert.Equal(t, int64(3), snap.FailedFetches)
	assert.Equal(t, int64(2), snap.TimeoutFetches)
}

func TestModuleMetrics_RecordSkipped(t *testing.T) {
	m := &ModuleMetrics{}

	m.RecordSkipped()
	m.RecordSkipped()

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.SkippedRequests)
}

func TestModuleMetrics_Snapshot_ZeroValues(t *testing.T) {
	m := &ModuleMetrics{}

	snap := m.Snapshot()

	assert.Equal(t, int64(0), snap.TotalRequests)
	assert.Equal(t, int64(0), snap.SuccessfulFetches)
	assert.Equal(t, int64(0), snap.FailedFetches)
	assert.Equal(t, int64(0), snap.TimeoutFetches)
	assert.Equal(t, int64(0), snap.SkippedRequests)
	assert.Equal(t, int64(0), snap.TotalSegmentsAdded)
	assert.Equal(t, float64(0), snap.AvgFetchDurationMs)
}

func TestModuleMetrics_Snapshot_AvgDuration_NoFetches(t *testing.T) {
	m := &ModuleMetrics{}

	snap := m.Snapshot()

	// Should not divide by zero
	assert.Equal(t, float64(0), snap.AvgFetchDurationMs)
}

func TestModuleMetrics_ConcurrentAccess(t *testing.T) {
	m := &ModuleMetrics{}
	const goroutines = 100
	const iterations = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				m.RecordRequest()
				m.RecordFetchSuccess(10*time.Millisecond, 2)
				m.RecordFetchFailure(j%2 == 0)
				m.RecordSkipped()
			}
		}()
	}

	wg.Wait()

	snap := m.Snapshot()
	assert.Equal(t, int64(goroutines*iterations), snap.TotalRequests)
	assert.Equal(t, int64(goroutines*iterations), snap.SuccessfulFetches)
	assert.Equal(t, int64(goroutines*iterations*2), snap.TotalSegmentsAdded)
	assert.Equal(t, int64(goroutines*iterations), snap.FailedFetches)
	assert.Equal(t, int64(goroutines*iterations), snap.SkippedRequests)
}

func TestModuleMetrics_MixedOperations(t *testing.T) {
	m := &ModuleMetrics{}

	// Simulate a realistic usage pattern
	for i := 0; i < 10; i++ {
		m.RecordRequest()
	}
	for i := 0; i < 7; i++ {
		m.RecordFetchSuccess(50*time.Millisecond, 3)
	}
	for i := 0; i < 2; i++ {
		m.RecordFetchFailure(false)
	}
	m.RecordFetchFailure(true)
	m.RecordSkipped()

	snap := m.Snapshot()
	assert.Equal(t, int64(10), snap.TotalRequests)
	assert.Equal(t, int64(7), snap.SuccessfulFetches)
	assert.Equal(t, int64(3), snap.FailedFetches)
	assert.Equal(t, int64(1), snap.TimeoutFetches)
	assert.Equal(t, int64(1), snap.SkippedRequests)
	assert.Equal(t, int64(21), snap.TotalSegmentsAdded)
	assert.Equal(t, float64(50), snap.AvgFetchDurationMs)
}
