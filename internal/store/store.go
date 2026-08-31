package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"skarbnyk/internal/parse"
	"skarbnyk/internal/prozorro"
	"skarbnyk/internal/ria"

	_ "embed"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db *bun.DB
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	if err := sqldb.PingContext(ctx); err != nil {
		sqldb.Close()
		return nil, err
	}
	s := &Store{db: bun.NewDB(sqldb, pgdialect.New())}
	if err := s.migrate(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	for _, stmt := range strings.Split(schemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() { _ = s.db.Close() }

func (s *Store) Cursor(ctx context.Context, fallback time.Time) time.Time {
	v, err := s.metaValue(ctx, "date_modified")
	if err != nil || v == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return fallback
	}
	return t
}

func (s *Store) SetCursor(ctx context.Context, t time.Time) error {
	return s.setMeta(ctx, "date_modified", t.UTC().Format(time.RFC3339Nano))
}

func (s *Store) setMeta(ctx context.Context, key, value string) error {
	_, err := s.db.NewInsert().
		Model(&meta{Key: key, Value: value}).
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Exec(ctx)
	return err
}

func (s *Store) metaValue(ctx context.Context, key string) (string, error) {
	var m meta
	err := s.db.NewSelect().Model(&m).Where("key = ?", key).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return m.Value, err
}

// ChargeRIA spends one freemium average_price slot. Cache hits must not call this.
func (s *Store) ChargeRIA(ctx context.Context, now time.Time) error {
	hourB, monthB := ria.HourBucket(now), ria.MonthBucket(now)
	hourStored, err := s.metaValue(ctx, "ria_hour")
	if err != nil {
		return err
	}
	monthStored, err := s.metaValue(ctx, "ria_month")
	if err != nil {
		return err
	}
	hourN := ria.ParseCount(hourStored, hourB)
	monthN := ria.ParseCount(monthStored, monthB)
	if !ria.Allow(hourN, monthN) {
		return ria.ErrBudget
	}
	if err := s.setMeta(ctx, "ria_hour", ria.FormatCount(hourB, hourN+1)); err != nil {
		return err
	}
	return s.setMeta(ctx, "ria_month", ria.FormatCount(monthB, monthN+1))
}

func (s *Store) UpsertProcedure(ctx context.Context, p prozorro.Procedure, raw json.RawMessage) error {
	title, desc := p.Text()
	row := procedure{
		ID:                p.ID,
		AuctionID:         p.AuctionID,
		SellingMethod:     p.SellingMethod,
		SellingEntity:     p.EntityName(),
		Status:            p.Status,
		StartAmount:       p.Value.Amount,
		Currency:          p.Value.Currency,
		Title:             title,
		Description:       desc,
		PreviousAuctionID: p.PreviousAuctionID,
		TenderAttempts:    p.TenderAttempts,
		Raw:               raw,
	}
	if t := prozorro.ParseTime(p.AuctionPeriod.StartDate); !t.IsZero() {
		row.AuctionStart = &t
	}
	if t := prozorro.ParseTime(p.DateModified); !t.IsZero() {
		row.DateModified = &t
	}
	_, err := upsertProcedureQ(s.db, &row).Exec(ctx)
	return err
}

func upsertProcedureQ(db *bun.DB, p *procedure) *bun.InsertQuery {
	return db.NewInsert().
		Model(p).
		On("CONFLICT (id) DO UPDATE").
		Set("auction_id = EXCLUDED.auction_id").
		Set("selling_method = EXCLUDED.selling_method").
		Set("selling_entity = EXCLUDED.selling_entity").
		Set("status = EXCLUDED.status").
		Set("start_amount = COALESCE(NULLIF(p.start_amount, 0), EXCLUDED.start_amount)").
		Set("currency = EXCLUDED.currency").
		Set("title = EXCLUDED.title").
		Set("description = EXCLUDED.description").
		Set("auction_start = EXCLUDED.auction_start").
		Set("date_modified = EXCLUDED.date_modified").
		Set("previous_auction_id = EXCLUDED.previous_auction_id").
		Set("tender_attempts = EXCLUDED.tender_attempts").
		Set("raw = EXCLUDED.raw")
}

func (s *Store) Unmatched(ctx context.Context, limit int) ([]Row, error) {
	var ps []procedure
	err := s.db.NewSelect().
		Model(&ps).
		ExcludeColumn("raw").
		Where("NOT EXISTS (SELECT 1 FROM lot_attrs AS a WHERE a.procedure_id = p.id)").
		Apply(notFinished).
		OrderExpr("p.date_modified DESC NULLS LAST").
		Limit(limit).
		Scan(ctx)
	return rowsFrom(ps), err
}

func (s *Store) SaveAttrs(ctx context.Context, id string, a parse.Attrs, markaID, modelID, cityID int) error {
	row := lotAttr{
		ProcedureID: id,
		Kind:        string(a.Kind),
		Rooms:       a.Rooms,
		Area:        a.Area,
		Year:        a.Year,
		City:        a.City,
		Brand:       a.Brand,
		Model:       a.Model,
		MarkaID:     markaID,
		ModelID:     modelID,
		CityID:      cityID,
		Confidence:  a.Confidence,
		UpdatedAt:   time.Now(),
	}
	_, err := s.db.NewInsert().
		Model(&row).
		On("CONFLICT (procedure_id) DO UPDATE").
		Exec(ctx)
	return err
}

func (s *Store) SaveScore(ctx context.Context, id string, discount *float64, pass int, family string, median float64) error {
	row := lotScore{
		ProcedureID: id,
		DiscountPct: discount,
		PassN:       pass,
		Family:      family,
		RIAMedian:   median,
		ScoredAt:    time.Now(),
	}
	_, err := saveScoreQ(s.db, &row).Exec(ctx)
	return err
}

func saveScoreQ(db *bun.DB, row *lotScore) *bun.InsertQuery {
	return db.NewInsert().
		Model(row).
		On("CONFLICT (procedure_id) DO UPDATE").
		Set("discount_pct = EXCLUDED.discount_pct").
		Set("pass_n = EXCLUDED.pass_n").
		Set("auction_family = EXCLUDED.auction_family").
		Set("ria_median = EXCLUDED.ria_median").
		Set("scored_at = EXCLUDED.scored_at")
}

// MarkAlerted claims the lot for a single notification; false means someone already sent it.
func (s *Store) MarkAlerted(ctx context.Context, id string) (bool, error) {
	res, err := s.db.NewUpdate().
		Model((*lotScore)(nil)).
		Set("alerted_at = now()").
		Where("procedure_id = ?", id).
		Where("alerted_at IS NULL").
		Exec(ctx)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ReleaseAlert hands a claim back after a failed send; Match is single-flight, so nothing can race it.
func (s *Store) ReleaseAlert(ctx context.Context, id string) error {
	_, err := s.db.NewUpdate().
		Model((*lotScore)(nil)).
		Set("alerted_at = NULL").
		Where("procedure_id = ?", id).
		Exec(ctx)
	return err
}

func (s *Store) StaleScores(ctx context.Context, limit int) ([]Row, error) {
	var ps []procedure
	err := s.db.NewSelect().
		Model(&ps).
		ExcludeColumn("raw").
		Relation("Attrs").
		Relation("Score").
		Where("attrs.kind IN (?)", bun.In(liveKinds)).
		Apply(notFinished).
		Where("score.scored_at IS NULL OR score.scored_at < now() - interval '24 hours'").
		OrderExpr("p.date_modified DESC NULLS LAST").
		Limit(limit).
		Scan(ctx)
	return rowsFrom(ps), err
}

func (s *Store) Snapshot(ctx context.Context, hash string) (ria.Snapshot, bool, error) {
	var row priceSnapshot
	err := s.db.NewSelect().Model(&row).Where("spec_hash = ?", hash).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return ria.Snapshot{}, false, nil
	}
	if err != nil {
		return ria.Snapshot{}, false, err
	}
	if time.Since(row.FetchedAt) > 24*time.Hour {
		return ria.Snapshot{}, false, nil
	}
	return ria.Snapshot{Median: row.Median, Arithmetic: row.Arithmetic, Currency: row.Currency}, true, nil
}

func (s *Store) SaveSnapshot(ctx context.Context, hash string, sn ria.Snapshot) error {
	row := priceSnapshot{
		SpecHash:   hash,
		Median:     sn.Median,
		Arithmetic: sn.Arithmetic,
		Currency:   sn.Currency,
		FetchedAt:  time.Now(),
	}
	_, err := s.db.NewInsert().
		Model(&row).
		On("CONFLICT (spec_hash) DO UPDATE").
		Exec(ctx)
	return err
}

func (s *Store) DeleteOldSnapshots(ctx context.Context) error {
	_, err := s.db.NewDelete().
		Model((*priceSnapshot)(nil)).
		Where("fetched_at < now() - interval '24 hours'").
		Exec(ctx)
	return err
}

func (s *Store) ReplaceMarks(ctx context.Context, marks []ria.Mark) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*riaDict)(nil)).Where("kind = ?", "mark").Exec(ctx); err != nil {
			return err
		}
		if len(marks) == 0 {
			return nil
		}
		rows := make([]riaDict, len(marks))
		for i, m := range marks {
			rows[i] = riaDict{Kind: "mark", RiaID: m.ID, Name: m.Name}
		}
		_, err := tx.NewInsert().Model(&rows).Exec(ctx)
		return err
	})
}

