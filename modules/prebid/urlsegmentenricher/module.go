// Package urlsegmentenricher implements a Prebid Server module that enriches
// bid requests with audience segments fetched from an external URL based on
// the page URL from the bid request.
//
// The module hooks into the processed auction request stage to:
//  1. Extract the page URL from the bid request (site.page or app.bundle)
//  2. Call an external URL with the page URL as a query parameter
//  3. Parse the JSON response containing segment data
//  4. Add the segments to the bid request's user data section
//
// Configuration is loaded from the Prebid Server config file under:
//
//	hooks:
//	  modules:
//	    prebid:
//	      urlsegmentenricher:
//	        enabled: true
//	        external_url: "https://segments.example.com/api"
//	        timeout_ms: 500
//	        url_query_param: "url"
//	        segment_taxonomy: 4
//	        max_segments: 100
//	        segment_data_name: "urlsegmentenricher"
//	        additional_headers:
//	          Authorization: "Bearer token"
package urlsegmentenricher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/hooks/hookanalytics"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
)

// Compile-time interface assertions to ensure Module implements the required hook interfaces.
var (
	_ hookstage.ProcessedAuctionRequest = (*Module)(nil)
)

// Builder is the entry point for the module, called by Prebid Server during initialization.
// It reads the module configuration and creates a new Module instance.
func Builder(rawCfg json.RawMessage, deps moduledeps.ModuleDeps) (interface{}, error) {
	cfg, err := parseConfig(rawCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse urlsegmentenricher config: %w", err)
	}

	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid urlsegmentenricher config: %w", err)
	}

	baseClient := deps.HTTPClient
	if baseClient == nil {
		baseClient = http.DefaultClient
	}

	fetcher := NewHTTPSegmentFetcher(baseClient, cfg)

	return &Module{
		cfg:     cfg,
		fetcher: fetcher,
		metrics: &ModuleMetrics{},
	}, nil
}

// Module implements the URL segment enricher Prebid Server module.
type Module struct {
	cfg     Config
	fetcher SegmentFetcher
	metrics MetricsRecorder
}

// HandleProcessedAuctionHook is called after the auction request has been parsed and validated.
// It enriches the bid request with audience segments fetched from the configured external URL.
func (m *Module) HandleProcessedAuctionHook(
	ctx context.Context,
	_ hookstage.ModuleInvocationContext,
	payload hookstage.ProcessedAuctionRequestPayload,
) (hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload], error) {
	result := hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{}

	m.metrics.RecordRequest()

	if payload.Request == nil || payload.Request.BidRequest == nil {
		result.Warnings = append(result.Warnings, "bid request is nil, skipping segment enrichment")
		m.metrics.RecordSkipped()
		return result, nil
	}

	bidRequest := payload.Request.BidRequest

	// Extract the page URL from the bid request
	pageURL := extractPageURL(bidRequest)
	if pageURL == "" {
		result.DebugMessages = append(result.DebugMessages, "no page URL found in bid request, skipping segment enrichment")
		m.metrics.RecordSkipped()
		return result, nil
	}

	// Create a context with the configured timeout
	fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(m.cfg.TimeoutMs)*time.Millisecond)
	defer cancel()

	// Fetch segments from the external URL
	startTime := time.Now()
	segments, err := m.fetcher.FetchSegments(fetchCtx, pageURL)
	fetchDuration := time.Since(startTime)

	if err != nil {
		isTimeout := errors.Is(err, context.DeadlineExceeded) ||
			strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "timeout")

		m.metrics.RecordFetchFailure(isTimeout)

		errMsg := fmt.Sprintf("failed to fetch segments for URL %q: %v", pageURL, err)
		result.Warnings = append(result.Warnings, errMsg)
		result.AnalyticsTags = buildErrorAnalytics("fetch_segments", errMsg)
		// Return without error to not block the auction
		return result, nil
	}

	if len(segments) == 0 {
		m.metrics.RecordFetchSuccess(fetchDuration, 0)
		result.DebugMessages = append(result.DebugMessages,
			fmt.Sprintf("no segments returned for URL %q", pageURL))
		return result, nil
	}

	// Apply max segments limit
	if m.cfg.MaxSegments > 0 && len(segments) > m.cfg.MaxSegments {
		segments = segments[:m.cfg.MaxSegments]
	}

	m.metrics.RecordFetchSuccess(fetchDuration, len(segments))

	// Capture values for the closure
	cfg := m.cfg
	segmentsCopy := make([]SegmentData, len(segments))
	copy(segmentsCopy, segments)

	// Add segments to the bid request via a mutation
	result.ChangeSet.AddMutation(
		func(p hookstage.ProcessedAuctionRequestPayload) (hookstage.ProcessedAuctionRequestPayload, error) {
			return enrichBidRequestWithSegments(p, segmentsCopy, cfg)
		},
		hookstage.MutationUpdate,
		"user", "data",
	)

	result.DebugMessages = append(result.DebugMessages,
		fmt.Sprintf("added %d segments for URL %q (fetch took %dms)",
			len(segments), pageURL, fetchDuration.Milliseconds()))

	return result, nil
}

