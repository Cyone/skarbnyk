package jobs

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"skarbnyk/internal/parse"
	"skarbnyk/internal/store"
)

func alert(row store.Row, a parse.Attrs, discount, market float64) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chat := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chat == "" {
		return
	}
	minD := envFloat("ALERT_MIN_DISCOUNT", 0.25)
	minC := envFloat("ALERT_MIN_CONFIDENCE", 0.6)
	if a.Confidence < minC || discount < minD {
		return
	}
	title := row.Title
	if title == "" {
		title = row.AuctionID
	}
	text := fmt.Sprintf("Скарбник: %.0f%% нижче ринку\n%s\nстарт %.0f %s · ринок %.0f грн\nhttps://prozorro.sale/auction/%s",
		discount*100, title, row.StartAmount, row.Currency, market, row.AuctionID)
	u := "https://api.telegram.org/bot" + token + "/sendMessage"
	form := url.Values{"chat_id": {chat}, "text": {text}}
	res, err := http.PostForm(u, form)
	if err != nil {
		log.Printf("telegram: %v", err)
		return
	}
	res.Body.Close()
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
