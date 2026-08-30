package jobs

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"skarbnyk/internal/parse"
	"skarbnyk/internal/store"
)

func alertWorthy(a parse.Attrs, discount float64) bool {
	if os.Getenv("TELEGRAM_BOT_TOKEN") == "" || os.Getenv("TELEGRAM_CHAT_ID") == "" {
		return false
	}
	return a.Confidence >= envFloat("ALERT_MIN_CONFIDENCE", 0.6) && discount >= envFloat("ALERT_MIN_DISCOUNT", 0.25)
}

// telegram never uses http.DefaultClient: an untimed send would hold the Match mutex forever.
var telegram = &http.Client{Timeout: 10 * time.Second}

// alert reports whether the message reached Telegram, so a failed send can be retried.
func alert(ctx context.Context, row store.Row, discount, market float64) bool {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chat := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chat == "" {
		return false
	}
	title := row.Title
	if title == "" {
		title = row.AuctionID
	}
	text := fmt.Sprintf("Скарбник: %.0f%% нижче ринку\n%s\nстарт %.0f %s · ринок %.0f грн\nhttps://prozorro.sale/auction/%s",
		discount*100, title, row.StartAmount, row.Currency, market, row.AuctionID)
	u := "https://api.telegram.org/bot" + token + "/sendMessage"
	form := url.Values{"chat_id": {chat}, "text": {text}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("telegram: %s", strings.ReplaceAll(err.Error(), token, "***"))
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := telegram.Do(req)
	if err != nil {
		log.Printf("telegram: %s", strings.ReplaceAll(err.Error(), token, "***"))
		return false
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("telegram %s: %d", row.ID, res.StatusCode)
		return false
	}
	return true
}

func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
