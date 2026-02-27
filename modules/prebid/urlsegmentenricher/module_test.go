package urlsegmentenricher

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFetcher is a test double for SegmentFetcher.
type mockFetcher struct {
	segments []SegmentData
	err      error
	called   bool
	lastURL  string
}

func (m *mockFetcher) FetchSegments(_ context.Context, pageURL string) ([]SegmentData, error) {
	m.called = true
	m.lastURL = pageURL
	return m.segments, m.err
}

// mockMetrics is a test double for MetricsRecorder.
type mockMetrics struct {
	requests       int
	fetchSuccess   int
	fetchFailure   int
	timeouts       int
	skipped        int
	segmentsAdded  int
	lastDuration   time.Duration
}

func (m *mockMetrics) RecordRequest() {
	m.requests++
}

func (m *mockMetrics) RecordFetchSuccess(duration time.Duration, segmentCount int) {
	m.fetchSuccess++
	m.segmentsAdded += segmentCount
	m.lastDuration = duration
}

func (m *mockMetrics) RecordFetchFailure(isTimeout bool) {
	m.fetchFailure++
	if isTimeout {
		m.timeouts++
	}
}

func (m *mockMetrics) RecordSkipped() {
	m.skipped++
}

func (m *mockMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		TotalRequests:      int64(m.requests),
		SuccessfulFetches:  int64(m.fetchSuccess),
		FailedFetches:      int64(m.fetchFailure),
		TimeoutFetches:     int64(m.timeouts),
		SkippedRequests:    int64(m.skipped),
		TotalSegmentsAdded: int64(m.segmentsAdded),
	}
}

// newTestModule creates a Module with mock dependencies for testing.
func newTestModule(fetcher SegmentFetcher, metrics MetricsRecorder, cfg Config) *Module {
	if cfg.TimeoutMs == 0 {
		cfg.TimeoutMs = DefaultTimeoutMs
	}
	if cfg.URLQueryParam == "" {
		cfg.URLQueryParam = "url"
	}
	if cfg.SegmentTaxonomy == 0 {
		cfg.SegmentTaxonomy = DefaultSegmentTaxonomy
	}
	if cfg.MaxSegments == 0 {
		cfg.MaxSegments = DefaultMaxSegments
	}
	if cfg.SegmentDataName == "" {
		cfg.SegmentDataName = "urlsegmentenricher"
	}
	return &Module{
		cfg:     cfg,
		fetcher: fetcher,
		metrics: metrics,
	}
}

// newTestPayload creates a ProcessedAuctionRequestPayload with a site bid request.
func newTestPayload(pageURL string) hookstage.ProcessedAuctionRequestPayload {
	bidRequest := &openrtb2.BidRequest{
		Site: &openrtb2.Site{
			Page: pageURL,
		},
	}
	return hookstage.ProcessedAuctionRequestPayload{
		Request: &openrtb_ext.RequestWrapper{
			BidRequest: bidRequest,
		},
	}
}

// newTestAppPayload creates a ProcessedAuctionRequestPayload with an app bid request.
func newTestAppPayload(bundle string) hookstage.ProcessedAuctionRequestPayload {
	bidRequest := &openrtb2.BidRequest{
		App: &openrtb2.App{
			Bundle: bundle,
		},
	}
	return hookstage.ProcessedAuctionRequestPayload{
		Request: &openrtb_ext.RequestWrapper{
			BidRequest: bidRequest,
		},
	}
}

// ---- Builder Tests ----

func TestBuilder_ValidConfig(t *testing.T) {
	rawCfg := json.RawMessage(`{
		"enabled": true,
		"external_url": "https://segments.example.com/api",
		"timeout_ms": 300,
		"url_query_param": "page",
		"segment_taxonomy": 4,
		"max_segments": 50,
		"segment_data_name": "test-provider"
	}`)

	deps := moduledeps.ModuleDeps{HTTPClient: http.DefaultClient}
	module, err := Builder(rawCfg, deps)

	require.NoError(t, err)
	require.NotNil(t, module)

	m, ok := module.(*Module)
	require.True(t, ok)
	assert.Equal(t, "https://segments.example.com/api", m.cfg.ExternalURL)
	assert.Equal(t, 300, m.cfg.TimeoutMs)
	assert.Equal(t, "page", m.cfg.URLQueryParam)
	assert.Equal(t, 4, m.cfg.SegmentTaxonomy)
	assert.Equal(t, 50, m.cfg.MaxSegments)
	assert.Equal(t, "test-provider", m.cfg.SegmentDataName)
}

