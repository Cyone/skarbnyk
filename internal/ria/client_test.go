package ria

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMatchMark(t *testing.T) {
	marks := []Mark{{ID: 9, Name: "BMW"}, {ID: 79, Name: "Toyota"}}
	if MatchMark("BMW", marks) != 9 {
		t.Fatal("exact")
	}
	if MatchMark("toyota", marks) != 79 {
		t.Fatal("fold")
	}
	if MatchMark("", marks) != 0 {
		t.Fatal("empty")
	}
}

func TestSpecHashStable(t *testing.T) {
	if SpecHash("car", 9, 0, 2014, 0) != SpecHash("car", 9, 0, 2014, 0) {
		t.Fatal("stable")
	}
	if SpecHash("car", 9, 0, 2014, 0) == SpecHash("car", 9, 0, 2015, 0) {
		t.Fatal("year")
	}
}

const testKey = "top-secret-ria-key"

func TestGetHidesKeyOnTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := srv.URL
	srv.Close()

	c := &Client{Key: testKey, HTTP: &http.Client{}}
	_, err := c.get(context.Background(), endpoint, url.Values{})
	if err == nil {
		t.Fatal("want error from closed server")
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatalf("key leaked: %v", err)
	}
}

func TestGetHidesKeyOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key "+r.URL.Query().Get("api_key"), 500)
	}))
	defer srv.Close()

	c := &Client{Key: testKey, HTTP: srv.Client()}
	_, err := c.get(context.Background(), srv.URL, url.Values{})
	if err == nil {
		t.Fatal("want error from 500")
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatalf("key leaked: %v", err)
	}
}
