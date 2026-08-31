package prozorro

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"skarbnyk/internal/parse"
)

const Base = "https://procedure.prozorro.sale"

type Client struct {
	HTTP *http.Client
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 45 * time.Second}}
}

type Loc map[string]string

func (l *Loc) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || len(b) == 0 {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = Loc{"uk_UA": s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*l = m
	return nil
}

func (l Loc) UK() string {
	if l == nil {
		return ""
	}
	if v := l["uk_UA"]; v != "" {
		return v
	}
	return l["uk"]
}

type Procedure struct {
	ID                string `json:"_id"`
	AuctionID         string `json:"auctionId"`
	SellingMethod     string `json:"sellingMethod"`
	Status            string `json:"status"`
	DateModified      string `json:"dateModified"`
	PreviousAuctionID string `json:"previousAuctionId"`
	TenderAttempts    int    `json:"tenderAttempts"`
	Title             Loc    `json:"title"`
	Description       Loc    `json:"description"`
	Value             struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"value"`
	SellingEntity struct {
		Name       Loc `json:"name"`
		Identifier struct {
			ID        string `json:"id"`
			LegalName Loc    `json:"legalName"`
		} `json:"identifier"`
	} `json:"sellingEntity"`
	Items []struct {
		Description    Loc `json:"description"`
		Classification struct {
			ID string `json:"id"`
		} `json:"classification"`
		Address struct {
			Locality  Loc `json:"locality"`
			AddressID struct {
				ID   string `json:"id"`
				Name Loc    `json:"name"`
			} `json:"addressID"`
		} `json:"address"`
	} `json:"items"`
	AuctionPeriod struct {
		StartDate string `json:"startDate"`
	} `json:"auctionPeriod"`
}

func (p Procedure) EntityName() string {
	if n := p.SellingEntity.Identifier.LegalName.UK(); n != "" {
		return n
	}
	return p.SellingEntity.Name.UK()
}

func (p Procedure) ClassID() string {
	if len(p.Items) == 0 {
		return ""
	}
	return p.Items[0].Classification.ID
}

func (p Procedure) AddressCity() string {
	for _, it := range p.Items {
		if s := parse.Settlement(it.Address.Locality.UK(), it.Address.AddressID.Name.UK()); s != "" {
			return s
		}
	}
	return ""
}

func (p Procedure) Text() (title, desc string) {
	title = p.Title.UK()
	desc = p.Description.UK()
	for _, it := range p.Items {
		if d := it.Description.UK(); d != "" {
			desc += "\n" + d
		}
	}
	return title, desc
}

func (c *Client) Page(ctx context.Context, from time.Time) ([]json.RawMessage, error) {
	url := fmt.Sprintf("%s/api/search/byDateModified/%s?limit=100", Base, from.UTC().Format("2006-01-02T15:04:05.000000Z"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("prozorro %d: %s", res.StatusCode, truncate(body, 200))
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func Decode(raw json.RawMessage) (Procedure, error) {
	var p Procedure
	err := json.Unmarshal(raw, &p)
	return p, err
}

func ParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000000Z", s)
	}
	if err != nil {
		return time.Time{}
	}
	return t
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
