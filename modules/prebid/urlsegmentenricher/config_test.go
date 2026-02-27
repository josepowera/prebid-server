package urlsegmentenricher

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig_ValidFull(t *testing.T) {
	rawCfg := json.RawMessage(`{
		"enabled": true,
		"external_url": "https://segments.example.com/api",
		"timeout_ms": 300,
		"url_query_param": "page",
		"segment_taxonomy": 4,
		"max_segments": 50,
		"segment_data_name": "my-provider",
		"additional_headers": {
			"Authorization": "Bearer token",
			"X-Custom": "value"
		}
	}`)

	cfg, err := parseConfig(rawCfg)

	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "https://segments.example.com/api", cfg.ExternalURL)
	assert.Equal(t, 300, cfg.TimeoutMs)
	assert.Equal(t, "page", cfg.URLQueryParam)
	assert.Equal(t, 4, cfg.SegmentTaxonomy)
	assert.Equal(t, 50, cfg.MaxSegments)
	assert.Equal(t, "my-provider", cfg.SegmentDataName)
	assert.Equal(t, "Bearer token", cfg.AdditionalHeaders["Authorization"])
	assert.Equal(t, "value", cfg.AdditionalHeaders["X-Custom"])
}

func TestParseConfig_MinimalConfig(t *testing.T) {
	rawCfg := json.RawMessage(`{
		"enabled": true,
		"external_url": "https://segments.example.com/api"
	}`)

	cfg, err := parseConfig(rawCfg)

	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "https://segments.example.com/api", cfg.ExternalURL)
	// Other fields should be zero values
	assert.Equal(t, 0, cfg.TimeoutMs)
	assert.Equal(t, "", cfg.URLQueryParam)
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	rawCfg := json.RawMessage(`{invalid json}`)

	_, err := parseConfig(rawCfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}

func TestParseConfig_EmptyJSON(t *testing.T) {
	rawCfg := json.RawMessage(`{}`)

	cfg, err := parseConfig(rawCfg)

	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, "", cfg.ExternalURL)
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := Config{
		ExternalURL: "https://example.com/api",
		TimeoutMs:   500,
		MaxSegments: 100,
	}

	err := cfg.validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingExternalURL(t *testing.T) {
	cfg := Config{
		TimeoutMs: 500,
	}

	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "external_url is required")
}

func TestConfig_Validate_NegativeTimeout(t *testing.T) {
	cfg := Config{
		ExternalURL: "https://example.com/api",
		TimeoutMs:   -1,
	}

	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout_ms must be non-negative")
}

func TestConfig_Validate_NegativeMaxSegments(t *testing.T) {
	cfg := Config{
		ExternalURL: "https://example.com/api",
		MaxSegments: -1,
	}

	err := cfg.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_segments must be non-negative")
}

func TestConfig_ApplyDefaults_AllDefaults(t *testing.T) {
	cfg := Config{
		ExternalURL: "https://example.com/api",
	}

	cfg.applyDefaults()

	assert.Equal(t, DefaultTimeoutMs, cfg.TimeoutMs)
	assert.Equal(t, "url", cfg.URLQueryParam)
	assert.Equal(t, DefaultSegmentTaxonomy, cfg.SegmentTaxonomy)
	assert.Equal(t, DefaultMaxSegments, cfg.MaxSegments)
	assert.Equal(t, "urlsegmentenricher", cfg.SegmentDataName)
}

func TestConfig_ApplyDefaults_PreservesExistingValues(t *testing.T) {
	cfg := Config{
		ExternalURL:     "https://example.com/api",
		TimeoutMs:       200,
		URLQueryParam:   "page_url",
		SegmentTaxonomy: 6,
		MaxSegments:     50,
		SegmentDataName: "custom-provider",
	}

	cfg.applyDefaults()

	// Existing values should not be overwritten
	assert.Equal(t, 200, cfg.TimeoutMs)
	assert.Equal(t, "page_url", cfg.URLQueryParam)
	assert.Equal(t, 6, cfg.SegmentTaxonomy)
	assert.Equal(t, 50, cfg.MaxSegments)
	assert.Equal(t, "custom-provider", cfg.SegmentDataName)
}

func TestConfig_ApplyDefaults_ZeroMaxSegmentsGetsDefault(t *testing.T) {
	cfg := Config{
		ExternalURL: "https://example.com/api",
		MaxSegments: 0,
	}

	cfg.applyDefaults()

	assert.Equal(t, DefaultMaxSegments, cfg.MaxSegments)
}
