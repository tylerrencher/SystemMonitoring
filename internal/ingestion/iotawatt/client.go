package iotawatt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Series struct {
	Name string
	Unit string // "Watts" or "Volts"
}

type Reading struct {
	Time   time.Time
	Series string
	Watts  *float32
	Volts  *float32
}

type Client struct {
	host       string
	httpClient *http.Client
}

func NewClient(host string) *Client {
	return &Client{
		host: host,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) GetSeries() ([]Series, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/query?show=series", c.host))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Series []struct {
			Name string `json:"name"`
			Unit string `json:"unit"`
		} `json:"series"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	series := make([]Series, len(result.Series))
	for i, s := range result.Series {
		series[i] = Series{Name: s.Name, Unit: s.Unit}
	}
	return series, nil
}

// Query fetches readings for all series between begin and end grouped by groupSecs seconds.
// IoTawatt requires begin, end, AND group — without group it returns an error.
func (c *Client) Query(series []Series, begin, end time.Time, groupSecs int) ([]Reading, error) {
	names := make([]string, len(series))
	for i, s := range series {
		names[i] = s.Name
	}
	// time must be last in select so positional parsing works
	selectParam := "[" + strings.Join(names, ",") + ",time]"

	u := fmt.Sprintf("http://%s/query?select=%s&begin=%d&end=%d&group=%ds",
		c.host,
		url.QueryEscape(selectParam),
		begin.Unix(),
		end.Unix(),
		groupSecs,
	)

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Response: [[val0, val1, ..., "2026-05-15T12:00:00"], ...]
	var rows [][]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	unitMap := make(map[string]string, len(series))
	for _, s := range series {
		unitMap[s.Name] = s.Unit
	}

	var readings []Reading
	for _, row := range rows {
		if len(row) != len(series)+1 {
			continue
		}

		// Last element is the timestamp string
		var timeStr string
		if err := json.Unmarshal(row[len(row)-1], &timeStr); err != nil {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02T15:04:05", timeStr, time.Local)
		if err != nil {
			continue
		}

		for i, s := range series {
			var val float64
			if err := json.Unmarshal(row[i], &val); err != nil {
				continue
			}
			f := float32(val)
			r := Reading{Time: t, Series: s.Name}
			switch unitMap[s.Name] {
			case "Volts":
				r.Volts = &f
			default:
				r.Watts = &f
			}
			readings = append(readings, r)
		}
	}

	return readings, nil
}
