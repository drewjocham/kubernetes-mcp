package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"kube-watcher-app/internal/data"
)

// Client queries the Prometheus HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client using env KW_PROMETHEUS_URL or default localhost:9090.
func New() *Client {
	u := strings.TrimSpace(os.Getenv("KW_PROMETHEUS_URL"))
	if u == "" {
		u = "http://localhost:9090"
	}
	return &Client{
		baseURL: strings.TrimRight(u, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// NewWithURL returns a Client for a specific URL.
func NewWithURL(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

type queryRangeResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]any           `json:"values"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

// QueryRange executes a PromQL range query and returns metric series.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]data.MetricSeries, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(start.Unix(), 10))
	params.Set("end", strconv.FormatInt(end.Unix(), 10))
	params.Set("step", strconv.FormatInt(int64(step.Seconds()), 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/v1/query_range?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result queryRangeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus: %s", result.Error)
	}

	series := make([]data.MetricSeries, 0, len(result.Data.Result))
	for _, r := range result.Data.Result {
		pts := make([]data.DataPoint, 0, len(r.Values))
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
			pts = append(pts, data.DataPoint{
				Timestamp: time.Unix(int64(tsFloat), 0),
				Value:     val,
			})
		}
		series = append(series, data.MetricSeries{Labels: r.Metric, Points: pts})
	}
	return series, nil
}

type instantResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

// Query executes an instant PromQL query.
func (c *Client) Query(ctx context.Context, query string) ([]data.MetricSeries, error) {
	params := url.Values{}
	params.Set("query", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/v1/query?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result instantResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus: %s", result.Error)
	}

	series := make([]data.MetricSeries, 0, len(result.Data.Result))
	for _, r := range result.Data.Result {
		if len(r.Value) != 2 {
			continue
		}
		tsFloat, ok := r.Value[0].(float64)
		if !ok {
			continue
		}
		valStr, ok := r.Value[1].(string)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		series = append(series, data.MetricSeries{
			Labels: r.Metric,
			Points: []data.DataPoint{{Timestamp: time.Unix(int64(tsFloat), 0), Value: val}},
		})
	}
	return series, nil
}

// Ping checks if Prometheus is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/-/healthy", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus unhealthy: %s", resp.Status)
	}
	return nil
}
