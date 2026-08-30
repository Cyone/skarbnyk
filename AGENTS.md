# Skarbnyk agent guidance

Bargain finder: Prozorro.Sale lots scored against AUTO.RIA / DIM.RIA medians.

```
Prozorro CDB → poller → parse → RIA average_price → score → HTMX
                                                      ↘ Telegram
```

One Go binary (`cmd/skarbnyk`), one Postgres. Schema embed + migrate on boot.

## Layout

- `internal/prozorro` — CDB poll (`byDateModified`)
- `internal/parse` — car / apt / skip
- `internal/ria` — cached `average_price`
- `internal/score` — discount, family, pass
- `internal/store` — Postgres
- `internal/jobs` — poll / match / refresh / alert
- `internal/web` — HTMX, Ukrainian copy

## Run and check

- `make run` — Docker Postgres + binary, UI on `:8080`
- `make test` — `go test ./...`
- `go run ./cmd/skarbnyk -check` — parser + templates smoke

## Out of scope

No SETAM scrape, bidding, auth, second frontend, extra process, or new dependency.

## Quality

After Go edits: `make test`. Parser or score changes need a test. Secrets (`RIA_API_KEY`, Telegram) stay in env.

Project review: skill `review`. Security: skill `review-security` or `/review-security`. Bugs: `/review-bugbot`.
