# Roadmap

## v0.2.0 — Google integration

- [ ] Google engine: scrape Google Search results (HTML or API)
- [ ] Google News engine: scrape Google News results
- [ ] `engine_config` support for API keys:
  ```json
  {
    "engine_config": {
      "google": { "api_key": "..." },
      "google-news": { "api_key": "..." }
    }
  }
  ```

## v0.3.0 — Engine config & extensibility

- [ ] Per-engine configuration via `engine_config` map
- [ ] Custom engine plugins via config
- [ ] Engine-specific time range support
- [ ] Configurable result merging weights

## v0.4.0 — Output enhancements

- [ ] HTML output format
- [ ] CSV output format
- [ ] Custom template output
- [ ] Syntax highlighting in terminal table output

## v0.5.0 — Quality of life

- [ ] Configurable default limit/engines per command
- [ ] Shell completion (bash, zsh, fish)
- [ ] Man page
- [ ] Progress indicators for long-running operations
- [ ] Better error recovery for browser crashes

## v1.0.0 — Stable

- [ ] Full test coverage
- [ ] CI/CD pipeline
- [ ] Release binaries for Linux, macOS, Windows
- [ ] Homebrew tap

## Backlog

- [ ] AI/LLM summarization stage (post-sanitize)
- [ ] Concurrent pipe stages (auto-detect pipe topology)
- [ ] Proxy support for engines and scraper
- [ ] Cookie/session persistence
- [ ] Search result dedup across runs
- [ ] Export to SQLite/JSONL