// extractPageURL extracts the page URL from the bid request.
// For site requests, it uses site.page. For app requests, it uses app.bundle.
func extractPageURL(bidRequest *openrtb2.BidRequest) string {
	if bidRequest.Site != nil && bidRequest.Site.Page != "" {
		return bidRequest.Site.Page
	}
	if bidRequest.App != nil && bidRequest.App.Bundle != "" {
		return bidRequest.App.Bundle
	}
	return ""
}

// enrichBidRequestWithSegments adds the fetched segments to the bid request's user data.
func enrichBidRequestWithSegments(
	payload hookstage.ProcessedAuctionRequestPayload,
	segments []SegmentData,
	cfg Config,
) (hookstage.ProcessedAuctionRequestPayload, error) {
	if payload.Request == nil || payload.Request.BidRequest == nil {
		return payload, nil
	}

	bidRequest := payload.Request.BidRequest

	// Build the OpenRTB2 Data object with segments
	ortbSegments := make([]openrtb2.Segment, 0, len(segments))
	for _, seg := range segments {
		ortbSegments = append(ortbSegments, openrtb2.Segment{
			ID:   seg.ID,
			Name: seg.Name,
		})
	}

	newData := openrtb2.Data{
		ID:      cfg.SegmentDataName,
		Name:    cfg.SegmentDataName,
		Segment: ortbSegments,
	}

	// Ensure user exists
	if bidRequest.User == nil {
		bidRequest.User = &openrtb2.User{}
	}

	// Merge with existing user data, avoiding duplicates by data provider name
	bidRequest.User.Data = mergeUserData(bidRequest.User.Data, newData)

	return payload, nil
}

// mergeUserData merges new segment data into existing user data.
// If a data entry with the same ID already exists, it is replaced.
func mergeUserData(existing []openrtb2.Data, newData openrtb2.Data) []openrtb2.Data {
	result := make([]openrtb2.Data, 0, len(existing)+1)
	replaced := false

	for _, d := range existing {
		if d.ID == newData.ID {
			result = append(result, newData)
			replaced = true
		} else {
			result = append(result, d)
		}
	}

	if !replaced {
		result = append(result, newData)
	}

	return result
}

// buildErrorAnalytics creates an analytics tag for error reporting.
func buildErrorAnalytics(activityName, errMsg string) hookanalytics.Analytics {
	return hookanalytics.Analytics{
		Activities: []hookanalytics.Activity{
			{
				Name:   activityName,
				Status: hookanalytics.ActivityStatusError,
				Results: []hookanalytics.Result{
					{
						Status: hookanalytics.ResultStatusError,
						Values: map[string]interface{}{
							"error": errMsg,
						},
					},
				},
			},
		},
	}
}
