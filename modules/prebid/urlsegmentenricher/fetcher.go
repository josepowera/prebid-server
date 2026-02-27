package urlsegmentenricher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SegmentResponse represents the JSON response from the external segment enrichment service.
type SegmentResponse struct {
	// Segments is the list of segment IDs returned by the external service.
	Segments []SegmentData `json:"segments"`
}

// SegmentData represents a single segment entry from the external service response.
type SegmentData struct {
	// ID is the segment identifier.
	ID string `json:"id"`
	// Name is the human-readable segment name (optional).
	Name string `json:"name,omitempty"`
	// Value is an optional numeric value associated with the segment.
	Value float64 `json:"value,omitempty"`
}

// SegmentFetcher defines the interface for fetching segments from an external URL.
// This interface allows for easy mocking in tests.
type SegmentFetcher interface {
	FetchSegments(ctx context.Context, pageURL string) ([]SegmentData, error)
}

// HTTPSegmentFetcher implements SegmentFetcher using an HTTP client.
type HTTPSegmentFetcher struct {
	httpClient        *http.Client
	externalURL       string
	urlQueryParam     string
	additionalHeaders map[string]string
	timeoutMs         int
}

// NewHTTPSegmentFetcher creates a new HTTPSegmentFetcher with the given configuration.
func NewHTTPSegmentFetcher(
	baseClient *http.Client,
	cfg Config,
) *HTTPSegmentFetcher {
	// Create a new client with the configured timeout, reusing the transport from the base client
	client := &http.Client{
		Timeout: time.Duration(cfg.TimeoutMs) * time.Millisecond,
	}
	if baseClient != nil && baseClient.Transport != nil {
		client.Transport = baseClient.Transport
	}

	return &HTTPSegmentFetcher{
		httpClient:        client,
		externalURL:       cfg.ExternalURL,
		urlQueryParam:     cfg.URLQueryParam,
		additionalHeaders: cfg.AdditionalHeaders,
		timeoutMs:         cfg.TimeoutMs,
	}
}

// FetchSegments calls the external URL with the page URL as a query parameter
// and returns the parsed segment data.
func (f *HTTPSegmentFetcher) FetchSegments(ctx context.Context, pageURL string) ([]SegmentData, error) {
	if pageURL == "" {
		return nil, nil
	}

	// Build the request URL with the page URL as a query parameter
	reqURL, err := buildRequestURL(f.externalURL, f.urlQueryParam, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build request URL: %w", err)
	}

	// Create the HTTP request with context for cancellation/timeout
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set standard headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Set any additional configured headers
	for key, value := range f.additionalHeaders {
		req.Header.Set(key, value)
	}

	// Execute the request
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call external URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external URL returned non-200 status: %d", resp.StatusCode)
	}

	// Read and parse the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var segmentResp SegmentResponse
	if err := json.Unmarshal(body, &segmentResp); err != nil {
		return nil, fmt.Errorf("failed to parse segment response: %w", err)
	}

	return segmentResp.Segments, nil
}

// buildRequestURL constructs the full request URL by appending the page URL as a query parameter.
func buildRequestURL(baseURL, queryParam, pageURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}

	q := parsed.Query()
	q.Set(queryParam, pageURL)
	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}
