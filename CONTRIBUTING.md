# Contributing to 3s

## Development Setup

```bash
# Clone and enter the repo
git clone https://github.com/fusoya59/3s.git
cd 3s

# Requires: Go 1.26+
go mod download
go build ./...

# Install pre-commit hooks (runs lint + test on every commit)
pre-commit install
```

## Running Tests

```bash
go test ./...         # all tests
go test -race ./...   # with race detector
go test -v ./...      # verbose
```

Tests use golden files (`internal/engine/testdata/`) for search engine HTML parsing. No network required.

## Code Style

Lint + static analysis run automatically via pre-commit. See `.pre-commit-config.yaml` and `.golangci.yml`.

## Pull Requests

1. Branch from `main`, prefix with `feat/`, `fix/`, or `docs/`.
2. Keep changes focused — one concern per PR.
3. Add tests for new functionality.
4. Run `pre-commit install` (once) to enable local hooks. Commits trigger lint + test automatically.
5. CI runs lint + test + build — watch for failures.

## Architecture

```
main.go
  └─ cmd/          CLI subcommands (cobra-free, hand-rolled)
  └─ internal/
       ├─ engine/  Search engine interface + implementations
       ├─ search/  Parallel search + merge/dedup
       ├─ scraper/ Chrome CDP browser pool + fetch
       ├─ sanitizer/ go-trafilatura wrapper
       ├─ cache/   SQLite key-value cache
       ├─ pipe/    NDJSON encode/decode
       └─ output/  Format dispatch (JSON, table)
```

New engines implement the `internal/engine.Engine` interface and register in `registry.go`.

## License

MIT. All contributions are under the same license.
