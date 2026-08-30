package jobs

import (
	"context"
	"testing"

	"skarbnyk/internal/parse"
	"skarbnyk/internal/store"
)

func TestSpecHashApt(t *testing.T) {
	one := parse.Attrs{Kind: parse.KindApt, Rooms: 1}
	two := parse.Attrs{Kind: parse.KindApt, Rooms: 2}
	if specHash(one, 0) == specHash(two, 0) {
		t.Fatal("rooms must split the cache key")
	}
	dated := parse.Attrs{Kind: parse.KindApt, Rooms: 2, Year: 2014}
	if specHash(two, 0) != specHash(dated, 0) {
		t.Fatal("year must not split an apartment key")
	}
}

func TestSpecHashCar(t *testing.T) {
	a := parse.Attrs{Kind: parse.KindCar, Year: 2014}
	if specHash(a, 9) == specHash(a, 79) {
		t.Fatal("marka must split the cache key")
	}
	older := parse.Attrs{Kind: parse.KindCar, Year: 2013}
	if specHash(a, 9) == specHash(older, 9) {
		t.Fatal("year must split a car key")
	}
	if specHash(a, 9) == specHash(parse.Attrs{Kind: parse.KindApt, Rooms: 9}, 9) {
		t.Fatal("kinds must not collide")
	}
}

func TestPriceable(t *testing.T) {
	if priceable(parse.Attrs{Kind: parse.KindCar, Year: 2014}, 0) {
		t.Fatal("car without marka")
	}
	if priceable(parse.Attrs{Kind: parse.KindCar}, 9) {
		t.Fatal("car without year")
	}
	if !priceable(parse.Attrs{Kind: parse.KindCar, Year: 2014}, 9) {
		t.Fatal("car with marka and year")
	}
	if priceable(parse.Attrs{Kind: parse.KindApt}, 0) {
		t.Fatal("apartment without rooms")
	}
	if !priceable(parse.Attrs{Kind: parse.KindApt, Rooms: 2}, 0) {
		t.Fatal("apartment with rooms")
	}
}

func TestAlertWorthy(t *testing.T) {
	a := parse.Attrs{Confidence: 0.8}
	if alertWorthy(a, 0.5) {
		t.Fatal("no telegram config")
	}
	t.Setenv("TELEGRAM_BOT_TOKEN", "t")
	t.Setenv("TELEGRAM_CHAT_ID", "c")
	if !alertWorthy(a, 0.5) {
		t.Fatal("above thresholds")
	}
	if alertWorthy(a, 0.1) {
		t.Fatal("below discount threshold")
	}
	if alertWorthy(parse.Attrs{Confidence: 0.3}, 0.5) {
		t.Fatal("below confidence threshold")
	}
}

func TestAlertReportsFailure(t *testing.T) {
	if telegram.Timeout == 0 {
		t.Fatal("untimed send would hold the match mutex forever")
	}
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if alert(context.Background(), store.Row{}, 0.5, 1) {
		t.Fatal("claimed a send without a bot configured")
	}
}