func TestBuilder_DefaultValues(t *testing.T) {
	rawCfg := json.RawMessage(`{
		"enabled": true,
		"external_url": "https://segments.example.com/api"
	}`)

	deps := moduledeps.ModuleDeps{HTTPClient: http.DefaultClient}
	module, err := Builder(rawCfg, deps)

	require.NoError(t, err)
	require.NotNil(t, module)

	m, ok := module.(*Module)
	require.True(t, ok)
	assert.Equal(t, DefaultTimeoutMs, m.cfg.TimeoutMs)
	assert.Equal(t, "url", m.cfg.URLQueryParam)
	assert.Equal(t, DefaultSegmentTaxonomy, m.cfg.SegmentTaxonomy)
	assert.Equal(t, DefaultMaxSegments, m.cfg.MaxSegments)
	assert.Equal(t, "urlsegmentenricher", m.cfg.SegmentDataName)
}

func TestBuilder_InvalidJSON(t *testing.T) {
	rawCfg := json.RawMessage(`invalid json`)
	deps := moduledeps.ModuleDeps{HTTPClient: http.DefaultClient}

	module, err := Builder(rawCfg, deps)

	assert.Error(t, err)
	assert.Nil(t, module)
}

func TestBuilder_MissingExternalURL(t *testing.T) {
	rawCfg := json.RawMessage(`{
		"enabled": true,
		"timeout_ms": 300
	}`)

	deps := moduledeps.ModuleDeps{HTTPClient: http.DefaultClient}
	module, err := Builder(rawCfg, deps)

	assert.Error(t, err)
	assert.Nil(t, module)
	assert.Contains(t, err.Error(), "external_url is required")
}

func TestBuilder_NilHTTPClient(t *testing.T) {
	rawCfg := json.RawMessage(`{
		"enabled": true,
		"external_url": "https://segments.example.com/api"
	}`)

	deps := moduledeps.ModuleDeps{HTTPClient: nil}
	module, err := Builder(rawCfg, deps)

	require.NoError(t, err)
	require.NotNil(t, module)
}

// ---- HandleProcessedAuctionHook Tests ----

func TestHandleProcessedAuctionHook_SiteRequest_Success(t *testing.T) {
	segments := []SegmentData{
		{ID: "seg-1", Name: "Sports"},
		{ID: "seg-2", Name: "Tech"},
	}
	fetcher := &mockFetcher{segments: segments}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
	})

	payload := newTestPayload("https://example.com/page")
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	assert.Empty(t, result.Errors)
	assert.True(t, fetcher.called)
	assert.Equal(t, "https://example.com/page", fetcher.lastURL)
	assert.Equal(t, 1, metrics.requests)
	assert.Equal(t, 1, metrics.fetchSuccess)
	assert.Equal(t, 2, metrics.segmentsAdded)
	assert.Len(t, result.ChangeSet.Mutations(), 1)
}

func TestHandleProcessedAuctionHook_AppRequest_Success(t *testing.T) {
	segments := []SegmentData{
		{ID: "seg-1", Name: "Gaming"},
	}
	fetcher := &mockFetcher{segments: segments}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
	})

	payload := newTestAppPayload("com.example.app")
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	assert.True(t, fetcher.called)
	assert.Equal(t, "com.example.app", fetcher.lastURL)
	assert.Equal(t, 1, metrics.fetchSuccess)
	assert.Len(t, result.ChangeSet.Mutations(), 1)
}

func TestHandleProcessedAuctionHook_NilRequest(t *testing.T) {
	fetcher := &mockFetcher{}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
	})

	payload := hookstage.ProcessedAuctionRequestPayload{Request: nil}
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	assert.False(t, fetcher.called)
	assert.Equal(t, 1, metrics.skipped)
	assert.Len(t, result.Warnings, 1)
}

func TestHandleProcessedAuctionHook_NoPageURL(t *testing.T) {
	fetcher := &mockFetcher{}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
	})

	// Bid request with no site.page or app.bundle
	payload := hookstage.ProcessedAuctionRequestPayload{
		Request: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				Site: &openrtb2.Site{
					Domain: "example.com",
					// No Page field
				},
			},
		},
	}
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	assert.False(t, fetcher.called)
	assert.Equal(t, 1, metrics.skipped)
	assert.Len(t, result.DebugMessages, 1)
}

func TestHandleProcessedAuctionHook_FetchError(t *testing.T) {
	fetcher := &mockFetcher{err: errors.New("connection refused")}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
	})

	payload := newTestPayload("https://example.com/page")
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	// Should not return an error (non-blocking)
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.fetchFailure)
	assert.Equal(t, 0, metrics.timeouts)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "connection refused")
}

func TestHandleProcessedAuctionHook_TimeoutError(t *testing.T) {
	fetcher := &mockFetcher{err: errors.New("context deadline exceeded")}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
	})

	payload := newTestPayload("https://example.com/page")
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	assert.Equal(t, 1, metrics.fetchFailure)
	assert.Equal(t, 1, metrics.timeouts)
	assert.Len(t, result.Warnings, 1)
}

