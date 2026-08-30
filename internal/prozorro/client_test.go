package prozorro

import "testing"

func TestDecodeAndFamilyHints(t *testing.T) {
	raw := []byte(`{
		"_id":"abc",
		"auctionId":"AAE001-UA-2026-1",
		"sellingMethod":"arrestedAssets-english",
		"status":"active_tendering",
		"dateModified":"2026-08-01T12:00:00.000000Z",
		"title":{"uk_UA":"Автомобіль Toyota Camry 2018"},
		"value":{"amount":120000,"currency":"UAH"},
		"sellingEntity":{"name":{"uk_UA":"ДП «СЕТАМ»"},"identifier":{"id":"39958500"}},
		"items":[{"description":{"uk_UA":"легковий"},"classification":{"id":"34110000-1"}}],
		"auctionPeriod":{"startDate":"2026-08-10T09:00:00.000000Z"}
	}`)
	p, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.AuctionID != "AAE001-UA-2026-1" {
		t.Fatalf("id=%s", p.AuctionID)
	}
	if p.EntityName() != "ДП «СЕТАМ»" {
		t.Fatalf("entity=%s", p.EntityName())
	}
	if p.ClassID() != "34110000-1" {
		t.Fatalf("class=%s", p.ClassID())
	}
	title, _ := p.Text()
	if title == "" {
		t.Fatal("title")
	}
}
