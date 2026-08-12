# 🌋 Lindol API

[![CI](https://github.com/debiangee/phivolcs-lindol/actions/workflows/ci.yml/badge.svg)](https://github.com/debiangee/phivolcs-lindol/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev)

A Philippine earthquake alert microservice that detects seismic events from **PHIVOLCS** and **USGS**, normalizes the data, and serves it as a clean REST API with real-time notifications.

> **lindol** (Tagalog) — earthquake

> [!WARNING]
> **Testing deployment:** The hosted API is currently provided for testing and validation only. It is not a production service or an official PHIVOLCS endpoint. Availability, data coverage, and API behavior may change without notice.

## Why?

PHIVOLCS doesn't offer a public API. This service fills that gap by:

- Scraping PHIVOLCS for all PH earthquakes (M1.0+) with smart stop-on-known logic
- Polling USGS as a reliable fallback for significant quakes (M4.0+)
- Normalizing messy HTML data through a transformer pipeline
- Exposing a clean, rate-limited JSON API for app developers
- Sending instant notifications (Telegram, Discord, webhooks)

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                         EC2 / VPS                            │
│                                                             │
│  PHIVOLCS (every 5 min) ──▶ Transformer ──▶ DB             │
│      ↕ stops at known entry                                 │
│  USGS (every 2 min) ──────────────────────▶ DB             │
│                                               │             │
│                                               ▼             │
│                              REST API (rate limited: 1/min) │
│                                               │             │
│                            Notifications ──▶ Telegram       │
│                                          ──▶ Discord        │
│                                          ──▶ Webhook        │
└──────────────────────────────────┬──────────────────────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
               Your App       Dashboard      Another Service
```

**Key behaviors:**
- PHIVOLCS entries are ordered newest-first. Once the scraper hits an entry already in the database, it **stops** — no need to process thousands of old entries every time.
- Only **one hosted instance** scrapes PHIVOLCS. App developers consume the REST API.
- Every parse failure triggers a **dev alert** so you know immediately when PHIVOLCS changes their HTML.

## Tech Stack

| Layer | Choice |
|-------|--------|
| Language | Go |
| HTTP | net/http (stdlib) |
| Database | SQLite (CGo) |
| Scraping | goquery |
| Data cleaning | Custom transformer/normalizer |
| Logging | log/slog (stdlib) |
| Notifications | Telegram, Discord, Webhook |
| Rate Limiting | Per-IP token bucket (1 req/min) |

**Why Go?**
- Single binary — no runtime dependencies
- ~20 MB binary, ~10 MB idle RAM
- Deploy anywhere: Docker, bare metal, Raspberry Pi
- Download from GitHub Releases and run — that's it

## API Endpoints

```
GET  /                         — Testing deployment notice
GET  /api/earthquakes          — List recent earthquakes (paginated)
GET  /api/earthquakes/latest   — Get the most recent event
GET  /api/earthquakes/{id}     — Get earthquake detail
GET  /api/health               — Service health + source status
GET  /api/status               — Stats (total count, last poll times, uptime)
```

### Query Parameters

| Param | Description | Example |
|-------|-------------|---------|
| `minMagnitude` | Minimum magnitude | `3.0` |
| `maxMagnitude` | Maximum magnitude | `7.0` |
| `startDate` | Start date (ISO 8601 or YYYY-MM-DD) | `2026-08-01` |
| `endDate` | End date | `2026-08-11` |
| `limit` | Results per page (max 100) | `10` |
| `offset` | Pagination offset | `20` |

### Response Format

```json
{
  "data": [
    {
      "id": "phivolcs_ef6f6db5bccced09",
      "magnitude": 4.6,
      "latitude": 5.21,
      "longitude": 125.23,
      "depth_km": 10,
      "event_time": "2026-08-06T05:41:00Z",
      "location_description": "040 km SW of Sarangani (Davao Occidental)",
      "phivolcs_bulletin_url": "https://earthquake.phivolcs.dost.gov.ph/...",
      "enriched": false,
      "created_at": "2026-08-06T05:46:00Z",
      "updated_at": "2026-08-06T05:46:00Z"
    }
  ],
  "total": 3913,
  "limit": 20,
  "offset": 0,
  "has_more": true
}
```

### Rate Limiting

The public API is rate limited to **1 request per minute per IP**.

If you exceed the limit, you'll get:
```json
HTTP 429
{"error": "Rate limit exceeded. Maximum 1 request per minute.", "retry_after_sec": 60}
```

## Data Sources

### PHIVOLCS (Primary — on hosted instance)
- Endpoint: `https://earthquake.phivolcs.dost.gov.ph/`
- Format: HTML (scraped + transformed)
- Coverage: All PH earthquakes (M1.0+)
- Poll interval: Every 5 minutes
- Smart stop: Stops processing once it hits a known entry

### USGS (Fallback)
- Endpoint: `https://earthquake.usgs.gov/fdsnws/event/1/query`
- Format: GeoJSON (clean API)
- Coverage: PH region M4.0+ only
- Poll interval: Every 2 minutes

## Project Structure

```
lindol-api/
├── cmd/server/main.go              — Entry point, pollers, graceful shutdown
├── internal/
│   ├── config/config.go            — Environment config with defaults
│   ├── database/
│   │   ├── database.go             — SQLite + embedded migration runner
│   │   └── migrations/             — SQL schema files
│   ├── models/earthquake.go        — Data structures + USGS GeoJSON parsing
│   ├── transform/normalize.go      — Data cleaning + validation pipeline
│   ├── sources/
│   │   ├── usgs.go                 — USGS API client
│   │   └── phivolcs.go             — PHIVOLCS scraper + matcher
│   ├── services/
│   │   ├── earthquake.go           — USGS detection + DB queries
│   │   ├── phivolcs_poller.go      — PHIVOLCS primary polling + stop-on-known
│   │   ├── enrichment.go           — PHIVOLCS enrichment (on-demand)
│   │   ├── notification.go         — Alert dispatcher + dev alerts
│   │   └── health.go               — Source health tracking
│   ├── notifications/
│   │   ├── telegram.go             — Telegram Bot API
│   │   ├── discord.go              — Discord webhooks
│   │   └── webhook.go              — Generic JSON webhook
│   ├── server/
│   │   ├── server.go               — HTTP server + CORS
│   │   ├── routes.go               — API handlers + validation
│   │   └── ratelimit.go            — Per-IP rate limiter
│   └── utils/retry.go              — Exponential backoff
├── examples/                       — Integration guides
├── .github/workflows/              — CI + release automation
├── Dockerfile                      — Multi-stage (~15 MB image)
├── docker-compose.yml
└── .env.example
```

## Getting Started

### Prerequisites

- Go 1.23+ (for development)
- OR Docker (for deployment)

### Run from Source

```bash
git clone https://github.com/debiangee/phivolcs-lindol.git
cd phivolcs-lindol
cp .env.example .env    # edit with your config
go run ./cmd/server
```

### Run Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out
```

### Run locally with Docker

For local development, keep the API bound to localhost and load secrets from `.env` rather than putting them in shell history:

```bash
cp .env.example .env
# Edit .env locally; never commit it.
docker run --rm --env-file .env -p 127.0.0.1:3000:3000 ghcr.io/debiangee/lindol-api
```

### Production Docker Compose with HTTPS

The production Compose configuration runs Lindol behind Caddy. The API is available only inside the Docker network; Caddy exposes ports `80` and `443` and obtains/renews a Let’s Encrypt certificate automatically.

```bash
cp .env.example .env
# Edit .env with notification credentials. Keep this file private.
docker compose up -d --build
docker compose ps
docker compose logs -f caddy
```

The included `Caddyfile` uses the current hosted name:

```text
https://13-239-98-132.sslip.io/api/health
```

For another server, replace the hostname in `Caddyfile` with a domain whose DNS `A` record points to the server. Use an Elastic IP on AWS so the hostname does not change after a stop/start.

### AWS security-group rules

Use these inbound rules for the HTTPS deployment:

| Port | Source | Purpose |
|------|--------|---------|
| `22/tcp` | Your fixed IP `/32` only | SSH administration |
| `80/tcp` | `0.0.0.0/0` | HTTP-to-HTTPS redirect and certificate validation |
| `443/tcp` | `0.0.0.0/0` | HTTPS API traffic |
| `443/udp` | `0.0.0.0/0` (optional) | HTTP/3 |

Do **not** expose port `3000` publicly. It is intentionally not published by `docker-compose.yml`. Also restrict SSH in the AWS security group; do not use `0.0.0.0/0` for port `22`.

### Security checklist

- Never commit `.env`, private keys, certificates containing private material, databases, HAR captures, or local binaries.
- Use `--env-file .env` or Compose `env_file`; do not put bot tokens or webhook URLs directly in commands, images, or source files.
- Rotate notification tokens immediately if they are ever exposed.
- Keep `data/` and Caddy’s certificate volumes backed up securely; backups may contain operational or notification data.
- Verify the public deployment with `curl -I https://your-domain.example/api/health` and confirm that port `3000` is unreachable from the internet.
- Keep Docker, the host OS, and the AWS security group updated. Grant CI only the permissions required by its job.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 3000 | Internal server port |
| `ENV` | development | Environment (development/production) |
| `PHIVOLCS_PRIMARY` | false | Enable PHIVOLCS as primary source (for hosted instance) |
| `PHIVOLCS_POLL_INTERVAL_SEC` | 300 | PHIVOLCS poll interval |
| `USGS_POLL_INTERVAL_SEC` | 120 | USGS poll interval |
| `MIN_MAGNITUDE` | 2.5 | Minimum magnitude for USGS |
| `PHIVOLCS_DELAY_SEC` | 180 | Delay before PHIVOLCS enrichment |
| `TELEGRAM_BOT_TOKEN` | — | Telegram bot token; store only in `.env` |
| `TELEGRAM_CHAT_ID` | — | Telegram chat ID; store only in `.env` |
| `DISCORD_WEBHOOK_URL` | — | Discord webhook; store only in `.env` |
| `WEBHOOK_URL` | — | Generic webhook URL; store only in `.env` |

## Deployment Modes

### Public API (Hosted on EC2/VPS)

One centralized instance scrapes PHIVOLCS. Consumers use the HTTPS API instead of scraping PHIVOLCS themselves:

```
PHIVOLCS ◄── 1 req / 5 min ── Your Server ──▶ Caddy ──▶ HTTPS REST API
                                                    │
                                     ┌──────────────┼──────────┐
                                     ▼              ▼          ▼
                                App A          App B          App C
```

Set `PHIVOLCS_PRIMARY=true` for this mode. Keep the API container private and expose only the reverse proxy.

### Self-Hosted (USGS-only)

For developers who want their own instance without scraping PHIVOLCS:

- Uses only the USGS API (clean, no scraping)
- Reports M4.0+ earthquakes only
- Zero risk of being blocked
- Leave `PHIVOLCS_PRIMARY=false` (default)

## Notification Examples

**Earthquake alert:**
```
🚨 Earthquake Detected
Magnitude: 4.6
Location: 040 km SW of Sarangani
Coordinates: 5.21°N, 125.23°E
Depth: 10 km
Time: 06 Aug 2026 - 1:41 PM PHT
Source: PHIVOLCS
```

**Parser alert (dev notification):**
```
⚠️ Parser Alert
PHIVOLCS entry failed to parse.
Field: date
Raw value: `11 Agosto 2026 - 1:05 PM`
Error: no format matched
Action: Check if PHIVOLCS changed their HTML format.
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Examples

- [Telegram Setup Guide](examples/telegram-setup.md)
- [Webhook Integration](examples/webhook-integration.md)
- [API Usage Examples](examples/api-usage.md)

## Disclaimer

This project is **unofficial** and not affiliated with PHIVOLCS or USGS. Data is sourced from publicly available endpoints. Use at your own discretion.

## License

MIT

---

**Built by debiangee × Kiro**
