# Full path: PATH may already have a different `air` (JetBrains Toolbox).
GOBIN := $(shell go env GOPATH)/bin
AIR := $(GOBIN)/air

.PHONY: db test run

db:
	docker compose up -d

test:
	go test ./...

$(AIR):
	go install github.com/air-verse/air@v1.67.4

run: db $(AIR)
	DATABASE_URL=postgres://skarbnyk:skarbnyk@localhost:5432/skarbnyk?sslmode=disable \
		$(AIR)

# Optional: RIA_API_KEY, USD_UAH, POLL_SINCE, TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID,
# ALERT_MIN_DISCOUNT (0–1), ALERT_MIN_CONFIDENCE, HTTP_ADDR
