package web

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"skarbnyk/internal/store"
)

//go:embed templates/*.html
var tmplFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Server struct {
	Store *store.Store
	T     *template.Template
}

func New(st *store.Store) (*Server, error) {
	t, err := template.New("").Funcs(template.FuncMap{
		"deref": func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p
		},
		"percent": func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p * 100
		},
		"mul": func(a, b float64) float64 { return a * b },
	}).ParseFS(tmplFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{Store: st, T: t}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("GET /", s.list)
	mux.HandleFunc("GET /lots/{id}", s.detail)
	return mux
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.Filter{
		Kind:   q.Get("type"),
		City:   q.Get("city"),
		Family: q.Get("family"),
	}
	if v := q.Get("min"); v != "" {
		f.MinDiscount, _ = strconv.ParseFloat(v, 64)
	}
	if v := q.Get("pass"); v != "" {
		f.Pass, _ = strconv.Atoi(v)
	}
	lots, err := s.Store.List(r.Context(), f)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := map[string]any{"Lots": lots, "Filter": f}
	name := "list"
	if r.Header.Get("HX-Request") == "true" {
		name = "rows"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.T.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lot, err := s.Store.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "немає лота", 404)
		return
	}
	official := "https://prozorro.sale/auction/" + lot.AuctionID
	if strings.TrimSpace(lot.AuctionID) == "" {
		official = "https://prozorro.sale/"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.T.ExecuteTemplate(w, "detail", map[string]any{"Lot": lot, "Official": official}); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
