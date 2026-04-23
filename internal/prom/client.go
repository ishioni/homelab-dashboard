package prom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Sample struct {
	Metric    map[string]string
	Timestamp time.Time
	Value     float64
}

type RangeSeries struct {
	Metric map[string]string
	Points []Point
}

type Point struct {
	Timestamp time.Time
	Value     float64
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Query(ctx context.Context, query string) ([]Sample, error) {
	response := promResponse{}
	if err := c.do(ctx, "/api/v1/query", url.Values{"query": []string{query}}, &response); err != nil {
		return nil, err
	}

	samples := make([]Sample, 0, len(response.Data.Result))
	for _, raw := range response.Data.Result {
		timestamp, value, err := parseValuePair(raw.Value)
		if err != nil {
			return nil, fmt.Errorf("parse sample %q: %w", query, err)
		}

		samples = append(samples, Sample{
			Metric:    raw.Metric,
			Timestamp: timestamp,
			Value:     value,
		})
	}

	return samples, nil
}

func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]RangeSeries, error) {
	response := promRangeResponse{}
	values := url.Values{
		"query": []string{query},
		"start": []string{strconv.FormatInt(start.Unix(), 10)},
		"end":   []string{strconv.FormatInt(end.Unix(), 10)},
		"step":  []string{strconv.FormatFloat(step.Seconds(), 'f', -1, 64)},
	}

	if err := c.do(ctx, "/api/v1/query_range", values, &response); err != nil {
		return nil, err
	}

	series := make([]RangeSeries, 0, len(response.Data.Result))
	for _, raw := range response.Data.Result {
		points := make([]Point, 0, len(raw.Values))
		for _, pair := range raw.Values {
			timestamp, value, err := parseValuePair(pair)
			if err != nil {
				return nil, fmt.Errorf("parse range sample %q: %w", query, err)
			}
			points = append(points, Point{Timestamp: timestamp, Value: value})
		}

		series = append(series, RangeSeries{
			Metric: raw.Metric,
			Points: points,
		})
	}

	return series, nil
}

func (c *Client) Scalar(ctx context.Context, query string) (float64, error) {
	samples, err := c.Query(ctx, query)
	if err != nil {
		return 0, err
	}

	if len(samples) == 0 {
		return 0, nil
	}

	return samples[0].Value, nil
}

func (c *Client) do(ctx context.Context, path string, values url.Values, target any) error {
	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("prometheus request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("prometheus returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func parseValuePair(raw []any) (time.Time, float64, error) {
	if len(raw) != 2 {
		return time.Time{}, 0, fmt.Errorf("unexpected pair length %d", len(raw))
	}

	tsFloat, ok := raw[0].(float64)
	if !ok {
		return time.Time{}, 0, fmt.Errorf("timestamp is %T", raw[0])
	}

	valueString, ok := raw[1].(string)
	if !ok {
		return time.Time{}, 0, fmt.Errorf("value is %T", raw[1])
	}

	value, err := strconv.ParseFloat(valueString, 64)
	if err != nil {
		if strings.EqualFold(valueString, "nan") {
			value = math.NaN()
		} else {
			return time.Time{}, 0, err
		}
	}

	seconds := int64(tsFloat)
	nanos := int64((tsFloat - float64(seconds)) * float64(time.Second))
	return time.Unix(seconds, nanos).UTC(), value, nil
}

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string           `json:"resultType"`
		Result     []instantResult  `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string          `json:"resultType"`
		Result     []rangeResult   `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

type instantResult struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
}

type rangeResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
}
