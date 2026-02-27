package urlsegmentenricher

import (
	"sync/atomic"
	"time"
)

// ModuleMetrics tracks operational metrics for the URL segment enricher module.
// It uses atomic counters for thread-safe operation without locks.
type ModuleMetrics struct {
	// TotalRequests is the total number of hook invocations.
	TotalRequests atomic.Int64
	// SuccessfulFetches is the number of successful external URL calls.
	SuccessfulFetches atomic.Int64
	// FailedFetches is the number of failed external URL calls.
	FailedFetches atomic.Int64
	// TimeoutFetches is the number of external URL calls that timed out.
	TimeoutFetches atomic.Int64
	// SkippedRequests is the number of requests skipped (e.g., no page URL).
	SkippedRequests atomic.Int64
	// TotalSegmentsAdded is the total number of segments added across all requests.
	TotalSegmentsAdded atomic.Int64
	// TotalFetchDurationMs is the cumulative fetch duration in milliseconds.
	TotalFetchDurationMs atomic.Int64
	// FetchCount is the number of fetches that completed (for average calculation).
	FetchCount atomic.Int64
}

// MetricsRecorder defines the interface for recording module metrics.
// This interface allows for easy mocking in tests.
type MetricsRecorder interface {
	RecordRequest()
	RecordFetchSuccess(duration time.Duration, segmentCount int)
	RecordFetchFailure(isTimeout bool)
	RecordSkipped()
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot holds a point-in-time snapshot of module metrics.
type MetricsSnapshot struct {
	TotalRequests        int64
	SuccessfulFetches    int64
	FailedFetches        int64
	TimeoutFetches       int64
	SkippedRequests      int64
	TotalSegmentsAdded   int64
	AvgFetchDurationMs   float64
}

// RecordRequest increments the total request counter.
func (m *ModuleMetrics) RecordRequest() {
	m.TotalRequests.Add(1)
}

// RecordFetchSuccess records a successful fetch with its duration and segment count.
func (m *ModuleMetrics) RecordFetchSuccess(duration time.Duration, segmentCount int) {
	m.SuccessfulFetches.Add(1)
	m.TotalSegmentsAdded.Add(int64(segmentCount))
	m.TotalFetchDurationMs.Add(duration.Milliseconds())
	m.FetchCount.Add(1)
}

// RecordFetchFailure records a failed fetch, distinguishing timeouts from other errors.
func (m *ModuleMetrics) RecordFetchFailure(isTimeout bool) {
	m.FailedFetches.Add(1)
	if isTimeout {
		m.TimeoutFetches.Add(1)
	}
}

// RecordSkipped records a skipped request (e.g., no page URL available).
func (m *ModuleMetrics) RecordSkipped() {
	m.SkippedRequests.Add(1)
}

// Snapshot returns a point-in-time snapshot of the current metrics.
func (m *ModuleMetrics) Snapshot() MetricsSnapshot {
	fetchCount := m.FetchCount.Load()
	var avgDuration float64
	if fetchCount > 0 {
		avgDuration = float64(m.TotalFetchDurationMs.Load()) / float64(fetchCount)
	}

	return MetricsSnapshot{
		TotalRequests:      m.TotalRequests.Load(),
		SuccessfulFetches:  m.SuccessfulFetches.Load(),
		FailedFetches:      m.FailedFetches.Load(),
		TimeoutFetches:     m.TimeoutFetches.Load(),
		SkippedRequests:    m.SkippedRequests.Load(),
		TotalSegmentsAdded: m.TotalSegmentsAdded.Load(),
		AvgFetchDurationMs: avgDuration,
	}
}
