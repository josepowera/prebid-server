package countryfilter

import (
	"github.com/prebid/prebid-server/v3/hooks/hookanalytics"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
)

const (
	// activityCountryFilter is the name of the single activity reported by this module.
	activityCountryFilter = "country_filter"

	// analyticsKeyCountry is the key used in the analytics result values map to
	// record the device country that was evaluated.
	analyticsKeyCountry = "country"
)

// newCountryFilterActivity returns an Analytics struct with a single activity
// pre-populated with a success status. The caller should update the status and
// append a Result before attaching it to the HookResult.
func newCountryFilterActivity() hookanalytics.Analytics {
	return hookanalytics.Analytics{
		Activities: []hookanalytics.Activity{
			{
				Name:   activityCountryFilter,
				Status: hookanalytics.ActivityStatusSuccess,
			},
		},
	}
}

// addAllowedAnalyticTag appends an "allow" result for the given country to the
// hook result's analytics tags.
func addAllowedAnalyticTag(
	result *hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload],
	country string,
) {
	result.AnalyticsTags.Activities[0].Results = append(
		result.AnalyticsTags.Activities[0].Results,
		hookanalytics.Result{
			Status: hookanalytics.ResultStatusAllow,
			Values: map[string]interface{}{
				analyticsKeyCountry: country,
			},
			AppliedTo: hookanalytics.AppliedTo{Request: true},
		},
	)
}

// addBlockedAnalyticTag appends a "block" result for the given country to the
// hook result's analytics tags.
func addBlockedAnalyticTag(
	result *hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload],
	country string,
) {
	result.AnalyticsTags.Activities[0].Results = append(
		result.AnalyticsTags.Activities[0].Results,
		hookanalytics.Result{
			Status: hookanalytics.ResultStatusBlock,
			Values: map[string]interface{}{
				analyticsKeyCountry: country,
			},
			AppliedTo: hookanalytics.AppliedTo{Request: true},
		},
	)
}
