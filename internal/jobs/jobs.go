package jobs

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"skarbnyk/internal/parse"
	"skarbnyk/internal/prozorro"
	"skarbnyk/internal/ria"
	"skarbnyk/internal/score"
	"skarbnyk/internal/store"
)

type Runner struct {
	Store  *store.Store
	PZ     *prozorro.Client
	RIA    *ria.Client
	USD    float64
	Since  time.Time
}

func (r *Runner) Start(ctx context.Context) {
	go loop(ctx, 3*time.Minute, r.Poll)
	go loop(ctx, 15*time.Minute, r.Match)
	go loop(ctx, 24*time.Hour, r.Refresh)
	go func() {
		_ = r.Poll(ctx)
		_ = r.RefreshDicts(ctx)
		_ = r.Match(ctx)
	}()
}

func loop(ctx context.Context, d time.Duration, fn func(context.Context) error) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := fn(ctx); err != nil {
				log.Printf("job: %v", err)
			}
		}
	}
}

func (r *Runner) Poll(ctx context.Context) error {
	from := r.Store.Cursor(ctx, r.Since)
	log.Printf("poll from %s", from.UTC().Format(time.RFC3339))
	pages := 0
	for {
		raws, err := r.PZ.Page(ctx, from)
		if err != nil {
			return err
		}
		if len(raws) == 0 {
			return r.Store.SetCursor(ctx, from)
		}
		var max time.Time
		for _, raw := range raws {
			p, err := prozorro.Decode(raw)
			if err != nil {
				continue
			}
			if err := r.Store.UpsertProcedure(ctx, p, raw); err != nil {
				log.Printf("upsert %s: %v", p.ID, err)
				continue
			}
			if t := prozorro.ParseTime(p.DateModified); t.After(max) {
				max = t
			}
		}
		pages++
		if max.IsZero() || !max.After(from) {
			return r.Store.SetCursor(ctx, from.Add(time.Millisecond))
		}
		from = max.Add(time.Microsecond)
		if err := r.Store.SetCursor(ctx, from); err != nil {
			return err
		}
		if pages >= 40 {
			log.Printf("poll paused after %d pages", pages)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (r *Runner) Match(ctx context.Context) error {
	rows, err := r.Store.Unmatched(ctx, 200)
	if err != nil {
		return err
	}
	marks, _ := r.Store.Marks(ctx)
	for _, row := range rows {
		classID := ""
		var raw json.RawMessage
		_ = r.Store.Pool.QueryRow(ctx, `SELECT raw FROM procedures WHERE id=$1`, row.ID).Scan(&raw)
		if len(raw) > 0 {
			if p, err := prozorro.Decode(raw); err == nil {
				classID = p.ClassID()
			}
		}
		a := parse.Classify(row.Title, row.Description, classID)
		markaID := ria.MatchMark(a.Brand, marks)
		if err := r.Store.SaveAttrs(ctx, row.ID, a, markaID, 0, 0); err != nil {
			log.Printf("attrs %s: %v", row.ID, err)
			continue
		}
		family := score.Family(row.SellingMethod)
		pass := score.PassN(row.TenderAttempts, row.PreviousID)
		if a.Kind == parse.KindSkip || a.Confidence < 0.6 || r.RIA == nil || r.RIA.Key == "" {
			_ = r.Store.SaveScore(ctx, row.ID, nil, pass, family, 0)
			continue
		}
		sn, err := r.price(ctx, a, markaID)
		if err != nil {
			log.Printf("ria %s: %v", row.ID, err)
			_ = r.Store.SaveScore(ctx, row.ID, nil, pass, family, 0)
			continue
		}
		market := sn.Median
		if sn.Currency == "USD" && r.USD > 0 {
			market *= r.USD
		}
		d := score.Discount(row.StartAmount, market)
		if err := r.Store.SaveScore(ctx, row.ID, &d, pass, family, market); err != nil {
			log.Printf("score %s: %v", row.ID, err)
			continue
		}
		alert(row, a, d, market)
	}
	return nil
}

func (r *Runner) Refresh(ctx context.Context) error {
	if err := r.RefreshDicts(ctx); err != nil {
		return err
	}
	if _, err := r.Store.Pool.Exec(ctx, `DELETE FROM price_snapshots WHERE fetched_at < now() - interval '24 hours'`); err != nil {
		return err
	}
	return r.rescore(ctx)
}

func (r *Runner) rescore(ctx context.Context) error {
	rows, err := r.Store.StaleScores(ctx, 200)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	// Drop attrs so Match re-parses and re-prices.
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if _, err := r.Store.Pool.Exec(ctx, `DELETE FROM lot_attrs WHERE procedure_id = ANY($1)`, ids); err != nil {
		return err
	}
	return r.Match(ctx)
}

func (r *Runner) price(ctx context.Context, a parse.Attrs, markaID int) (ria.Snapshot, error) {
	h := ria.SpecHash(string(a.Kind), markaID, 0, a.Year, 0)
	if sn, ok, err := r.Store.Snapshot(ctx, h); err != nil {
		return ria.Snapshot{}, err
	} else if ok {
		return sn, nil
	}
	var (
		sn  ria.Snapshot
		err error
	)
	if a.Kind == parse.KindCar {
		sn, err = r.RIA.AverageCar(ctx, markaID, 0, a.Year)
	} else {
		sn, err = r.RIA.AverageApt(ctx, 0, a.Rooms)
	}
	if err != nil {
		return sn, err
	}
	_ = r.Store.SaveSnapshot(ctx, h, sn)
	return sn, nil
}

func (r *Runner) RefreshDicts(ctx context.Context) error {
	if r.RIA == nil || r.RIA.Key == "" {
		return nil
	}
	marks, err := r.RIA.Marks(ctx)
	if err != nil {
		return err
	}
	return r.Store.ReplaceMarks(ctx, marks)
}

func EnvUSD() float64 {
	if v := os.Getenv("USD_UAH"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return 41.5
}

func DefaultSince() time.Time {
	if v := os.Getenv("POLL_SINCE"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			return t
		}
	}
	return time.Now().AddDate(0, 0, -14)
}
