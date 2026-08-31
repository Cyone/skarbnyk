package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"skarbnyk/internal/jobs"
	"skarbnyk/internal/parse"
	"skarbnyk/internal/prozorro"
	"skarbnyk/internal/ria"
	"skarbnyk/internal/store"
	"skarbnyk/internal/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-check" {
		if err := selfcheck(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("ok")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://skarbnyk:skarbnyk@localhost:5432/skarbnyk?sslmode=disable"
	}
	st, err := store.Open(ctx, dbURL)
	if err != nil {
		log.Fatalf("db: %s", redactPassword(err.Error(), dbURL))
	}
	defer st.Close()

	srv, err := web.New(st)
	if err != nil {
		log.Fatalf("web: %v", err)
	}

	var riaClient *ria.Client
	if key := os.Getenv("RIA_API_KEY"); key != "" {
		riaClient = ria.New(key)
	} else {
		log.Print("RIA_API_KEY empty — lots will list without market scores")
	}

	run := &jobs.Runner{
		Store: st,
		PZ:    prozorro.New(),
		RIA:   riaClient,
		USD:   jobs.EnvUSD(),
		Since: jobs.DefaultSince(),
	}
	run.Start(ctx)

	addr := ":8080"
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		addr = v
	}
	h := &http.Server{Addr: addr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("listening %s", addr)
		if err := h.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = h.Shutdown(shutdown)
}

// redactPassword keeps a DSN parse failure from printing the database password.
func redactPassword(msg, dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return msg
	}
	pw, ok := u.User.Password()
	if !ok || pw == "" {
		return msg
	}
	return strings.ReplaceAll(msg, pw, "***")
}

func selfcheck() error {
	a := parse.Classify("Двокімнатна квартира загальною площею 44,97 кв.м. м. Київ", "", "")
	if a.Kind != parse.KindApt || a.Rooms != 2 {
		return fmt.Errorf("apt parse: %+v", a)
	}
	c := parse.Classify("Легковий автомобіль Toyota Camry 2018", "", "34110000-0")
	if c.Kind != parse.KindCar || c.Brand != "Toyota" || c.Year != 2018 {
		return fmt.Errorf("car parse: %+v", c)
	}
	if _, err := web.New(nil); err != nil {
		return fmt.Errorf("templates: %v", err)
	}
	return nil
}
