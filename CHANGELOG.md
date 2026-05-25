# Changelog

All notable changes to 3s.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0-beta] — 2026-05-24

### Added

- **search** — meta-search across DuckDuckGo, Brave, Bing News
- **scrape** — headless Chrome extraction via bonk CDP, shadow DOM support
- **sanitize** — HTML→markdown conversion via go-trafilatura
- **run** — all-in-one pipeline (search → scrape → sanitize)
- **pipe mode** — composable NDJSON streaming between stages
- **cache** — SQLite TTL cache for search results
- **status** — health checks for browser, engines, cache, config
- **init** — setup wizard (config, cache dirs, Chromium check)
- Output formats: JSON, NDJSON, terminal table
- Docker support (Alpine multi-stage)
- CI/CD: lint, test, 5-platform build, release workflow

[v0.1.0-beta]: https://github.com/fusoya59/3s/releases/tag/v0.1.0-beta
