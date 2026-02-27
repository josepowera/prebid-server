# URL Segment Enricher Module

A Prebid Server module that enriches bid requests with audience segments fetched from an external URL based on the page URL from the bid request.

## Overview

This module hooks into the **processed auction request** stage to:

1. Extract the page URL from the bid request (`site.page` for web, `app.bundle` for mobile)
2. Call a configurable external URL with the page URL as a query parameter
3. Parse the JSON response containing segment data
4. Add the segments to the bid request's `user.data` section following the OpenRTB 2.x specification

## Configuration

Add the following to your Prebid Server configuration file (e.g., `pbs.yaml`):

```yaml
hooks:
  enabled: true
  modules:
    prebid:
      urlsegmentenricher:
        enabled: true
        external_url: "https://segments.example.com/api/v1/segments"
        timeout_ms: 500
        url_query_param: "url"
        segment_taxonomy: 4
        max_segments: 100
        segment_data_name: "urlsegmentenricher"
        additional_headers:
          Authorization: "Bearer your-api-token"
  host_execution_plan:
    endpoints:
      /openrtb2/auction:
        stages:
          processed-auction-request:
            groups:
              - timeout: 500
                hook_sequence:
                  - module_code: prebid.urlsegmentenricher
                    hook_impl_code: processed-auction-request
```

### Configuration Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `enabled` | bool | Yes | `false` | Controls whether the module is active |
| `external_url` | string | Yes | - | The endpoint to call for segment enrichment |
| `timeout_ms` | int | No | `500` | Maximum time in milliseconds to wait for the external URL response |
| `url_query_param` | string | No | `"url"` | Query parameter name used to pass the page URL to the external service |
| `segment_taxonomy` | int | No | `4` | IAB taxonomy value for segments (4 = IAB Audience Taxonomy 1.1) |
| `max_segments` | int | No | `100` | Maximum number of segments to add (0 = unlimited) |
| `segment_data_name` | string | No | `"urlsegmentenricher"` | Name/ID for the segment data provider in the bid request |
| `additional_headers` | map | No | `{}` | Additional HTTP headers to send with the external URL request |

## External URL API

The module calls the configured `external_url` with the page URL appended as a query parameter:

```
GET https://segments.example.com/api/v1/segments?url=https%3A%2F%2Fexample.com%2Farticle
```

### Expected Response Format

The external URL must return a JSON response with the following structure:

```json
{
  "segments": [
    {
      "id": "segment-123",
      "name": "Sports Enthusiasts",
      "value": 0.85
    },
    {
      "id": "segment-456",
      "name": "Tech Savvy"
    }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `segments[].id` | string | Yes | The segment identifier |
| `segments[].name` | string | No | Human-readable segment name |
| `segments[].value` | float | No | Numeric value associated with the segment |

## Bid Request Enrichment

The module adds segments to the bid request in the `user.data` array following the OpenRTB 2.x specification:

```json
{
  "user": {
    "data": [
      {
        "id": "urlsegmentenricher",
        "name": "urlsegmentenricher",
        "segment": [
          {"id": "segment-123", "name": "Sports Enthusiasts"},
          {"id": "segment-456", "name": "Tech Savvy"}
        ]
      }
    ]
  }
}
```

If the bid request already contains user data from the same provider (matching by `id`), it will be replaced with the new data.

## Metrics

The module tracks the following internal metrics:

| Metric | Description |
|--------|-------------|
| `TotalRequests` | Total number of hook invocations |
| `SuccessfulFetches` | Number of successful external URL calls |
| `FailedFetches` | Number of failed external URL calls |
| `TimeoutFetches` | Number of external URL calls that timed out |
| `SkippedRequests` | Number of requests skipped (e.g., no page URL) |
| `TotalSegmentsAdded` | Total number of segments added across all requests |
| `AvgFetchDurationMs` | Average fetch duration in milliseconds |

## Error Handling

The module is designed to be non-blocking:

- If the external URL call fails (network error, timeout, non-200 response), the module logs a warning and allows the auction to proceed without enrichment
- If the bid request has no page URL, the module skips enrichment silently
- All errors are reported as warnings in the hook result, not as hard errors

## Development

### Running Tests

```bash
go test ./modules/prebid/urlsegmentenricher/...
```

### Running Tests with Coverage

```bash
go test -cover ./modules/prebid/urlsegmentenricher/...
```
