// Package promclient provides a minimal HTTP client for the Prometheus HTTP API.
package promclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Series holds the data for a single Prometheus time series.
type Series struct {
	// Label is a human-readable summary of the metric labels.
	Label string
	// Labels is the raw metric label set returned by Prometheus.
	Labels     map[string]string
	Timestamps []time.Time
	Values     []float64
}

// Client is a minimal Prometheus HTTP API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New returns a Client targeting the given Prometheus base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryRangeParams holds the parameters for a range query.
type QueryRangeParams struct {
	Query string
	Start time.Time
	End   time.Time
	// Step is the resolution step, e.g. "60" (seconds as string).
	Step string
}

// QueryRange executes a Prometheus range query and returns matching series.
func (c *Client) QueryRange(ctx context.Context, params QueryRangeParams) ([]Series, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/query_range")
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	q := url.Values{
		"query": {params.Query},
		"start": {formatTimestamp(params.Start)},
		"end":   {formatTimestamp(params.End)},
		"step":  {params.Step},
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query Prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned HTTP %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s", apiResp.Error)
	}

	series := make([]Series, 0, len(apiResp.Data.Result))
	for _, r := range apiResp.Data.Result {
		s := Series{
			Label:  MetricLabel(r.Metric),
			Labels: r.Metric,
		}
		for _, v := range r.Values {
			if len(v) != 2 {
				continue
			}
			tsFloat, ok := v[0].(float64)
			if !ok {
				continue
			}
			valStr, ok := v[1].(string)
			if !ok {
				continue
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}
			s.Timestamps = append(s.Timestamps, time.Unix(int64(tsFloat), 0))
			s.Values = append(s.Values, val)
		}
		if len(s.Timestamps) > 0 {
			series = append(series, s)
		}
	}

	return series, nil
}

// MetricLabel builds a human-readable label from a Prometheus metric label set,
// excluding __name__, job, and instance.
func MetricLabel(labels map[string]string) string {
	exclude := map[string]bool{"__name__": true, "job": true, "instance": true}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		if !exclude[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}

	if len(parts) == 0 {
		if name, ok := labels["__name__"]; ok {
			return name
		}
		return "value"
	}
	return strings.Join(parts, ", ")
}

// apiResponse is the top-level Prometheus HTTP API response envelope.
type apiResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			// Each element is [unixTimestamp float64, value string].
			Values [][]any `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func formatTimestamp(t time.Time) string {
	return strconv.FormatFloat(float64(t.UnixNano())/1e9, 'f', 3, 64)
}
