package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"skarbnyk/internal/parse"
	"skarbnyk/internal/prozorro"
	"skarbnyk/internal/ria"
	"skarbnyk/internal/score"
	"skarbnyk/internal/store"
)

type Runner struct {
	Store *store.Store
	PZ    *prozorro.Client
	RIA   *ria.Client
	USD   float64
	Since time.Time

	matching sync.Mutex
}

func (r *Runner) Start(ctx context.Context) {
	go loop(ctx, 3*time.Minute, r.Poll)
	go loop(ctx, 15*time.Minute, r.Match)
	go loop(ctx, 24*time.Hour, r.Refresh)
	go func() {
		_ = r.Poll(ctx)
		_ = r.RefreshDicts(ctx)
		_ = r.Match(ctx)
		_ = r.BackfillCities(ctx)
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
			if len(raws) >= 100 {
				// ponytail: a full page that does not advance would loop forever; step past the
				// boundary and accept losing records sharing that millisecond.
				return r.Store.SetCursor(ctx, from.Add(time.Millisecond))
			}
			return r.Store.SetCursor(ctx, from)
		}
		from = max
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
	if !r.matching.TryLock() {
		log.Print("match: previous run still in flight, skipping")
		return nil
	}
	defer r.matching.Unlock()
	rows, err := r.Store.Unmatched(ctx, 200)
	if err != nil {
		return err
	}
	marks, _ := r.Store.Marks(ctx)
	for _, row := range rows {
		classID := ""
		var p prozorro.Procedure
		raw, _ := r.Store.ProcedureRaw(ctx, row.ID)
		if len(raw) > 0 {
			if decoded, err := prozorro.Decode(raw); err == nil {
				p = decoded
				classID = p.ClassID()
			}
		}
		a := parse.Classify(row.Title, row.Description, classID)
		if c := p.AddressCity(); c != "" {
			a.City = c
		}
		markaID := ria.MatchMark(a.Brand, marks)
		if err := r.Store.SaveAttrs(ctx, row.ID, a, markaID, 0, 0); err != nil {
			log.Printf("attrs %s: %v", row.ID, err)
			continue
		}
		family := score.Family(row.SellingMethod)
		pass := score.PassN(row.TenderAttempts, row.PreviousID)
		if a.Kind == parse.KindSkip || a.Confidence < 0.6 || r.RIA == nil || r.RIA.Key == "" ||
			!priceable(a, markaID) || (row.Currency != "" && row.Currency != "UAH") {
			_ = r.Store.SaveScore(ctx, row.ID, nil, pass, family, 0)
			continue
		}
		sn, err := r.price(ctx, a, markaID)
		if err != nil {
			if errors.Is(err, ria.ErrBudget) {
				log.Printf("ria: %v", err)
				// Leave the lot unmatched so the next hour can retry.
				_ = r.Store.DeleteAttrs(ctx, row.ID)
				return nil
			}
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
		if !alertWorthy(a, d) {
			continue
		}
		claimed, err := r.Store.MarkAlerted(ctx, row.ID)
		if err != nil {
			log.Printf("alert claim %s: %v", row.ID, err)
			continue
		}
		if !claimed || alert(ctx, row, d, market) {
			continue
		}
		// A cancelled ctx killed the send too, so release on a fresh one or the lot never alerts.
		if err := r.Store.ReleaseAlert(context.WithoutCancel(ctx), row.ID); err != nil {
			log.Printf("alert release %s: %v", row.ID, err)
		}
	}
	return nil
}

func (r *Runner) Refresh(ctx context.Context) error {
	if err := r.RefreshDicts(ctx); err != nil {
		return err
	}
	if err := r.BackfillCities(ctx); err != nil {
		return err
	}
	if err := r.Store.DeleteOldSnapshots(ctx); err != nil {
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
	if err := r.Store.DeleteAttrs(ctx, ids...); err != nil {
		return err
	}
	return r.Match(ctx)
}

func specHash(a parse.Attrs, markaID int) string {
	if a.Kind == parse.KindCar {
		// ponytail: model is never resolved, so car medians stay marka-wide until a model dict exists.
		return ria.SpecHash("car", markaID, a.Year)
	}
	// ponytail: no city dimension until a DIM.RIA city dict exists; medians stay national.
	return ria.SpecHash("apt", a.Rooms)
}

func priceable(a parse.Attrs, markaID int) bool {
	if a.Kind == parse.KindCar {
		return markaID > 0 && a.Year > 0
	}
	return a.Rooms > 0
}

func (r *Runner) price(ctx context.Context, a parse.Attrs, markaID int) (ria.Snapshot, error) {
	h := specHash(a, markaID)
	if sn, ok, err := r.Store.Snapshot(ctx, h); err != nil {
		return ria.Snapshot{}, err
	} else if ok {
		return sn, nil
	}
	if err := r.Store.ChargeRIA(ctx, time.Now()); err != nil {
		return ria.Snapshot{}, err
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
	if err := r.Store.SaveSnapshot(ctx, h, sn); err != nil {
		log.Printf("snapshot %s: %v", h, err)
	}
	return sn, nil
}

func (r *Runner) BackfillCities(ctx context.Context) error {
	done, err := r.Store.CitiesFromAddressDone(ctx)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	n := 0
	err = r.Store.EachAddressed(ctx, func(id string, raw json.RawMessage) error {
		p, err := prozorro.Decode(raw)
		if err != nil {
			log.Printf("city decode %s: %v", id, err)
			return nil
		}
		c := p.AddressCity()
		if c == "" {
			return nil
		}
		changed, err := r.Store.SetCity(ctx, id, c)
		if err != nil {
			return err
		}
		n += int(changed)
		return nil
	})
	if err != nil {
		return err
	}
	if err := r.Store.MarkCitiesFromAddress(ctx); err != nil {
		return err
	}
	log.Printf("cities: %d from address", n)
	return nil
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
