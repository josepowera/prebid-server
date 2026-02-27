package urlsegmentenricher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- HTTPSegmentFetcher Tests ----

func TestHTTPSegmentFetcher_Success(t *testing.T) {
	// Set up a test HTTP server that returns a valid segment response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the query parameter is set
		assert.Equal(t, "https://example.com/page", r.URL.Query().Get("url"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := SegmentResponse{
			Segments: []SegmentData{
				{ID: "seg-1", Name: "Sports", Value: 0.9},
				{ID: "seg-2", Name: "Tech"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     1000,
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	segments, err := fetcher.FetchSegments(context.Background(), "https://example.com/page")

	require.NoError(t, err)
	require.Len(t, segments, 2)
	assert.Equal(t, "seg-1", segments[0].ID)
	assert.Equal(t, "Sports", segments[0].Name)
	assert.Equal(t, 0.9, segments[0].Value)
	assert.Equal(t, "seg-2", segments[1].ID)
}

func TestHTTPSegmentFetcher_EmptyPageURL(t *testing.T) {
	cfg := Config{
		ExternalURL:   "https://example.com/api",
		TimeoutMs:     1000,
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	segments, err := fetcher.FetchSegments(context.Background(), "")

	require.NoError(t, err)
	assert.Nil(t, segments)
}

func TestHTTPSegmentFetcher_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     1000,
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	segments, err := fetcher.FetchSegments(context.Background(), "https://example.com/page")

	assert.Error(t, err)
	assert.Nil(t, segments)
	assert.Contains(t, err.Error(), "non-200 status: 500")
}

func TestHTTPSegmentFetcher_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     1000,
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	segments, err := fetcher.FetchSegments(context.Background(), "https://example.com/page")

	assert.Error(t, err)
	assert.Nil(t, segments)
	assert.Contains(t, err.Error(), "failed to parse segment response")
}

func TestHTTPSegmentFetcher_Timeout(t *testing.T) {
	// Server that delays longer than the timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"segments":[]}`))
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     50, // 50ms timeout, server delays 200ms
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	ctx := context.Background()
	segments, err := fetcher.FetchSegments(ctx, "https://example.com/page")

	assert.Error(t, err)
	assert.Nil(t, segments)
}

func TestHTTPSegmentFetcher_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     5000,
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancel()

	segments, err := fetcher.FetchSegments(ctx, "https://example.com/page")

	assert.Error(t, err)
	assert.Nil(t, segments)
}

func TestHTTPSegmentFetcher_AdditionalHeaders(t *testing.T) {
	var receivedAuthHeader string
	var receivedCustomHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedCustomHeader = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"segments":[]}`))
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     1000,
		URLQueryParam: "url",
		AdditionalHeaders: map[string]string{
			"Authorization":   "Bearer test-token",
			"X-Custom-Header": "custom-value",
		},
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	_, err := fetcher.FetchSegments(context.Background(), "https://example.com/page")

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token", receivedAuthHeader)
	assert.Equal(t, "custom-value", receivedCustomHeader)
}

func TestHTTPSegmentFetcher_CustomQueryParam(t *testing.T) {
	var receivedQueryParam string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQueryParam = r.URL.Query().Get("page_url")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"segments":[]}`))
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     1000,
		URLQueryParam: "page_url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	_, err := fetcher.FetchSegments(context.Background(), "https://example.com/page")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/page", receivedQueryParam)
}

func TestHTTPSegmentFetcher_EmptySegmentsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"segments":[]}`))
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     1000,
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	segments, err := fetcher.FetchSegments(context.Background(), "https://example.com/page")

	require.NoError(t, err)
	assert.Empty(t, segments)
}

func TestHTTPSegmentFetcher_NilSegmentsInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := Config{
		ExternalURL:   server.URL,
		TimeoutMs:     1000,
		URLQueryParam: "url",
	}
	fetcher := NewHTTPSegmentFetcher(http.DefaultClient, cfg)

	segments, err := fetcher.FetchSegments(context.Background(), "https://example.com/page")

	require.NoError(t, err)
	assert.Nil(t, segments)
}

// ---- buildRequestURL Tests ----

func TestBuildRequestURL_SimpleURL(t *testing.T) {
	result, err := buildRequestURL("https://example.com/api", "url", "https://page.com/article")

	require.NoError(t, err)
	assert.Contains(t, result, "url=https%3A%2F%2Fpage.com%2Farticle")
}

func TestBuildRequestURL_ExistingQueryParams(t *testing.T) {
	result, err := buildRequestURL("https://example.com/api?key=value", "url", "https://page.com/article")

	require.NoError(t, err)
	assert.Contains(t, result, "key=value")
	assert.Contains(t, result, "url=https%3A%2F%2Fpage.com%2Farticle")
}

func TestBuildRequestURL_InvalidBaseURL(t *testing.T) {
	_, err := buildRequestURL("://invalid-url", "url", "https://page.com/article")

	assert.Error(t, err)
}

func TestBuildRequestURL_SpecialCharactersInPageURL(t *testing.T) {
	pageURL := "https://example.com/page?q=hello world&lang=en"
	result, err := buildRequestURL("https://api.example.com/segments", "url", pageURL)

	require.NoError(t, err)
	// The page URL should be properly encoded
	assert.NotContains(t, result, "hello world")
}
