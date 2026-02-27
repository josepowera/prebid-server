package countryfilter

import (
	"fmt"
	"sort"

	"github.com/prebid/prebid-server/v3/hooks/hookanalytics"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
)

// nbrCodeCountryBlocked is the NBR (No-Bid Reason) code returned when a
// request is rejected because the device country is not in the allowed list.
// 8 = "blocked publisher" in the OpenRTB spec; we reuse it here as the
// closest standard code for a geo-based block.
const nbrCodeCountryBlocked = 8

// handleProcessedAuctionHook contains the core filtering logic.
// It is a standalone function so it can be unit-tested without constructing
// a full Module.
//
// allowedCountries is the set of permitted ISO 3166-1 alpha-3 country codes
// built from the module configuration at startup.
func handleProcessedAuctionHook(
	allowedCountries map[string]struct{},
	payload hookstage.ProcessedAuctionRequestPayload,
) (hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload], error) {
	result := hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{}
	result.AnalyticsTags = newCountryFilterActivity()

	if payload.Request == nil || payload.Request.BidRequest == nil {
		result.AnalyticsTags.Activities[0].Status = hookanalytics.ActivityStatusError
		result.Errors = append(result.Errors, "countryfilter: nil bid request in payload")
		return result, nil
	}

	bidReq := payload.Request.BidRequest

	// No device information – we cannot determine the country, so we allow
	// the request through rather than blocking legitimate traffic.
	if bidReq.Device == nil {
		addAllowedAnalyticTag(&result, "")
		return result, nil
	}

	// Device.Geo may also be nil.
	if bidReq.Device.Geo == nil {
		addAllowedAnalyticTag(&result, "")
		return result, nil
	}

	country := bidReq.Device.Geo.Country

	if _, allowed := allowedCountries[country]; !allowed {
		result.Reject = true
		result.NbrCode = nbrCodeCountryBlocked
		result.Message = fmt.Sprintf(
			"countryfilter: request rejected, device country %q is not in the allowed list %v",
			country, sortedKeys(allowedCountries),
		)
		addBlockedAnalyticTag(&result, country)
	} else {
		addAllowedAnalyticTag(&result, country)
	}

	return result, nil
}

// sortedKeys returns the keys of a string set as a sorted slice,
// used only for human-readable messages.
func sortedKeys(set map[string]struct{}) []string {
	list := make([]string, 0, len(set))
	for k := range set {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}
