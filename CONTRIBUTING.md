# Contributing to Lindol API

Salamat sa interest mo! Contributions are welcome — whether it's a bug fix, new notification channel, or documentation improvement.

## Getting Started

### Prerequisites

- Go 1.22+
- Git
- A text editor / IDE with Go support

### Setup

```bash
# Clone the repo
git clone https://github.com/debiangee/lindol-api.git
cd lindol-api

# Copy environment config
cp .env.example .env

# Run the service
go run ./cmd/server
```

The service will start on `http://localhost:3000`. It immediately begins polling USGS for PH earthquakes.

### Verify your setup

```bash
curl http://localhost:3000/api/health
```

You should see a JSON response with `"status": "ok"`.

## Development Workflow

1. **Fork and clone** the repository
2. **Create a branch** from `main`:
   ```bash
   git checkout -b feat/your-feature
   ```
3. **Make your changes** — keep them focused on one thing
4. **Test locally** — run the service and verify behavior
5. **Run checks:**
   ```bash
   go build ./...
   go vet ./...
   go test ./...
   ```
6. **Commit** with a clear message:
   ```
   feat(phase-5): improve PHIVOLCS date parsing
   fix(notifications): handle Telegram rate limits
   docs: add webhook integration example
   ```
7. **Push and open a PR**

## Project Structure

```
cmd/server/          — Entry point
internal/
├── config/          — Environment config
├── database/        — SQLite + migrations
├── models/          — Data structures
├── notifications/   — Alert channels (Telegram, Discord, webhook)
├── server/          — HTTP server + routes
├── services/        — Business logic
├── sources/         — External data fetchers (USGS, PHIVOLCS)
└── utils/           — Shared utilities
```

## Guidelines

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `log/slog` for all logging — no `fmt.Println` in production code
- Wrap errors with context: `fmt.Errorf("fetch USGS: %w", err)`
- Use context propagation for cancellation
- Prefer stdlib over external dependencies

### Adding a Notification Channel

1. Create a new file in `internal/notifications/` (e.g., `sms.go`)
2. Implement `Enabled()`, `SendInitialAlert()`, and `SendEnrichmentUpdate()`
3. Wire it into `internal/services/notification.go`
4. Add config vars to `internal/config/config.go` and `.env.example`

### Working with PHIVOLCS Scraping

The PHIVOLCS HTML is fragile — they can change it without notice. When working on the scraper:

- Test against the sample HTML fixtures (see `testdata/` if available)
- Write defensive code — handle missing fields, malformed HTML
- Never crash on parse errors; log and skip
- Keep scraping logic isolated in `internal/sources/phivolcs.go`

### Commit Messages

We follow conventional commits:

- `feat:` — new feature
- `fix:` — bug fix
- `docs:` — documentation only
- `refactor:` — code change that doesn't add features or fix bugs
- `chore:` — maintenance (deps, CI, configs)

Reference the roadmap phase when relevant: `feat(phase-5): ...`

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run a specific package
go test ./internal/sources/...
```

For testing against real USGS data (public API, no auth needed):
```bash
curl "https://earthquake.usgs.gov/fdsnws/event/1/query?format=geojson&minlatitude=4.5&maxlatitude=21.5&minlongitude=116&maxlongitude=128&minmagnitude=2.5&orderby=time&limit=5"
```

## Reporting Issues

When opening an issue, include:

- What you expected vs what happened
- Steps to reproduce
- Go version (`go version`)
- OS and architecture
- Relevant log output

## Code of Conduct

Be respectful. This is a community project for Filipino developers and earthquake preparedness. Everyone is welcome regardless of experience level.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
