package web

import (
	"bytes"
	"strings"
	"testing"

	"skarbnyk/internal/store"
)

func TestKindUA(t *testing.T) {
	if kindUA("car") != "авто" || kindUA("apt") != "квартира" {
		t.Fatalf("car=%s apt=%s", kindUA("car"), kindUA("apt"))
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
