package store

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"skarbnyk/internal/parse"
	"skarbnyk/internal/prozorro"
	"skarbnyk/internal/ria"

	_ "embed"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Store, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	if _, err := p.Exec(ctx, schemaSQL); err != nil {
		p.Close()
		return nil, err
	}
	return &Store{Pool: p}, nil
}

func (s *Store) Close() { s.Pool.Close() }

func (s *Store) Cursor(ctx context.Context, fallback time.Time) time.Time {
	var v string
	err := s.Pool.QueryRow(ctx, `SELECT value FROM meta WHERE key='date_modified'`).Scan(&v)
	if err != nil {
		return fallback
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return fallback
	}
	return t
}

func (s *Store) SetCursor(ctx context.Context, t time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO meta(key,value) VALUES('date_modified',$1)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, t.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpsertProcedure(ctx context.Context, p prozorro.Procedure, raw json.RawMessage) error {
	title, desc := p.Text()
	mod := prozorro.ParseTime(p.DateModified)
	start := prozorro.ParseTime(p.AuctionPeriod.StartDate)
	var startPtr *time.Time
	if !start.IsZero() {
		startPtr = &start
	}
	var modPtr *time.Time
	if !mod.IsZero() {
		modPtr = &mod
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO procedures(id,auction_id,selling_method,selling_entity,status,start_amount,currency,title,description,auction_start,date_modified,previous_auction_id,tender_attempts,raw)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			auction_id=EXCLUDED.auction_id,
			selling_method=EXCLUDED.selling_method,
			selling_entity=EXCLUDED.selling_entity,
			status=EXCLUDED.status,
			start_amount=EXCLUDED.start_amount,
			currency=EXCLUDED.currency,
			title=EXCLUDED.title,
			description=EXCLUDED.description,
			auction_start=EXCLUDED.auction_start,
			date_modified=EXCLUDED.date_modified,
			previous_auction_id=EXCLUDED.previous_auction_id,
			tender_attempts=EXCLUDED.tender_attempts,
			raw=EXCLUDED.raw`,
		p.ID, p.AuctionID, p.SellingMethod, p.EntityName(), p.Status,
		p.Value.Amount, p.Value.Currency, title, desc, startPtr, modPtr,
		nullIfEmpty(p.PreviousAuctionID), p.TenderAttempts, raw)
	return err
}

func (s *Store) Unmatched(ctx context.Context, limit int) ([]Row, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT p.id, p.auction_id, p.selling_method, COALESCE(p.selling_entity,''), COALESCE(p.status,''),
		       COALESCE(p.start_amount,0), COALESCE(p.currency,''), COALESCE(p.title,''), COALESCE(p.description,''),
		       p.auction_start, p.date_modified, COALESCE(p.previous_auction_id,''), COALESCE(p.tender_attempts,0)
		FROM procedures p
		LEFT JOIN lot_attrs a ON a.procedure_id=p.id
		WHERE a.procedure_id IS NULL
		ORDER BY p.date_modified DESC NULLS LAST
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *Store) SaveAttrs(ctx context.Context, id string, a parse.Attrs, markaID, modelID, cityID int) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO lot_attrs(procedure_id,kind,rooms,area,year,city,brand,model,marka_id,model_id,city_id,parse_confidence,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now())
		ON CONFLICT (procedure_id) DO UPDATE SET
			kind=EXCLUDED.kind, rooms=EXCLUDED.rooms, area=EXCLUDED.area, year=EXCLUDED.year,
			city=EXCLUDED.city, brand=EXCLUDED.brand, model=EXCLUDED.model,
			marka_id=EXCLUDED.marka_id, model_id=EXCLUDED.model_id, city_id=EXCLUDED.city_id,
			parse_confidence=EXCLUDED.parse_confidence, updated_at=now()`,
		id, string(a.Kind), nullInt(a.Rooms), nullFloat(a.Area), nullInt(a.Year),
		nullIfEmpty(a.City), nullIfEmpty(a.Brand), nullIfEmpty(a.Model),
		nullInt(markaID), nullInt(modelID), nullInt(cityID), a.Confidence)
	return err
}

func (s *Store) SaveScore(ctx context.Context, id string, discount *float64, pass int, family string, median float64) error {
	var d any
	if discount != nil {
		d = *discount
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO scores(procedure_id,discount_pct,pass_n,auction_family,ria_median,scored_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT (procedure_id) DO UPDATE SET
			discount_pct=EXCLUDED.discount_pct, pass_n=EXCLUDED.pass_n,
			auction_family=EXCLUDED.auction_family, ria_median=EXCLUDED.ria_median, scored_at=now()`,
		id, d, pass, family, nullFloat(median))
	return err
}

func (s *Store) StaleScores(ctx context.Context, limit int) ([]Row, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT p.id, p.auction_id, p.selling_method, COALESCE(p.selling_entity,''), COALESCE(p.status,''),
		       COALESCE(p.start_amount,0), COALESCE(p.currency,''), COALESCE(p.title,''), COALESCE(p.description,''),
		       p.auction_start, p.date_modified, COALESCE(p.previous_auction_id,''), COALESCE(p.tender_attempts,0)
		FROM procedures p
		JOIN lot_attrs a ON a.procedure_id=p.id
		LEFT JOIN scores s ON s.procedure_id=p.id
		WHERE a.kind IN ('car','apt')
		  AND COALESCE(p.status,'') NOT IN ('complete','cancelled','unsuccessful','deleted')
		  AND (s.scored_at IS NULL OR s.scored_at < now() - interval '24 hours')
		ORDER BY p.date_modified DESC NULLS LAST
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *Store) Snapshot(ctx context.Context, hash string) (ria.Snapshot, bool, error) {
	var sn ria.Snapshot
	var fetched time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT median, arithmetic, currency, fetched_at FROM price_snapshots WHERE spec_hash=$1`, hash).
		Scan(&sn.Median, &sn.Arithmetic, &sn.Currency, &fetched)
	if err == pgx.ErrNoRows {
		return sn, false, nil
	}
	if err != nil {
		return sn, false, err
	}
	if time.Since(fetched) > 24*time.Hour {
		return sn, false, nil
	}
	return sn, true, nil
}

func (s *Store) SaveSnapshot(ctx context.Context, hash string, sn ria.Snapshot) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO price_snapshots(spec_hash,median,arithmetic,currency,fetched_at)
		VALUES($1,$2,$3,$4,now())
		ON CONFLICT (spec_hash) DO UPDATE SET
			median=EXCLUDED.median, arithmetic=EXCLUDED.arithmetic, currency=EXCLUDED.currency, fetched_at=now()`,
		hash, sn.Median, sn.Arithmetic, sn.Currency)
	return err
}

func (s *Store) ReplaceMarks(ctx context.Context, marks []ria.Mark) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM ria_dicts WHERE kind='mark'`); err != nil {
		return err
	}
	for _, m := range marks {
		if _, err := tx.Exec(ctx, `INSERT INTO ria_dicts(kind,ria_id,name) VALUES('mark',$1,$2)`, m.ID, m.Name); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) Marks(ctx context.Context) ([]ria.Mark, error) {
	rows, err := s.Pool.Query(ctx, `SELECT ria_id, name FROM ria_dicts WHERE kind='mark'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ria.Mark
	for rows.Next() {
		var m ria.Mark
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type Filter struct {
	Kind        string
	City        string
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

func (s *Store) List(ctx context.Context, f Filter) ([]Row, error) {
	q := `
		SELECT p.id, p.auction_id, p.selling_method, COALESCE(p.selling_entity,''), COALESCE(p.status,''),
		       COALESCE(p.start_amount,0), COALESCE(p.currency,''), COALESCE(p.title,''), COALESCE(p.description,''),
		       p.auction_start, p.date_modified, COALESCE(p.previous_auction_id,''), COALESCE(p.tender_attempts,0),
		       COALESCE(a.kind,''), COALESCE(a.rooms,0), COALESCE(a.area,0), COALESCE(a.year,0), COALESCE(a.city,''),
		       COALESCE(a.brand,''), COALESCE(a.parse_confidence,0),
		       s.discount_pct, COALESCE(s.pass_n,0), COALESCE(s.auction_family,''), s.ria_median
		FROM procedures p
		JOIN lot_attrs a ON a.procedure_id=p.id
		LEFT JOIN scores s ON s.procedure_id=p.id
		WHERE a.kind IN ('car','apt')`
	args := []any{}
	n := 1
	if f.Kind != "" {
		q += ` AND a.kind=$` + strconv.Itoa(n)
		args = append(args, f.Kind)
		n++
	}
	if f.City != "" {
		q += ` AND a.city ILIKE $` + strconv.Itoa(n)
		args = append(args, "%"+f.City+"%")
		n++
	}
	if f.MinDiscount > 0 {
		q += ` AND s.discount_pct >= $` + strconv.Itoa(n)
		args = append(args, f.MinDiscount)
		n++
	}
	if f.Pass > 0 {
		q += ` AND s.pass_n = $` + strconv.Itoa(n)
		args = append(args, f.Pass)
		n++
	}
	if f.Family != "" {
		q += ` AND s.auction_family = $` + strconv.Itoa(n)
		args = append(args, f.Family)
		n++
	}
	q += ` ORDER BY s.discount_pct DESC NULLS LAST, p.auction_start ASC NULLS LAST LIMIT 200`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanList(rows)
}

func (s *Store) Get(ctx context.Context, id string) (Row, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT p.id, p.auction_id, p.selling_method, COALESCE(p.selling_entity,''), COALESCE(p.status,''),
		       COALESCE(p.start_amount,0), COALESCE(p.currency,''), COALESCE(p.title,''), COALESCE(p.description,''),
		       p.auction_start, p.date_modified, COALESCE(p.previous_auction_id,''), COALESCE(p.tender_attempts,0),
		       COALESCE(a.kind,''), COALESCE(a.rooms,0), COALESCE(a.area,0), COALESCE(a.year,0), COALESCE(a.city,''),
		       COALESCE(a.brand,''), COALESCE(a.parse_confidence,0),
		       s.discount_pct, COALESCE(s.pass_n,0), COALESCE(s.auction_family,''), s.ria_median
		FROM procedures p
		LEFT JOIN lot_attrs a ON a.procedure_id=p.id
		LEFT JOIN scores s ON s.procedure_id=p.id
		WHERE p.id=$1`, id)
	var r Row
	err := row.Scan(
		&r.ID, &r.AuctionID, &r.SellingMethod, &r.SellingEntity, &r.Status,
		&r.StartAmount, &r.Currency, &r.Title, &r.Description,
		&r.AuctionStart, &r.DateModified, &r.PreviousID, &r.TenderAttempts,
		&r.Kind, &r.Rooms, &r.Area, &r.Year, &r.City, &r.Brand, &r.Confidence,
		&r.Discount, &r.PassN, &r.Family, &r.RIAMedian,
	)
	return r, err
}

func scanRows(rows pgx.Rows) ([]Row, error) {
	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(
			&r.ID, &r.AuctionID, &r.SellingMethod, &r.SellingEntity, &r.Status,
			&r.StartAmount, &r.Currency, &r.Title, &r.Description,
			&r.AuctionStart, &r.DateModified, &r.PreviousID, &r.TenderAttempts,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanList(rows pgx.Rows) ([]Row, error) {
	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(
			&r.ID, &r.AuctionID, &r.SellingMethod, &r.SellingEntity, &r.Status,
			&r.StartAmount, &r.Currency, &r.Title, &r.Description,
			&r.AuctionStart, &r.DateModified, &r.PreviousID, &r.TenderAttempts,
			&r.Kind, &r.Rooms, &r.Area, &r.Year, &r.City, &r.Brand, &r.Confidence,
			&r.Discount, &r.PassN, &r.Family, &r.RIAMedian,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullFloat(n float64) any {
	if n == 0 {
		return nil
	}
	return n
}
