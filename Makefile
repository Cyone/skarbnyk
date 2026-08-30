.PHONY: db test run

db:
	docker compose up -d

test:
	go test ./...

run: db
	DATABASE_URL=postgres://skarbnyk:skarbnyk@localhost:5432/skarbnyk?sslmode=disable \
		go run ./cmd/skarbnyk

# Optional: RIA_API_KEY, USD_UAH, POLL_SINCE, TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID,
# ALERT_MIN_DISCOUNT (0–1), ALERT_MIN_CONFIDENCE, HTTP_ADDR
