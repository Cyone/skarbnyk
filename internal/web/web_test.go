package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skarbnyk/internal/store"
)

func TestFavicon(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "icon") {
		t.Fatalf("status=%d type=%s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() < 20 {
		t.Fatal("empty icon")
	}
}

func TestKindUA(t *testing.T) {
	if kindUA("car") != "авто" || kindUA("apt") != "квартира" {
		t.Fatalf("car=%s apt=%s", kindUA("car"), kindUA("apt"))
	}
}

func TestListRenders(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.T.ExecuteTemplate(&buf, "list", map[string]any{
		"Lots":      []store.Row{{Title: "тест", AuctionID: "UA-1", Currency: "UAH"}},
		"Filter":    store.Filter{},
		"Cities":    []string{"Київ", "Львів"},
		"Entities":  []string{"ДП «СЕТАМ»", "ФДМУ"},
		"Passes":    []int{1, 2, 11},
		"Page":      2,
		"Pages":     4,
		"Limit":     50,
		"PageSizes": []int{25, 50, 100},
		"Prev":      "/",
		"Next":      "/?page=3",
	}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Шукати") || !strings.Contains(out, "table-wrap") || !strings.Contains(out, `class="wrap"`) || !strings.Contains(out, "/favicon.ico") {
		t.Fatal("list chrome missing")
	}
	if !strings.Contains(out, `name="city"`) || !strings.Contains(out, "<li>Київ</li>") {
		t.Fatal("city suggest missing")
	}
	if !strings.Contains(out, `name="entity"`) || !strings.Contains(out, "Організатор") || !strings.Contains(out, "ДП «СЕТАМ»") {
		t.Fatal("entity filter missing")
	}
	if !strings.Contains(out, "Сторінка 2 з 4") || !strings.Contains(out, "Далі") || !strings.Contains(out, "Назад") {
		t.Fatal("pager missing")
	}
	if !strings.Contains(out, `id="results"`) || !strings.Contains(out, `hx-target="#results"`) || !strings.Contains(out, `hx-swap="outerHTML"`) {
		t.Fatal("htmx results target missing")
	}
	if strings.Contains(out, `name="page"`) {
		t.Fatal("search must not keep page")
	}
	if !strings.Contains(out, `<select name="pass">`) || !strings.Contains(out, `value="11"`) {
		t.Fatal("pass must be a select of stored values")
	}
	if !strings.Contains(out, `id="per-page"`) || !strings.Contains(out, "На сторінці") || !strings.Contains(out, `hx-include="#per-page"`) {
		t.Fatal("page size must sit on the pager")
	}
}

func TestPageURL(t *testing.T) {
	f := store.Filter{Kind: "car", City: "Київ"}
	if pageURL(f, 1, 50) != "/?city=%D0%9A%D0%B8%D1%97%D0%B2&type=car" {
		t.Fatalf("page1=%s", pageURL(f, 1, 50))
	}
	if pageURL(f, 2, 50) != "/?city=%D0%9A%D0%B8%D1%97%D0%B2&page=2&type=car" {
		t.Fatalf("page2=%s", pageURL(f, 2, 50))
	}
	if pageURL(f, 1, 25) != "/?city=%D0%9A%D0%B8%D1%97%D0%B2&n=25&type=car" {
		t.Fatalf("n25=%s", pageURL(f, 1, 25))
	}
	full := store.Filter{Kind: "apt", City: "Одеса", Entity: "СЕТАМ", MinDiscount: 0.25, Family: "dutch", Pass: 2}
	if pageURL(full, 3, 100) != "/?city=%D0%9E%D0%B4%D0%B5%D1%81%D0%B0&entity=%D0%A1%D0%95%D0%A2%D0%90%D0%9C&family=dutch&min=0.25&n=100&page=3&pass=2&type=apt" {
		t.Fatalf("full=%s", pageURL(full, 3, 100))
	}
	if parseLimit("") != 50 || parseLimit("25") != 25 || parseLimit("100") != 100 || parseLimit("7") != 50 || parseLimit("abc") != 50 {
		t.Fatal("parseLimit")
	}
	if pagesOf(0, 50) != 1 || pagesOf(50, 50) != 1 || pagesOf(51, 50) != 2 {
		t.Fatal("pagesOf")
	}
	if clampPage(0, 3) != 1 || clampPage(9, 3) != 3 {
		t.Fatal("clampPage")
	}
}

func TestDetailKindUkrainian(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.T.ExecuteTemplate(&buf, "detail", map[string]any{
		"Lot":      store.Row{Kind: "apt", Title: "тест"},
		"Official": "/",
	}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "квартира") {
		t.Fatal("missing український тип")
	}
	if strings.Contains(out, ">apt<") {
		t.Fatal("raw kind leaked")
	}
}
