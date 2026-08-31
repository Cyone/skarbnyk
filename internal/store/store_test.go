package store

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestOpenMigrate(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL")
	}
	ctx := context.Background()
	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Count(ctx, Filter{}); err != nil {
		t.Fatal(err)
	}
}

func testDB() *bun.DB {
	return bun.NewDB(nil, pgdialect.New())
}

func TestListFilterSQL(t *testing.T) {
	q := listSelect(testDB(), Filter{
		Kind: "car", City: "Київ", Entity: "сетам", MinDiscount: 0.25, Pass: 2, Family: "dutch",
	})
	got := q.String()
	for _, want := range []string{
		"attrs.kind",
		"ILIKE",
		"selling_entity",
		"discount_pct",
		"pass_n",
		"auction_family",
		"complete",
		"cancelled",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sql missing %q:\n%s", want, got)
		}
	}
}

func TestPassesSQL(t *testing.T) {
	got := passesSelect(testDB()).String()
	for _, want := range []string{"pass_n", "lot_attrs", "complete"} {
		if !strings.Contains(got, want) {
			t.Fatalf("sql missing %q:\n%s", want, got)
		}
	}
}

func TestEntitiesSQL(t *testing.T) {
	got := entitiesSelect(testDB()).String()
	for _, want := range []string{"selling_entity", "lot_attrs", "complete"} {
		if !strings.Contains(got, want) {
			t.Fatalf("sql missing %q:\n%s", want, got)
		}
	}
}

func TestSaveAttrsConflictSQL(t *testing.T) {
	got := testDB().NewInsert().
		Model(&lotAttr{ProcedureID: "x", Kind: "car"}).
		On("CONFLICT (procedure_id) DO UPDATE").
		String()
	if !strings.Contains(got, "EXCLUDED") || !strings.Contains(got, "parse_confidence") {
		t.Fatalf("attrs upsert:\n%s", got)
	}
}

func TestUpsertKeepsZeroStartAmount(t *testing.T) {
	got := upsertProcedureQ(testDB(), &procedure{ID: "x"}).String()
	if !strings.Contains(got, `INSERT INTO "procedures" AS "p"`) {
		t.Fatalf("upsert table alias:\n%s", got)
	}
	if !strings.Contains(got, "NULLIF(p.start_amount, 0)") || !strings.Contains(got, "EXCLUDED.start_amount") {
		t.Fatalf("upsert lost start_amount guard:\n%s", got)
	}
}

func TestSaveScoreLeavesAlertedAt(t *testing.T) {
	got := saveScoreQ(testDB(), &lotScore{ProcedureID: "x"}).String()
	set := got
	if i := strings.Index(got, "DO UPDATE SET"); i >= 0 {
		set = got[i:]
	}
	if j := strings.Index(set, " RETURNING"); j >= 0 {
		set = set[:j]
	}
	if strings.Contains(set, "alerted_at") {
		t.Fatalf("rescore must not touch alerted_at:\n%s", got)
	}
}
