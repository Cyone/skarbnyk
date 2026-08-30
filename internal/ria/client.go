package ria

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	Key  string
	HTTP *http.Client
}

func New(key string) *Client {
	return &Client{Key: key, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type Snapshot struct {
	Median     float64
	Arithmetic float64
	Currency   string
}

type Mark struct {
	ID   int
	Name string
}

func SpecHash(kind string, markaID, modelID, year, cityID int) string {
	s := fmt.Sprintf("%s/%d/%d/%d/%d", kind, markaID, modelID, year, cityID)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (c *Client) Marks(ctx context.Context) ([]Mark, error) {
	q := url.Values{"api_key": {c.Key}}
	raw, err := c.get(ctx, "https://developers.ria.com/auto/categories/1/marks?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Value int    `json:"value"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]Mark, 0, len(rows))
	for _, r := range rows {
		out = append(out, Mark{ID: r.Value, Name: r.Name})
	}
	return out, nil
}

func (c *Client) AverageCar(ctx context.Context, markaID, modelID, year int) (Snapshot, error) {
	q := url.Values{"api_key": {c.Key}}
	if markaID > 0 {
		q.Set("marka_id", strconv.Itoa(markaID))
	}
	if modelID > 0 {
		q.Set("model_id", strconv.Itoa(modelID))
	}
	if year > 0 {
		q.Add("yers", strconv.Itoa(year))
	}
	return c.average(ctx, "https://developers.ria.com/auto/average_price?"+q.Encode(), "USD")
}

func (c *Client) AverageApt(ctx context.Context, cityID, rooms int) (Snapshot, error) {
	q := url.Values{
		"api_key":      {c.Key},
		"category":     {"1"},
		"sub_category": {"2"},
		"operation":    {"1"},
		"date_from":    {time.Now().AddDate(0, -2, 0).Format("2006-01")},
		"date_to":      {time.Now().Format("2006-01")},
	}
	if cityID > 0 {
		q.Set("city_id", strconv.Itoa(cityID))
	}
	if rooms > 0 {
		q.Set("rooms_count", strconv.Itoa(rooms))
	}
	return c.average(ctx, "https://developers.ria.com/dom/average_price?"+q.Encode(), "UAH")
}

func (c *Client) average(ctx context.Context, u, currency string) (Snapshot, error) {
	raw, err := c.get(ctx, u)
	if err != nil {
		return Snapshot{}, err
	}
	var body struct {
		Total           int     `json:"total"`
		ArithmeticMean  float64 `json:"arithmeticMean"`
		InterQuartile   float64 `json:"interQuartileMean"`
		Percentiles     map[string]float64 `json:"percentiles"`
		Message         string  `json:"message"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return Snapshot{}, err
	}
	if body.Message != "" {
		return Snapshot{}, fmt.Errorf("ria: %s", body.Message)
	}
	med := body.InterQuartile
	if p, ok := body.Percentiles["50.0"]; ok {
		med = p
	}
	if med == 0 {
		med = body.ArithmeticMean
	}
	return Snapshot{Median: med, Arithmetic: body.ArithmeticMean, Currency: currency}, nil
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("ria %d: %s", res.StatusCode, string(b))
	}
	return b, nil
}

func MatchMark(name string, marks []Mark) int {
	if name == "" {
		return 0
	}
	want := strings.ToLower(name)
	for _, m := range marks {
		if strings.EqualFold(m.Name, name) {
			return m.ID
		}
	}
	for _, m := range marks {
		if strings.Contains(strings.ToLower(m.Name), want) {
			return m.ID
		}
	}
	return 0
}
