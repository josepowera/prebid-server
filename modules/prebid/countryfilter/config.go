package countryfilter

import (
	"encoding/json"
	"fmt"
)

// config holds the module-level configuration parsed from the
// hooks.modules.prebid.countryfilter section of the Prebid Server config file.
type config struct {
	// AllowedCountries is the list of ISO 3166-1 alpha-3 country codes that are
	// permitted to participate in the auction. Requests from any other country
	// will be rejected at the ProcessedAuctionRequest stage.
	AllowedCountries []string `json:"allowed_countries"`
}

// newConfig parses the raw JSON module configuration into a config struct.
func newConfig(data json.RawMessage) (config, error) {
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("countryfilter: failed to parse module config: %w", err)
	}
	return cfg, nil
}

// toSet converts the AllowedCountries slice into a set (map) for O(1) lookups.
func (c config) toSet() map[string]struct{} {
	set := make(map[string]struct{}, len(c.AllowedCountries))
	for _, country := range c.AllowedCountries {
		set[country] = struct{}{}
	}
	return set
}
