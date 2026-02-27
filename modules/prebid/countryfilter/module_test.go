package countryfilter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prebid/openrtb/v20/openrtb2"
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
		name           string
		payload        hookstage.ProcessedAuctionRequestPayload
		wantReject     bool
		wantNbrCode    int
		wantErrors     []string
		wantMsgContain string
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
			wantReject:  false,
			wantNbrCode: 0,
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
			wantReject:  false,
			wantNbrCode: 0,
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
			wantReject:     true,
			wantNbrCode:    nbrCodeCountryBlocked,
			wantMsgContain: "USA",
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
			wantReject:     true,
			wantNbrCode:    nbrCodeCountryBlocked,
			wantMsgContain: "DEU",
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
			wantReject:  true,
			wantNbrCode: nbrCodeCountryBlocked,
		},
		{
			name: "nil Device – request passes (cannot determine country)",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: nil,
				}),
			},
			wantReject:  false,
			wantNbrCode: 0,
		},
		{
			name: "nil Device.Geo – request passes (cannot determine country)",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: wrapRequest(&openrtb2.BidRequest{
					Device: &openrtb2.Device{Geo: nil},
				}),
			},
			wantReject:  false,
			wantNbrCode: 0,
		},
		{
			name: "nil BidRequest – returns error in result, no rejection",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: nil,
			},
			wantReject: false,
			wantErrors: []string{"countryfilter: nil bid request in payload"},
		},
		{
			name: "nil RequestWrapper.BidRequest – returns error in result, no rejection",
			payload: hookstage.ProcessedAuctionRequestPayload{
				Request: &openrtb_ext.RequestWrapper{BidRequest: nil},
			},
			wantReject: false,
			wantErrors: []string{"countryfilter: nil bid request in payload"},
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