func TestHandleProcessedAuctionHook_EmptySegments(t *testing.T) {
	fetcher := &mockFetcher{segments: []SegmentData{}}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
	})

	payload := newTestPayload("https://example.com/page")
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	assert.Equal(t, 1, metrics.fetchSuccess)
	assert.Equal(t, 0, metrics.segmentsAdded)
	assert.Empty(t, result.ChangeSet.Mutations())
}

func TestHandleProcessedAuctionHook_MaxSegmentsLimit(t *testing.T) {
	// Create 10 segments but limit to 3
	segments := make([]SegmentData, 10)
	for i := range segments {
		segments[i] = SegmentData{ID: "seg-" + string(rune('0'+i))}
	}

	fetcher := &mockFetcher{segments: segments}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL: "https://example.com/api",
		MaxSegments: 3,
	})

	payload := newTestPayload("https://example.com/page")
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	assert.Equal(t, 3, metrics.segmentsAdded)
	assert.Len(t, result.ChangeSet.Mutations(), 1)
}

func TestHandleProcessedAuctionHook_MutationApplied(t *testing.T) {
	segments := []SegmentData{
		{ID: "seg-1", Name: "Sports"},
		{ID: "seg-2", Name: "Tech"},
	}
	fetcher := &mockFetcher{segments: segments}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL:     "https://example.com/api",
		SegmentDataName: "test-provider",
	})

	payload := newTestPayload("https://example.com/page")
	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)

	require.NoError(t, err)
	require.Len(t, result.ChangeSet.Mutations(), 1)

	// Apply the mutation and verify the result
	mutatedPayload, err := result.ChangeSet.Mutations()[0].Apply(payload)
	require.NoError(t, err)

	bidRequest := mutatedPayload.Request.BidRequest
	require.NotNil(t, bidRequest.User)
	require.Len(t, bidRequest.User.Data, 1)
	assert.Equal(t, "test-provider", bidRequest.User.Data[0].ID)
	assert.Equal(t, "test-provider", bidRequest.User.Data[0].Name)
	require.Len(t, bidRequest.User.Data[0].Segment, 2)
	assert.Equal(t, "seg-1", bidRequest.User.Data[0].Segment[0].ID)
	assert.Equal(t, "Sports", bidRequest.User.Data[0].Segment[0].Name)
	assert.Equal(t, "seg-2", bidRequest.User.Data[0].Segment[1].ID)
}

func TestHandleProcessedAuctionHook_MergesWithExistingUserData(t *testing.T) {
	segments := []SegmentData{
		{ID: "new-seg-1", Name: "New Segment"},
	}
	fetcher := &mockFetcher{segments: segments}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL:     "https://example.com/api",
		SegmentDataName: "test-provider",
	})

	// Payload with existing user data from a different provider
	bidRequest := &openrtb2.BidRequest{
		Site: &openrtb2.Site{Page: "https://example.com/page"},
		User: &openrtb2.User{
			Data: []openrtb2.Data{
				{
					ID:   "other-provider",
					Name: "Other Provider",
					Segment: []openrtb2.Segment{
						{ID: "existing-seg"},
					},
				},
			},
		},
	}
	payload := hookstage.ProcessedAuctionRequestPayload{
		Request: &openrtb_ext.RequestWrapper{BidRequest: bidRequest},
	}

	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)
	require.NoError(t, err)
	require.Len(t, result.ChangeSet.Mutations(), 1)

	mutatedPayload, err := result.ChangeSet.Mutations()[0].Apply(payload)
	require.NoError(t, err)

	// Should have both providers' data
	require.Len(t, mutatedPayload.Request.BidRequest.User.Data, 2)
}

func TestHandleProcessedAuctionHook_ReplacesExistingProviderData(t *testing.T) {
	segments := []SegmentData{
		{ID: "new-seg-1", Name: "Updated Segment"},
	}
	fetcher := &mockFetcher{segments: segments}
	metrics := &mockMetrics{}
	module := newTestModule(fetcher, metrics, Config{
		ExternalURL:     "https://example.com/api",
		SegmentDataName: "test-provider",
	})

	// Payload with existing data from the same provider
	bidRequest := &openrtb2.BidRequest{
		Site: &openrtb2.Site{Page: "https://example.com/page"},
		User: &openrtb2.User{
			Data: []openrtb2.Data{
				{
					ID:   "test-provider",
					Name: "test-provider",
					Segment: []openrtb2.Segment{
						{ID: "old-seg"},
					},
				},
			},
		},
	}
	payload := hookstage.ProcessedAuctionRequestPayload{
		Request: &openrtb_ext.RequestWrapper{BidRequest: bidRequest},
	}

	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	result, err := module.HandleProcessedAuctionHook(ctx, miCtx, payload)
	require.NoError(t, err)
	require.Len(t, result.ChangeSet.Mutations(), 1)

	mutatedPayload, err := result.ChangeSet.Mutations()[0].Apply(payload)
	require.NoError(t, err)

	// Should still have only one provider's data (replaced)
	require.Len(t, mutatedPayload.Request.BidRequest.User.Data, 1)
	assert.Equal(t, "new-seg-1", mutatedPayload.Request.BidRequest.User.Data[0].Segment[0].ID)
}

