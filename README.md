# 3s — Search, Scrape, Sanitize

[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**3s** is a Go CLI that meta-searches multiple search engines, scrapes result URLs via headless Chrome with shadow DOM extraction, and sanitizes HTML to readable markdown via go-trafilatura.

```
Search ──→ Scrape ──→ Sanitize
  │           │           │
  ▼           ▼           ▼
 URLs      raw HTML    markdown
```

## Install

### From source

```bash
# Requires: Go 1.26+
go install github.com/fusoya59/3s@latest
```

### From Docker

```bash
docker build -t 3s .
docker run --rm -it 3s search "hello world"
```

### Dependencies

- **SQLite** — pure-Go via `modernc.org/sqlite`. No CGO or system libraries required.
- **Chromium** — required for scraping. Install via package manager or run `3s init`.
  - Debian/Ubuntu: `sudo apt install chromium-browser`
  - Arch: `sudo pacman -S chromium`
  - Alpine: `apk add chromium`

## Testing

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./cmd/...
go test ./internal/engine/...

# Verbose output
go test -v ./...
```

## Quick Start

```bash
# Search
3s search "golang web scraping"

# Search → Scrape → Sanitize (all-in-one)
3s run "golang web scraping" -l 5

# Pipe pipeline (each stage streams NDJSON)
3s search "golang" | 3s scrape | 3s sanitize > results.json

# Set up config and check status
3s init
3s status
```

## Usage

### Global Flags

```
-c <path>    Config file path (default: ~/.config/3s/config.json)
-f <format>  Output format: json or table (default: json)
-h           Show help
-v           Show version
```

### search

```
3s search <query> [options]

-l <n>          Max results (default: 10)
-e <engines>    Comma-separated engines: brave,duckduckgo,brave-news,bingnews
-r              Refresh cache (fetch fresh results)
--locale        Search locale (e.g. en-US, de-DE)
--safesearch    Safe search level: 0=off, 1=moderate, 2=strict
--search-timeout Search timeout in seconds
```

### scrape

```
3s scrape [options] [url]

-m <n>          Max characters for scraped content (default: 25000)
-r              Refresh cache
--browser-bin   Path to Chrome/Chromium binary
```

Pipe mode: `3s search "query" | 3s scrape`
Single mode: `3s scrape https://example.com`

### sanitize

```
3s sanitize [options] [rawhtml]

-m <n>          Max characters for sanitized content (default: 25000)
```

Pipe mode: `3s search "query" | 3s scrape | 3s sanitize`
Single mode: `3s sanitize '<html>...</html>'`

### run

```
3s run <query> [options]

-l <n>          Max results (default: 10)
-e <engines>    Comma-separated engines
-m <n>          Max characters for scraped/sanitized content
-j <n>          Concurrent scrapes (default: 3, NOT -c which is config path)
-r              Refresh cache
-o <file>       Output file (default: stdout)
--browser-bin   Path to Chrome/Chromium binary
```

### init

```
3s init
```

Creates config/cache directories, checks for Chromium, runs health checks.

### status

```
3s status [--verbose]
```

Checks browser, engines (duckduckgo, brave, bingnews), cache, and config.
Shows per-failure recovery tips. Exits 1 if any check fails, 0 otherwise.

- `--verbose` — show detailed error output (raw HTTP codes, file paths)

Examples:

```bash
# Quick health check
3s status

# Detailed output for debugging
3s status --verbose

# Script-friendly: only proceed if all checks pass
3s status && 3s run "my query"
```

### cache

```
3s cache purge
```

Deletes the cache database.

## Output Formats

- `-f json` (default): NDJSON when piped, JSON array on terminal
- `-f table`: Terminal table (errors on pipe)

## Configuration

Config file: `~/.config/3s/config.json`

| Field | Default | Description |
|-------|---------|-------------|
| locale | en-US | Search locale |
| safesearch | 0 | Safe search level (0-2) |
| user_agent | Mozilla/5.0 ... Firefox/135.0 | HTTP user agent |
| search_timeout | 15 | Search timeout in seconds |
| scrape_timeout | 30 | Scrape timeout in seconds |
| content_min_chars | 500 | Minimum content characters before poll exits |
| content_poll_timeout | 5 | Content poll timeout in seconds |
| cache_path | ~/.cache/3s/cache.db | SQLite cache path |
| cache_ttl | 300 | Cache TTL in seconds (5 min) |
| browser_bin_path | "" | Path to Chrome/Chromium binary |
| engine_config | {} | Per-engine settings (stub) |

## Architecture

```
main.go
  └─ cmd/
       ├─ root.go        CLI dispatch
       ├─ search.go      Search subcommand
       ├─ scrape.go      Scrape subcommand
       ├─ sanitize.go    Sanitize subcommand
       ├─ run.go         All-in-one pipeline
       ├─ init.go        Setup
       ├─ status.go      Health check
       └─ cache.go       Cache operations
  └─ internal/
       ├─ config/        Config load/validate
       ├─ cache/         SQLite key-value cache
       ├─ engine/        Search engine interface + implementations
       ├─ search/        Parallel search + merge/dedup
       ├─ scraper/       bonk browser pool + fetch + orchestrator
       ├─ sanitizer/     go-trafilatura wrapper
       ├─ pipe/          NDJSON encode/decode
       └─ output/        Format dispatch (JSON, table, NDJSON)
```

## Dependencies

| Library | Purpose |
|---------|---------|
| [bonk](https://github.com/joakimcarlsson/bonk) | Chrome CDP browser automation |
| [go-trafilatura](https://github.com/markusmobius/go-trafilatura) | HTML → markdown extraction |
| [goquery](https://github.com/PuerkitoBio/goquery) | HTML parsing for search engines |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | SQLite driver (pure Go) |
| [golang.org/x/sync](https://golang.org/x/sync) | errgroup for parallel search |
| [golang.org/x/term](https://golang.org/x/term) | Terminal width detection |

## License

MIT
