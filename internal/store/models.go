package store

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

var (
	finished  = []string{"complete", "cancelled", "unsuccessful", "deleted"}
	liveKinds = []string{"car", "apt"}
)

type meta struct {
	bun.BaseModel `bun:"table:meta"`

	Key   string `bun:"key,pk"`
	Value string `bun:"value"`
}

type procedure struct {
	bun.BaseModel `bun:"table:procedures,alias:p"`

	ID                string          `bun:"id,pk"`
	AuctionID         string          `bun:"auction_id"`
	SellingMethod     string          `bun:"selling_method"`
	SellingEntity     string          `bun:"selling_entity"`
	Status            string          `bun:"status"`
	StartAmount       float64         `bun:"start_amount"`
	Currency          string          `bun:"currency"`
	Title             string          `bun:"title"`
	Description       string          `bun:"description"`
	AuctionStart      *time.Time      `bun:"auction_start"`
	DateModified      *time.Time      `bun:"date_modified"`
	PreviousAuctionID string          `bun:"previous_auction_id,nullzero"`
	TenderAttempts    int             `bun:"tender_attempts"`
	Raw               json.RawMessage `bun:"raw,type:jsonb"`

	Attrs *lotAttr  `bun:"rel:has-one,join:id=procedure_id"`
	Score *lotScore `bun:"rel:has-one,join:id=procedure_id"`
}

type lotAttr struct {
	bun.BaseModel `bun:"table:lot_attrs,alias:a"`

	ProcedureID string    `bun:"procedure_id,pk"`
	Kind        string    `bun:"kind"`
	Rooms       int       `bun:"rooms,nullzero"`
	Area        float64   `bun:"area,nullzero"`
	Year        int       `bun:"year,nullzero"`
	City        string    `bun:"city,nullzero"`
	Brand       string    `bun:"brand,nullzero"`
	Model       string    `bun:"model,nullzero"`
	MarkaID     int       `bun:"marka_id,nullzero"`
	ModelID     int       `bun:"model_id,nullzero"`
	CityID      int       `bun:"city_id,nullzero"`
	Confidence  float64   `bun:"parse_confidence"`
	UpdatedAt   time.Time `bun:"updated_at,nullzero"`
}

type lotScore struct {
	bun.BaseModel `bun:"table:scores,alias:s"`

	ProcedureID string    `bun:"procedure_id,pk"`
	DiscountPct *float64  `bun:"discount_pct"`
	PassN       int       `bun:"pass_n"`
	Family      string    `bun:"auction_family"`
	RIAMedian   float64   `bun:"ria_median,nullzero"`
	ScoredAt    time.Time `bun:"scored_at,nullzero"`
	AlertedAt   time.Time `bun:"alerted_at,nullzero"`
}

type riaDict struct {
	bun.BaseModel `bun:"table:ria_dicts"`

	Kind  string `bun:"kind,pk"`
	RiaID int    `bun:"ria_id,pk"`
	Name  string `bun:"name"`
}

type priceSnapshot struct {
	bun.BaseModel `bun:"table:price_snapshots"`

	SpecHash   string    `bun:"spec_hash,pk"`
	Median     float64   `bun:"median"`
	Arithmetic float64   `bun:"arithmetic"`
	Currency   string    `bun:"currency"`
	FetchedAt  time.Time `bun:"fetched_at,nullzero"`
}

func rowFrom(p procedure) Row {
	r := Row{
		ID:             p.ID,
		AuctionID:      p.AuctionID,
		SellingMethod:  p.SellingMethod,
		SellingEntity:  p.SellingEntity,
		Status:         p.Status,
		StartAmount:    p.StartAmount,
		Currency:       p.Currency,
		Title:          p.Title,
		Description:    p.Description,
		AuctionStart:   p.AuctionStart,
		DateModified:   p.DateModified,
		PreviousID:     p.PreviousAuctionID,
		TenderAttempts: p.TenderAttempts,
	}
	if p.Attrs != nil {
		r.Kind = p.Attrs.Kind
		r.Rooms = p.Attrs.Rooms
		r.Area = p.Attrs.Area
		r.Year = p.Attrs.Year
		r.City = p.Attrs.City
		r.Brand = p.Attrs.Brand
		r.Confidence = p.Attrs.Confidence
	}
	if p.Score != nil {
		r.Discount = p.Score.DiscountPct
		r.PassN = p.Score.PassN
		r.Family = p.Score.Family
		if p.Score.RIAMedian != 0 {
			m := p.Score.RIAMedian
			r.RIAMedian = &m
		}
	}
	return r
}

func rowsFrom(ps []procedure) []Row {
	if len(ps) == 0 {
		return nil
	}
	out := make([]Row, len(ps))
	for i, p := range ps {
		out[i] = rowFrom(p)
	}
	return out
}
