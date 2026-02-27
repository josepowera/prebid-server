// Package urlsegmentenricher implements a Prebid Server module that enriches
// bid requests with audience segments fetched from an external URL based on
// the page URL from the bid request.
package urlsegmentenricher

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/prebid/prebid-server/v3/util/jsonutil"
)

const (
	// DefaultTimeoutMs is the default timeout for external URL calls in milliseconds.
	DefaultTimeoutMs = 500
	// DefaultMaxSegments is the default maximum number of segments to add.
	DefaultMaxSegments = 100
	// DefaultSegmentTaxonomy is the default segment taxonomy value.
	DefaultSegmentTaxonomy = 4
)

// Config holds the module configuration loaded from the Prebid Server config file.
// All fields are configurable via the hooks.modules.prebid.urlsegmentenricher section.
type Config struct {
	// Enabled controls whether the module is active.
	Enabled bool `json:"enabled" mapstructure:"enabled"`

	// ExternalURL is the endpoint to call for segment enrichment.
	// The page URL from the bid request will be appended as a query parameter.
	ExternalURL string `json:"external_url" mapstructure:"external_url"`

	// TimeoutMs is the maximum time in milliseconds to wait for the external URL response.
	// Defaults to 500ms if not set.
	TimeoutMs int `json:"timeout_ms" mapstructure:"timeout_ms"`

	// URLQueryParam is the query parameter name used to pass the page URL to the external service.
	// Defaults to "url" if not set.
	URLQueryParam string `json:"url_query_param" mapstructure:"url_query_param"`

	// SegmentTaxonomy is the IAB taxonomy value to use when adding segments.
	// Defaults to 4 (IAB Audience Taxonomy 1.1) if not set.
	SegmentTaxonomy int `json:"segment_taxonomy" mapstructure:"segment_taxonomy"`

	// MaxSegments is the maximum number of segments to add to the bid request.
	// Defaults to 100 if not set. Set to 0 for unlimited.
	MaxSegments int `json:"max_segments" mapstructure:"max_segments"`

	// AdditionalHeaders is a map of additional HTTP headers to send with the external URL request.
	AdditionalHeaders map[string]string `json:"additional_headers" mapstructure:"additional_headers"`

	// SegmentDataName is the name to use for the segment data provider in the bid request.
	// Defaults to "urlsegmentenricher" if not set.
	SegmentDataName string `json:"segment_data_name" mapstructure:"segment_data_name"`
}

// parseConfig parses the module configuration from raw JSON.
func parseConfig(data json.RawMessage) (Config, error) {
	var cfg Config
	if err := jsonutil.UnmarshalValid(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

// validate checks that the configuration is valid.
func (c *Config) validate() error {
	if c.ExternalURL == "" {
		return errors.New("external_url is required")
	}
	if c.TimeoutMs < 0 {
		return errors.New("timeout_ms must be non-negative")
	}
	if c.MaxSegments < 0 {
		return errors.New("max_segments must be non-negative")
	}
	return nil
}

// applyDefaults sets default values for unset configuration fields.
func (c *Config) applyDefaults() {
	if c.TimeoutMs == 0 {
		c.TimeoutMs = DefaultTimeoutMs
	}
	if c.URLQueryParam == "" {
		c.URLQueryParam = "url"
	}
	if c.SegmentTaxonomy == 0 {
		c.SegmentTaxonomy = DefaultSegmentTaxonomy
	}
	if c.MaxSegments == 0 {
		c.MaxSegments = DefaultMaxSegments
	}
	if c.SegmentDataName == "" {
		c.SegmentDataName = "urlsegmentenricher"
	}
}