// ---- extractPageURL Tests ----

func TestExtractPageURL_SitePage(t *testing.T) {
	bidRequest := &openrtb2.BidRequest{
		Site: &openrtb2.Site{
			Page: "https://example.com/article",
		},
	}
	assert.Equal(t, "https://example.com/article", extractPageURL(bidRequest))
}

func TestExtractPageURL_AppBundle(t *testing.T) {
	bidRequest := &openrtb2.BidRequest{
		App: &openrtb2.App{
			Bundle: "com.example.app",
		},
	}
	assert.Equal(t, "com.example.app", extractPageURL(bidRequest))
}

func TestExtractPageURL_SitePreferredOverApp(t *testing.T) {
	bidRequest := &openrtb2.BidRequest{
		Site: &openrtb2.Site{
			Page: "https://example.com/page",
		},
		App: &openrtb2.App{
			Bundle: "com.example.app",
		},
	}
	assert.Equal(t, "https://example.com/page", extractPageURL(bidRequest))
}

func TestExtractPageURL_NoURL(t *testing.T) {
	bidRequest := &openrtb2.BidRequest{
		Site: &openrtb2.Site{
			Domain: "example.com",
		},
	}
	assert.Equal(t, "", extractPageURL(bidRequest))
}

func TestExtractPageURL_NilSiteAndApp(t *testing.T) {
	bidRequest := &openrtb2.BidRequest{}
	assert.Equal(t, "", extractPageURL(bidRequest))
}

// ---- mergeUserData Tests ----

func TestMergeUserData_EmptyExisting(t *testing.T) {
	newData := openrtb2.Data{
		ID:   "provider-1",
		Name: "Provider 1",
		Segment: []openrtb2.Segment{
			{ID: "seg-1"},
		},
	}

	result := mergeUserData(nil, newData)

	require.Len(t, result, 1)
	assert.Equal(t, "provider-1", result[0].ID)
}

func TestMergeUserData_AddsNewProvider(t *testing.T) {
	existing := []openrtb2.Data{
		{ID: "provider-1", Name: "Provider 1"},
	}
	newData := openrtb2.Data{
		ID:   "provider-2",
		Name: "Provider 2",
	}

	result := mergeUserData(existing, newData)

	require.Len(t, result, 2)
	assert.Equal(t, "provider-1", result[0].ID)
	assert.Equal(t, "provider-2", result[1].ID)
}

func TestMergeUserData_ReplacesExistingProvider(t *testing.T) {
	existing := []openrtb2.Data{
		{
			ID:   "provider-1",
			Name: "Provider 1",
			Segment: []openrtb2.Segment{
				{ID: "old-seg"},
			},
		},
	}
	newData := openrtb2.Data{
		ID:   "provider-1",
		Name: "Provider 1",
		Segment: []openrtb2.Segment{
			{ID: "new-seg"},
		},
	}

	result := mergeUserData(existing, newData)

	require.Len(t, result, 1)
	require.Len(t, result[0].Segment, 1)
	assert.Equal(t, "new-seg", result[0].Segment[0].ID)
}

// ---- enrichBidRequestWithSegments Tests ----

func TestEnrichBidRequestWithSegments_CreatesUserIfNil(t *testing.T) {
	segments := []SegmentData{
		{ID: "seg-1", Name: "Sports"},
	}
	cfg := Config{
		SegmentDataName: "test-provider",
	}

	bidRequest := &openrtb2.BidRequest{
		Site: &openrtb2.Site{Page: "https://example.com"},
		// No User
	}
	payload := hookstage.ProcessedAuctionRequestPayload{
		Request: &openrtb_ext.RequestWrapper{BidRequest: bidRequest},
	}

	result, err := enrichBidRequestWithSegments(payload, segments, cfg)

	require.NoError(t, err)
	require.NotNil(t, result.Request.BidRequest.User)
	require.Len(t, result.Request.BidRequest.User.Data, 1)
	assert.Equal(t, "test-provider", result.Request.BidRequest.User.Data[0].ID)
}

func TestEnrichBidRequestWithSegments_NilPayloadRequest(t *testing.T) {
	segments := []SegmentData{{ID: "seg-1"}}
	cfg := Config{SegmentDataName: "test-provider"}

	payload := hookstage.ProcessedAuctionRequestPayload{Request: nil}

	result, err := enrichBidRequestWithSegments(payload, segments, cfg)

	require.NoError(t, err)
	assert.Nil(t, result.Request)
}