func (s *Store) Marks(ctx context.Context) ([]ria.Mark, error) {
	var rows []riaDict
	if err := s.db.NewSelect().Model(&rows).Where("kind = ?", "mark").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]ria.Mark, len(rows))
	for i, r := range rows {
		out[i] = ria.Mark{ID: r.RiaID, Name: r.Name}
	}
	return out, nil
}

func citiesSelect(db *bun.DB) *bun.SelectQuery {
	return db.NewSelect().
		Model((*lotAttr)(nil)).
		ColumnExpr("DISTINCT a.city").
		Join("JOIN procedures AS p ON p.id = a.procedure_id").
		Where("a.city IS NOT NULL AND a.city <> ''").
		Where("a.kind IN (?)", bun.In(liveKinds)).
		Apply(notFinished).
		OrderExpr("a.city")
}

func (s *Store) Cities(ctx context.Context) ([]string, error) {
	var out []string
	err := citiesSelect(s.db).Scan(ctx, &out)
	return out, err
}

func entitiesSelect(db *bun.DB) *bun.SelectQuery {
	return db.NewSelect().
		Model((*procedure)(nil)).
		ColumnExpr("DISTINCT p.selling_entity").
		Join("JOIN lot_attrs AS a ON a.procedure_id = p.id").
		Where("p.selling_entity IS NOT NULL AND p.selling_entity <> ''").
		Where("a.kind IN (?)", bun.In(liveKinds)).
		Apply(notFinished).
		OrderExpr("p.selling_entity")
}

