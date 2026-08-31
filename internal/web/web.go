package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
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
		"mul":    func(a, b float64) float64 { return a * b },
		"kindUA": kindUA,
	}).ParseFS(tmplFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{Store: st, T: t}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("GET /favicon.ico", serveFavicon)
	mux.HandleFunc("GET /", s.list)
	mux.HandleFunc("GET /lots/{id}", s.detail)
	return mux
}

func serveFavicon(w http.ResponseWriter, _ *http.Request) {
	b, err := staticFS.ReadFile("static/favicon.ico")
	if err != nil {
		http.Error(w, "немає", 404)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.Filter{
		Kind:   q.Get("type"),
		City:   q.Get("city"),
		Entity: q.Get("entity"),
		Family: q.Get("family"),
	}
	if v := q.Get("min"); v != "" {
		f.MinDiscount, _ = strconv.ParseFloat(v, 64)
	}
	if v := q.Get("pass"); v != "" {
		f.Pass, _ = strconv.Atoi(v)
	}
	page := 1
	if v := q.Get("page"); v != "" {
		page, _ = strconv.Atoi(v)
	}
	limit := parseLimit(q.Get("n"))
	total, err := s.Store.Count(r.Context(), f)
	if err != nil {
		log.Printf("count: %v", err)
		http.Error(w, "внутрішня помилка", 500)
		return
	}
	pages := pagesOf(total, limit)
	page = clampPage(page, pages)
	lots, err := s.Store.List(r.Context(), f, limit, (page-1)*limit)
	if err != nil {
		log.Printf("list: %v", err)
		http.Error(w, "внутрішня помилка", 500)
		return
	}
	data := map[string]any{
		"Lots": lots, "Filter": f,
		"Page": page, "Pages": pages, "Total": total, "Limit": limit, "PageSizes": pageSizes,
		"Prev": pageURL(f, page-1, limit), "Next": pageURL(f, page+1, limit),
	}
	name := "list"
	if r.Header.Get("HX-Request") == "true" {
		name = "results"
	} else {
		cities, err := s.Store.Cities(r.Context())
		if err != nil {
			log.Printf("cities: %v", err)
		}
		data["Cities"] = cities
		passes, err := s.Store.Passes(r.Context())
		if err != nil {
			log.Printf("passes: %v", err)
		}
		data["Passes"] = passes
		entities, err := s.Store.Entities(r.Context())
		if err != nil {
			log.Printf("entities: %v", err)
		}
		data["Entities"] = entities
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.T.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "внутрішня помилка", 500)
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
		log.Printf("render detail: %v", err)
		http.Error(w, "внутрішня помилка", 500)
	}
}

var pageSizes = []int{25, 50, 100}

const defaultLimit = 50

func parseLimit(s string) int {
	n, _ := strconv.Atoi(s)
	for _, ok := range pageSizes {
		if n == ok {
			return n
		}
	}
	return defaultLimit
}

func pagesOf(total, limit int) int {
	if total <= 0 || limit <= 0 {
		return 1
	}
	return (total + limit - 1) / limit
}

func clampPage(p, pages int) int {
	if p < 1 {
		return 1
	}
	if p > pages {
		return pages
	}
	return p
}

func pageURL(f store.Filter, page, limit int) string {
	q := url.Values{}
	if f.Kind != "" {
		q.Set("type", f.Kind)
	}
	if f.City != "" {
		q.Set("city", f.City)
	}
	if f.Entity != "" {
		q.Set("entity", f.Entity)
	}
	if f.MinDiscount > 0 {
		q.Set("min", strconv.FormatFloat(f.MinDiscount, 'f', -1, 64))
	}
	if f.Family != "" {
		q.Set("family", f.Family)
	}
	if f.Pass > 0 {
		q.Set("pass", strconv.Itoa(f.Pass))
	}
	if limit != defaultLimit && limit > 0 {
		q.Set("n", strconv.Itoa(limit))
	}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	if enc := q.Encode(); enc != "" {
		return "/?" + enc
	}
	return "/"
}

func kindUA(k string) string {
	switch k {
	case "car":
		return "авто"
	case "apt":
		return "квартира"
	default:
		return k
	}
}
