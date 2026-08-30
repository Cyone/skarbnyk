# Скарбник (Skarbnyk)

Bargain finder for [Prozorro.Sale](https://prozorro.sale/) lots, scored against [AUTO.RIA](https://auto.ria.com/) / [DIM.RIA](https://dom.ria.com/) market prices.

In Carpathian folklore, Скарбник sits on hidden treasure. Here it sits on public auctions and flags cars and apartments that start well below the street.

One Go binary: poller + matcher + HTMX UI. One Postgres. No SETAM scrape, no bidding, no second frontend process.

```
Prozorro.Sale CDB → poller → classify / parse → RIA average_price → score → HTML/HTMX
                                                                  ↘ Telegram (optional)
```

## Run

Needs Docker (Postgres) and Go 1.24+.

```bash
make run
```

Opens the UI on [http://localhost:8080](http://localhost:8080). Without Docker: start Postgres yourself, then `go run ./cmd/skarbnyk`.

| Env | Default | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://skarbnyk:skarbnyk@localhost:5432/skarbnyk?sslmode=disable` | |
| `HTTP_ADDR` | `:8080` | |
| `RIA_API_KEY` | empty | From [developers.ria.com](https://developers.ria.com/). Without it, lots list with no market scores. |
| `USD_UAH` | `41.5` | AUTO.RIA medians are USD; converted to грн. |
| `POLL_SINCE` | last 14 days | `YYYY-MM-DD` first-run window. After that the cursor advances. |
| `TELEGRAM_BOT_TOKEN` | empty | Optional alerts. |
| `TELEGRAM_CHAT_ID` | empty | Required with the token. |
| `ALERT_MIN_DISCOUNT` | `0.25` | 0–1 (25% below market). |
| `ALERT_MIN_CONFIDENCE` | `0.6` | Skip junk parses. |

```bash
make test
go run ./cmd/skarbnyk -check
```

Free RIA pack is 1,000 calls/month, 30/hour. Cache is one call per unique spec, not per lot.

## Implemented

**Ingest**

- Incremental `GET /api/search/byDateModified` poll every 3 minutes (max 40 pages/run, 1s between pages).
- Upsert by procedure `_id`; re-pull when `dateModified` moves.
- Cursor in Postgres. First run uses `POLL_SINCE` or the last 14 days.
- Stores selling method, selling entity, start amount, `previousAuctionId`, `tenderAttempts`, raw JSON.
- SETAM-organized rows are ingested only if they already appear in the Prozorro CDB. No setam.net.ua catalog.

**Classify and score**

- Rule-based parser: car / apartment / skip (scrap, land-only, rights-of-claim, mixed junk).
- Cars: CAV `34*`, title keywords, VIN, common marks. Apartments: CPV `04*`/`70*`, «квартира», Ukrainian room words (`Двокімнатна` → 2), area, city.
- Houses currently land as apartments (cheap reuse of the apt path).
- Discount `1 − start / RIA median`. USD comps converted with `USD_UAH`.
- Auction family: English / Dutch / other. Repeat pass from `tenderAttempts` or `previousAuctionId`.
- Score the **current** start. Dutch in-auction ticks are not treated as a markdown.
- `parse_confidence < 0.6` is stored without a market score.

**RIA**

- Cached mark dictionaries (daily refresh).
- Freemium `auto/average_price` and `dom/average_price`.
- Spec-hash cache, 24h TTL; daily drop + re-score of open lots.
- Paid AI / period series: not used.

**UI**

- `GET /` list with HTMX filters: type, city, min discount, English/Dutch, pass.
- `GET /lots/{id}` detail: start vs RIA median, confidence, family, organizer, official Prozorro URL.
- Ukrainian copy. RIA attribution in the footer.
- No login.

**Alerts**

- Telegram when confidence and discount both clear the thresholds.
- Email is not wired.

**Ops**

- Schema embed + migrate on boot. `docker-compose.yml` for local Postgres.
- `skarbnyk -check` parses a known apt/car string and loads templates.

## Next

Ordered by leverage, not by ceremony.

1. **Tighter RIA specs** — map parsed model → `model_id`, city → `city_id`. Today only mark + year (cars) and rooms (apts) hit the API, so medians are wide.
2. **Houses as their own kind** — parser already sees «будинок»; split from apartments and call the matching DIM.RIA category.
3. **List polish** — auction start time on the card, skip/low-confidence filter, selling-entity chip (so a SETAM-organized row is visible if it showed up).
4. **JSON `GET /api/lots`** — same filters as the HTML list, for a phone client later.
5. **Email alerts** — same rule as Telegram (`confidence ≥ X` and `discount ≥ Y`).
6. **Classifier misses** — count skip vs car/apt on a week of live data; LLM only if the miss rate hurts.
7. **Land** — only if you want it; land-only lots are skipped on purpose.
8. **Deploy** — one VPS (Hetzner / Fly / UA), `RIA_API_KEY` server-side only. ~5–10 € + 0–500 грн/mo RIA.

## Out of scope

- Scraping or accreditation for [setam.net.ua](https://setam.net.ua/). That catalog is not in the Prozorro CDB.
- Bidding, deposits, becoming a майданчик.
- Auth, accounts, WASM, a second frontend, Celery/Kafka/Elasticsearch.

## Cost

0–500 грн/mo RIA + ~5–10 € VPS if you host it.
