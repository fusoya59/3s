package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fusoya59/3s/internal/cache"
	"github.com/fusoya59/3s/internal/config"
	"github.com/fusoya59/3s/internal/output"
	"github.com/fusoya59/3s/internal/scraper"
)

func cmdScrape(args []string, cfgPath string) int {
	fs := flag.NewFlagSet("scrape", flag.ContinueOnError)
	maxChars := fs.Int("m", 25000, "max characters for scraped content")
	_ = maxChars // unused by scrape; flag kept for interface compatibility
	batchSize := fs.Int("b", 3, "batch size — concurrent scrapes")
	_ = batchSize // accepted for future use
	refresh := fs.Bool("r", false, "refresh cache")
	browserBin := fs.String("browser-bin", "", "path to Chrome/Chromium binary")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: 3s scrape [options] [url]\n\n")
		fmt.Fprintf(os.Stderr, "If url is provided, scrapes that single URL.\n")
		fmt.Fprintf(os.Stderr, "If stdin is a pipe, reads NDJSON records and scrapes each URL.\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -c <path>    Config file path (default: %s)\n", config.DefaultPath())
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	url := fs.Arg(0)
	isTTY := output.IsTerminal()

	// Load config
	cfgPathResolved := cfgPath
	if cfgPathResolved == "" {
		cfgPathResolved = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPathResolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid config: %v\n", err)
		return 1
	}
	if err := cfg.ExpandPath(); err != nil {
		fmt.Fprintf(os.Stderr, "error expanding config paths: %v\n", err)
		return 1
	}

	minChars := cfg.ContentMinChars
	pollTimeout := cfg.ContentPollTimeout

	// Resolve browser binary
	binPath := cfg.BrowserBinPath
	if *browserBin != "" {
		binPath = *browserBin
	}

	// Open cache
	var c *cache.Cache
	if !*refresh {
		cacheDir := ""
		if idx := strings.LastIndex(cfg.CachePath, "/"); idx >= 0 {
			cacheDir = cfg.CachePath[:idx]
		}
		if cacheDir != "" {
			if err := os.MkdirAll(cacheDir, 0755); err != nil {
				output.Stderrf("warning: cannot create cache directory: %v\n", err)
			}
		}
		c, err = cache.Open(cfg.CachePath, time.Duration(cfg.CacheTTL)*time.Second)
		if err != nil {
			output.Stderrf("warning: cache unavailable: %v\n", err)
			c = nil
		}
	}
	if c != nil {
		defer func() { _ = c.Close() }()
	}

	// Create browser pool
	pool, err := scraper.NewPool(binPath, time.Duration(cfg.ScrapeTimeout)*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot start browser: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run '3s init' to set up chromium\n")
		return 1
	}
	defer func() { _ = pool.Close() }()

	timeout := time.Duration(cfg.ScrapeTimeout) * time.Second

	if url != "" {
		// Single URL mode
		rec := scraper.FetchSingle(pool, url, timeout, minChars, pollTimeout, *refresh, c)

		// Output single record as JSON
		// Pretty-print for TTY; NDJSON for pipe (so sanitize can consume it)
		enc := json.NewEncoder(os.Stdout)
		if isTTY {
			enc.SetIndent("", "  ")
		}
		if err := enc.Encode(rec); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	} else {
		// Pipe mode: read NDJSON from stdin
		if output.IsStdinTerminal() {
			fmt.Fprintf(os.Stderr, "error: no URL provided and stdin is a terminal\n\n")
			fs.Usage()
			return 1
		}

		if err := scraper.ScrapePipe(os.Stdin, os.Stdout, pool, timeout, *refresh, c, minChars, pollTimeout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}

	return 0
}
