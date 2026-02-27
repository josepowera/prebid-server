package countryfilter

import (
	"context"
	"encoding/json"

	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
)

// allowedCountries is the set of ISO 3166-1 alpha-3 country codes that are
// permitted to participate in the auction. Requests from any other country
// will be rejected at the ProcessedAuctionRequest stage.
var allowedCountries = map[string]struct{}{
	"SVN": {},
	"CRO": {},
}

// Builder returns a new instance of the countryfilter Module.
// It satisfies the modules.ModuleBuilderFn signature.
func Builder(_ json.RawMessage, _ moduledeps.ModuleDeps) (interface{}, error) {
	return Module{}, nil
}

// Module is the country-filter Prebid Server module.
// It implements the hookstage.ProcessedAuctionRequest interface so that it
// is invoked after the bid request has been parsed and enriched.
type Module struct{}

// HandleProcessedAuctionHook rejects any bid request whose device country is
// not in the allowedCountries set.
//
// The hook returns Reject=true together with an NbrCode when the country is
// not allowed, which causes Prebid Server to return an empty BidResponse with
// the supplied NBR code.
func (m Module) HandleProcessedAuctionHook(
	_ context.Context,
	_ hookstage.ModuleInvocationContext,
	payload hookstage.ProcessedAuctionRequestPayload,
) (hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload], error) {
	return handleProcessedAuctionHook(payload)
}
