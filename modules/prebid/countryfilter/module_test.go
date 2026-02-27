package countryfilter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/hooks/hookanalytics"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

func TestBuilder(t *testing.T) {
	t.Run("valid config with allowed countries", func(t *testing.T) {
		rawCfg := json.RawMessage(`{"allowed_countries":["SVN","CRO"]}`)
		module, err := Builder(rawCfg, moduledeps.ModuleDeps{})
		require.NoError(t, err)
		m, ok := module.(Module)
		require.True(t, ok, "Builder must return a Module")
		assert.Contains(t, m.allowedCountries, "SVN")
		assert.Contains(t, m.allowedCountries, "CRO")
		assert.Len(t, m.allowedCountries, 2)
	})

	t.Run("empty allowed countries list", func(t *testing.T) {
		rawCfg := json.RawMessage(`{"allowed_countries":[]}`)
		module, err := Builder(rawCfg, moduledeps.ModuleDeps{})
		require.NoError(t, err)
		m, ok := module.(Module)
		require.True(t, ok)
		assert.Empty(t, m.allowedCountries)
	})

	t.Run("invalid JSON config returns error", func(t *testing.T) {
		rawCfg := json.RawMessage(`not-valid-json`)
		_, err := Builder(rawCfg, moduledeps.ModuleDeps{})
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// newConfig
// ---------------------------------------------------------------------------

func TestNewConfig(t *testing.T) {
	t.Run("parses allowed_countries correctly", func(t *testing.T) {
		raw := json.RawMessage(`{"allowed_countries":["SVN","CRO","USA"]}`)
		cfg, err := newConfig(raw)
		require.NoError(t, err)
		assert.Equal(t, []string{"SVN", "CRO", "USA"}, cfg.AllowedCountries)
	})

	t.Run("empty JSON object yields empty slice", func(t *testing.T) {
		raw := json.RawMessage(`{}`)
		cfg, err := newConfig(raw)
		require.NoError(t, err)
		assert.Empty(t, cfg.AllowedCountries)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := newConfig(json.RawMessage(`{bad`))
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// handleProcessedAuctionHook (unit-level)
// ---------------------------------------------------------------------------

func TestHandleProcessedAuctionHook(t *testing.T) {
	allowedSet := map[string]struct{}{
		"SVN": {},
		"CRO": {},
	}

	tests := []struct {
		name                string
		payload             hookstage.ProcessedAuctionRequestPayload
		wantReject          bool
		wantNbrCode         int
		wantErrors          []string
		wantMsgContain      string
		wantAnalyticStatus  hookanalytics.ResultStatus
		wantAnalyticCountry string
		wantActivityStatus  hookanalytics.ActivityStatus
	}{
		{
			name: "allowed country SVN – request passes",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: &openrtb2.Device{
						Geo: &openrtb2.Geo{Country: "SVN"},
					},
				}),
			},
			wantReject:          false,
			wantNbrCode:         0,
			wantAnalyticStatus:  hookanalytics.ResultStatusAllow,
			wantAnalyticCountry: "SVN",
			wantActivityStatus:  hookanalytics.ActivityStatusSuccess,
		},
		{
			name: "allowed country CRO – request passes",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: &openrtb2.Device{
						Geo: &openrtb2.Geo{Country: "CRO"},
					},
				}),
			},
			wantReject:          false,
			wantNbrCode:         0,
			wantAnalyticStatus:  hookanalytics.ResultStatusAllow,
			wantAnalyticCountry: "CRO",
			wantActivityStatus:  hookanalytics.ActivityStatusSuccess,
		},
		{
			name: "disallowed country USA – request rejected",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: &openrtb2.Device{
						Geo: &openrtb2.Geo{Country: "USA"},
					},
				}),
			},
			wantReject:          true,
			wantNbrCode:         nbrCodeCountryBlocked,
			wantMsgContain:      "USA",
			wantAnalyticStatus:  hookanalytics.ResultStatusBlock,
			wantAnalyticCountry: "USA",
			wantActivityStatus:  hookanalytics.ActivityStatusSuccess,
		},
		{
			name: "disallowed country DEU – request rejected",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: &openrtb2.Device{
						Geo: &openrtb2.Geo{Country: "DEU"},
					},
				}),
			},
			wantReject:          true,
			wantNbrCode:         nbrCodeCountryBlocked,
			wantMsgContain:      "DEU",
			wantAnalyticStatus:  hookanalytics.ResultStatusBlock,
			wantAnalyticCountry: "DEU",
			wantActivityStatus:  hookanalytics.ActivityStatusSuccess,
		},
		{
			name: "empty country string – request rejected",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: &openrtb2.Device{
						Geo: &openrtb2.Geo{Country: ""},
					},
				}),
			},
			wantReject:          true,
			wantNbrCode:         nbrCodeCountryBlocked,
			wantAnalyticStatus:  hookanalytics.ResultStatusBlock,
			wantAnalyticCountry: "",
			wantActivityStatus:  hookanalytics.ActivityStatusSuccess,
		},
		{
			name: "nil Device – request passes (cannot determine country)",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: nil,
				}),
			},
			wantReject:          false,
			wantNbrCode:         0,
			wantAnalyticStatus:  hookanalytics.ResultStatusAllow,
			wantAnalyticCountry: "",
			wantActivityStatus:  hookanalytics.ActivityStatusSuccess,
		},
		{
			name: "nil Device.Geo – request passes (cannot determine country)",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: &openrtb2.Device{Geo: nil},
				}),
			},
			wantReject:          false,
			wantNbrCode:         0,
			wantAnalyticStatus:  hookanalytics.ResultStatusAllow,
			wantAnalyticCountry: "",
			wantActivityStatus:  hookanalytics.ActivityStatusSuccess,
		},
		{
			name: "nil BidRequest – returns error in result, no rejection",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: nil,
			},
			wantReject:         false,
			wantErrors:         []string{"countryfilter: nil bid request in payload"},
			wantActivityStatus: hookanalytics.ActivityStatusError,
		},
		{
			name: "nil RequestWrapper.BidRequest – returns error in result, no rejection",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: &openrtb_ext.RequestWrapper{BidRequest: nil},
			},
			wantReject:         false,
			wantErrors:         []string{"countryfilter: nil bid request in payload"},
			wantActivityStatus: hookanalytics.ActivityStatusError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := handleProcessedAuctionHook(allowedSet, tc.payload)

			require.NoError(t, err, "handleProcessedAuctionHook must not return a Go error")
			assert.Equal(t, tc.wantReject, result.Reject)
			assert.Equal(t, tc.wantNbrCode, result.NbrCode)

			if len(tc.wantErrors) > 0 {
				assert.Equal(t, tc.wantErrors, result.Errors)
			} else {
				assert.Empty(t, result.Errors)
			}

			if tc.wantMsgContain != "" {
				assert.Contains(t, result.Message, tc.wantMsgContain)
			}

			// Verify analytics tags are always populated.
			require.Len(t, result.AnalyticsTags.Activities, 1, "expected exactly one analytics activity")
			activity := result.AnalyticsTags.Activities[0]
			assert.Equal(t, activityCountryFilter, activity.Name)
			assert.Equal(t, tc.wantActivityStatus, activity.Status)

			// For non-error cases, verify the result entry.
			if tc.wantActivityStatus != hookanalytics.ActivityStatusError {
				require.Len(t, activity.Results, 1, "expected exactly one analytics result")
				analyticsResult := activity.Results[0]
				assert.Equal(t, tc.wantAnalyticStatus, analyticsResult.Status)
				assert.True(t, analyticsResult.AppliedTo.Request)
				assert.Equal(t, tc.wantAnalyticCountry, analyticsResult.Values[analyticsKeyCountry])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Module.HandleProcessedAuctionHook (integration-level)
// ---------------------------------------------------------------------------

func TestModuleHandleProcessedAuctionHook(t *testing.T) {
	rawCfg := json.RawMessage(`{"allowed_countries":["SVN","CRO"]}`)
	iface, err := Builder(rawCfg, moduledeps.ModuleDeps{})
	require.NoError(t, err)
	m := iface.(Module)

	ctx := context.Background()
	miCtx := hookstage.ModuleInvocationContext{}

	t.Run("allowed country SVN passes through module", func(t *testing.T) {
		payload := hookstage.ProcessedAuctionRequestPayload{
			Request: wrapRequest(&openrtb2.BidRequest{
				Device: &openrtb2.Device{
					Geo: &openrtb2.Geo{Country: "SVN"},
				},
			}),
		}
		result, err := m.HandleProcessedAuctionHook(ctx, miCtx, payload)
		require.NoError(t, err)
		assert.False(t, result.Reject)
		require.Len(t, result.AnalyticsTags.Activities, 1)
		assert.Equal(t, hookanalytics.ActivityStatusSuccess, result.AnalyticsTags.Activities[0].Status)
		require.Len(t, result.AnalyticsTags.Activities[0].Results, 1)
		assert.Equal(t, hookanalytics.ResultStatusAllow, result.AnalyticsTags.Activities[0].Results[0].Status)
	})

	t.Run("disallowed country GBR rejected by module", func(t *testing.T) {
		payload := hookstage.ProcessedAuctionRequestPayload{
			Request: wrapRequest(&openrtb2.BidRequest{
				Device: &openrtb2.Device{
					Geo: &openrtb2.Geo{Country: "GBR"},
				},
			}),
		}
		result, err := m.HandleProcessedAuctionHook(ctx, miCtx, payload)
		require.NoError(t, err)
		assert.True(t, result.Reject)
		assert.Equal(t, nbrCodeCountryBlocked, result.NbrCode)
		assert.Contains(t, result.Message, "GBR")
		require.Len(t, result.AnalyticsTags.Activities, 1)
		assert.Equal(t, hookanalytics.ActivityStatusSuccess, result.AnalyticsTags.Activities[0].Status)
		require.Len(t, result.AnalyticsTags.Activities[0].Results, 1)
		assert.Equal(t, hookanalytics.ResultStatusBlock, result.AnalyticsTags.Activities[0].Results[0].Status)
		assert.Equal(t, "GBR", result.AnalyticsTags.Activities[0].Results[0].Values[analyticsKeyCountry])
	})

	t.Run("different allowed countries set from config", func(t *testing.T) {
		rawCfg2 := json.RawMessage(`{"allowed_countries":["USA","GBR"]}`)
		iface2, err := Builder(rawCfg2, moduledeps.ModuleDeps{})
		require.NoError(t, err)
		m2 := iface2.(Module)

		// USA is now allowed
		payload := hookstage.ProcessedAuctionRequestPayload{
			Request: wrapRequest(&openrtb2.BidRequest{
				Device: &openrtb2.Device{
					Geo: &openrtb2.Geo{Country: "USA"},
				},
			}),
		}
		result, err := m2.HandleProcessedAuctionHook(ctx, miCtx, payload)
		require.NoError(t, err)
		assert.False(t, result.Reject, "USA should be allowed with the second config")
		assert.Equal(t, hookanalytics.ResultStatusAllow, result.AnalyticsTags.Activities[0].Results[0].Status)

		// SVN is now blocked
		payload2 := hookstage.ProcessedAuctionRequestPayload{
			Request: wrapRequest(&openrtb2.BidRequest{
				Device: &openrtb2.Device{
					Geo: &openrtb2.Geo{Country: "SVN"},
				},
			}),
		}
		result2, err := m2.HandleProcessedAuctionHook(ctx, miCtx, payload2)
		require.NoError(t, err)
		assert.True(t, result2.Reject, "SVN should be blocked with the second config")
		assert.Equal(t, hookanalytics.ResultStatusBlock, result2.AnalyticsTags.Activities[0].Results[0].Status)
	})
}

// ---------------------------------------------------------------------------
// Module implements hookstage.ProcessedAuctionRequest interface
// ---------------------------------------------------------------------------

func TestModuleImplementsProcessedAuctionRequestInterface(t *testing.T) {
	var _ hookstage.ProcessedAuctionRequest = Module{}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// wrapRequest wraps an openrtb2.BidRequest in an openrtb_ext.RequestWrapper.
func wrapRequest(req *openrtb2.BidRequest) *openrtb_ext.RequestWrapper {
	return &openrtb_ext.RequestWrapper{BidRequest: req}
}
