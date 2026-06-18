package quotarules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client talks to one Coralogix region with one API key. It is a thin wrapper
// over net/http: build a request, add the auth header, check the status, decode
// the JSON.
type Client struct {
	host   string // e.g. api.eu2.coralogix.com
	apiKey string
	http   *http.Client
}

func NewClient(host, apiKey string) *Client {
	return &Client{
		host:   host,
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 30 * time.Second,
			// Don't follow redirects. A bad API key makes the metrics API
			// answer with a 302 to a login page; following it would land on
			// some HTML and produce a confusing "invalid character" JSON error.
			// Stopping here lets us report it as the auth failure it is.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// do sends a request and decodes the JSON response body into out. method is
// "GET" or "POST"; body is nil for GET, or any value to be JSON-encoded.
func (c *Client) do(method, urlStr string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, urlStr, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", urlStr, err)
	}
	if resp.StatusCode != http.StatusOK {
		body := strings.TrimSpace(string(data))
		// 401/403, and the 302-to-login the metrics API uses, all mean the key
		// was rejected — say so plainly rather than dumping a status code.
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden,
			http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
			msg := fmt.Sprintf("authentication failed (HTTP %d) — check the API key and its permissions", resp.StatusCode)
			if body != "" {
				msg += ": " + body
			}
			return fmt.Errorf("%s", msg)
		default:
			if body == "" {
				body = "(no response body)"
			}
			return fmt.Errorf("%s returned HTTP %d: %s", urlStr, resp.StatusCode, body)
		}
	}

	if err := json.Unmarshal(data, out); err != nil {
		snippet := strings.TrimSpace(string(data))
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return fmt.Errorf("unexpected non-JSON response from %s: %s", urlStr, snippet)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Metrics API — the PromQL instant-query endpoint.
//
// We use it for everything that lives in metrics: the total quota and today's
// usage. It speaks the standard Prometheus instant-query format.
// ---------------------------------------------------------------------------

type promResponse struct {
	Data struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			// Value is [ <unix-ts>, "<number-as-string>" ].
			Value []any `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// promSample is one returned series: its labels and its numeric value.
type promSample struct {
	Labels map[string]string
	Value  float64
}

// query runs an instant PromQL query and returns the resulting samples.
func (c *Client) query(promQL string) ([]promSample, error) {
	q := url.Values{}
	q.Set("query", promQL)
	urlStr := fmt.Sprintf("https://%s/metrics/api/v1/query?%s", c.host, q.Encode())

	var resp promResponse
	if err := c.do("GET", urlStr, nil, &resp); err != nil {
		return nil, err
	}

	samples := make([]promSample, 0, len(resp.Data.Result))
	for _, r := range resp.Data.Result {
		// A Prometheus value is [timestamp, "stringValue"]; we want index 1.
		if len(r.Value) != 2 {
			return nil, fmt.Errorf("unexpected metric value shape: %v", r.Value)
		}
		s, ok := r.Value[1].(string)
		if !ok {
			return nil, fmt.Errorf("metric value was not a string: %v", r.Value[1])
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing metric value %q: %w", s, err)
		}
		samples = append(samples, promSample{Labels: r.Metric, Value: f})
	}
	return samples, nil
}

// FetchTotalQuota returns the daily units allowance from cx_data_plan_units_per_day.
func (c *Client) FetchTotalQuota() (float64, error) {
	samples, err := c.query("cx_data_plan_units_per_day")
	if err != nil {
		return 0, err
	}
	if len(samples) == 0 {
		return 0, fmt.Errorf("metric cx_data_plan_units_per_day returned no data")
	}
	return samples[0].Value, nil
}

// FetchUsageByEntityType returns today's units used, keyed by entity type. The
// entity_type label values match the quota rules' entityType exactly (e.g.
// "logs", "metrics", "olly"), so no mapping is needed. cx_data_usage_units is a
// running daily total, so the instant value is "today so far".
func (c *Client) FetchUsageByEntityType() (map[string]float64, error) {
	samples, err := c.query("sum by (entity_type)(cx_data_usage_units)")
	if err != nil {
		return nil, err
	}
	usage := make(map[string]float64, len(samples))
	for _, s := range samples {
		usage[s.Labels["entity_type"]] = s.Value
	}
	return usage, nil
}

// FetchBlockedUnits returns today's units that were blocked.
func (c *Client) FetchBlockedUnits() (float64, error) {
	samples, err := c.query(`sum(cx_data_usage_units{priority="blocked"})`)
	if err != nil {
		return 0, err
	}
	if len(samples) == 0 {
		return 0, nil // no blocked data today
	}
	return samples[0].Value, nil
}

// ---------------------------------------------------------------------------
// Quota rules — GET /dataplan/quota-rules/v1.
// ---------------------------------------------------------------------------

type quotaRulesResponse struct {
	RuleSet struct {
		ID    string      `json:"id"`
		Rules []QuotaRule `json:"rules"`
	} `json:"ruleSet"`
}

// QuotaRule mirrors one rule from the API.
type QuotaRule struct {
	EntityType     string  `json:"entityType"`     // e.g. logs, metrics, olly
	Allocation     float64 `json:"allocation"`     // percent OR units, see AllocationType
	AllocationType string  `json:"allocationType"` // ...PERCENTAGE or ...LOCKED_UNITS
	Enabled        bool    `json:"enabled"`
	CanOverflow    bool    `json:"canOverflow"`
}

// FetchQuotaRules returns the current quota allocation rules.
func (c *Client) FetchQuotaRules() ([]QuotaRule, error) {
	urlStr := fmt.Sprintf("https://%s/mgmt/openapi/5/dataplan/quota-rules/v1", c.host)

	var resp quotaRulesResponse
	if err := c.do("GET", urlStr, nil, &resp); err != nil {
		return nil, err
	}
	return resp.RuleSet.Rules, nil
}
