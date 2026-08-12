# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of Lindol API
- PHIVOLCS web scraping with smart stop-on-known logic
- USGS polling as fallback source
- REST API endpoints for earthquake data
- Real-time notifications (Telegram, Discord, Webhook)
- Per-IP rate limiting (1 req/min)
- SQLite database with automatic migrations
- Docker support with multi-stage builds
- Health monitoring and source status tracking
- Data normalization and transformation pipeline
- Exponential backoff retry logic
- Dev alerts for parser failures
- Two deployment modes (PHIVOLCS-primary vs USGS-only)

### Fixed
- Go version compatibility (upgraded to 1.23)
- Downgraded goquery to v1.10.1 for compatibility
- golangci-lint errors (unused types, unchecked errors)
- CI badge repository name mismatch
- Node.js deprecation warnings in CI

## [0.1.0] - 2026-08-11

### Initial Development
- Project scaffolding and structure
- Core functionality implemented
- CI/CD pipeline setup
- Documentation and examples

[Unreleased]: https://github.com/debiangee/phivolcs-lindol/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/debiangee/phivolcs-lindol/releases/tag/v0.1.0