func (s *Store) Entities(ctx context.Context) ([]string, error) {
	var out []string
	err := entitiesSelect(s.db).Scan(ctx, &out)
	return out, err
}

func passesSelect(db *bun.DB) *bun.SelectQuery {
	return db.NewSelect().
		Model((*lotScore)(nil)).
		ColumnExpr("DISTINCT s.pass_n").
		Join("JOIN procedures AS p ON p.id = s.procedure_id").
		Join("JOIN lot_attrs AS a ON a.procedure_id = p.id").
		Where("s.pass_n IS NOT NULL AND s.pass_n > 0").
		Where("a.kind IN (?)", bun.In(liveKinds)).
		Apply(notFinished).
		OrderExpr("s.pass_n")
}

func (s *Store) Passes(ctx context.Context) ([]int, error) {
	var out []int
	err := passesSelect(s.db).Scan(ctx, &out)
	return out, err
}

func (s *Store) SetCity(ctx context.Context, id, city string) (int64, error) {
	res, err := s.db.NewUpdate().
		Model((*lotAttr)(nil)).
		Set("city = ?", city).
		Set("updated_at = now()").
		Where("procedure_id = ?", id).
		Where("city IS DISTINCT FROM ?", city).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) EachAddressed(ctx context.Context, fn func(id string, raw json.RawMessage) error) error {
	rows, err := s.db.NewSelect().
		Model((*procedure)(nil)).
		Column("id", "raw").
		Join("JOIN lot_attrs AS a ON a.procedure_id = p.id").
		Apply(notFinished).
		Rows(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var raw json.RawMessage
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		if err := fn(id, raw); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) ProcedureRaw(ctx context.Context, id string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := s.db.NewSelect().Model((*procedure)(nil)).Column("raw").Where("id = ?", id).Scan(ctx, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return raw, err
}

func (s *Store) DeleteAttrs(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.NewDelete().
		Model((*lotAttr)(nil)).
		Where("procedure_id IN (?)", bun.In(ids)).
		Exec(ctx)
	return err
}

func (s *Store) CitiesFromAddressDone(ctx context.Context) (bool, error) {
	v, err := s.metaValue(ctx, "cities_from_address")
	return v == "1", err
}

func (s *Store) MarkCitiesFromAddress(ctx context.Context) error {
	return s.setMeta(ctx, "cities_from_address", "1")
}

type Filter struct {
	Kind        string
	City        string
	Entity      string
	MinDiscount float64
	Pass        int
	Family      string
}

type Row struct {
	ID             string
	AuctionID      string
	SellingMethod  string
	SellingEntity  string
	Status         string
	StartAmount    float64
	Currency       string
	Title          string
	Description    string
	AuctionStart   *time.Time
	DateModified   *time.Time
	PreviousID     string
	TenderAttempts int
	Kind           string
	Rooms          int
	Area           float64
	Year           int
	City           string
	Brand          string
	Confidence     float64
	Discount       *float64
	PassN          int
	Family         string
	RIAMedian      *float64
}

func notFinished(q *bun.SelectQuery) *bun.SelectQuery {
	return q.Where("COALESCE(p.status, '') NOT IN (?)", bun.In(finished))
}

func applyFilter(q *bun.SelectQuery, f Filter) *bun.SelectQuery {
	if f.Kind != "" {
		q = q.Where("attrs.kind = ?", f.Kind)
	}
	if f.City != "" {
		q = q.Where("attrs.city ILIKE ?", "%"+f.City+"%")
	}
	if f.Entity != "" {
		q = q.Where("p.selling_entity ILIKE ?", "%"+f.Entity+"%")
	}
	if f.MinDiscount > 0 {
		q = q.Where("score.discount_pct >= ?", f.MinDiscount)
	}
	if f.Pass > 0 {
		q = q.Where("score.pass_n = ?", f.Pass)
	}
	if f.Family != "" {
		q = q.Where("score.auction_family = ?", f.Family)
	}
	return q
}

func listSelect(db *bun.DB, f Filter) *bun.SelectQuery {
	return applyFilter(db.NewSelect().
		Model((*procedure)(nil)).
		ExcludeColumn("raw").
		Relation("Attrs").
		Relation("Score").
		Where("attrs.kind IN (?)", bun.In(liveKinds)).
		Apply(notFinished), f)
}

func (s *Store) Count(ctx context.Context, f Filter) (int, error) {
	return listSelect(s.db, f).Count(ctx)
}

func (s *Store) List(ctx context.Context, f Filter, limit, offset int) ([]Row, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var ps []procedure
	err := listSelect(s.db, f).
		OrderExpr("score.discount_pct DESC NULLS LAST").
		OrderExpr("p.auction_start ASC NULLS LAST").
		Limit(limit).
		Offset(offset).
		Scan(ctx, &ps)
	return rowsFrom(ps), err
}

func (s *Store) Get(ctx context.Context, id string) (Row, error) {
	var p procedure
	err := s.db.NewSelect().
		Model(&p).
		ExcludeColumn("raw").
		Relation("Attrs").
		Relation("Score").
		Where("p.id = ?", id).
		Scan(ctx)
	if err != nil {
		return Row{}, err
	}
	return rowFrom(p), nil
}
