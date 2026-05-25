package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fusoya59/3s/internal/cache"
	"github.com/fusoya59/3s/internal/config"
	"github.com/fusoya59/3s/internal/engine"
	"github.com/fusoya59/3s/internal/output"
	"github.com/fusoya59/3s/internal/record"
	"github.com/fusoya59/3s/internal/sanitizer"
	"github.com/fusoya59/3s/internal/scraper"
	"github.com/fusoya59/3s/internal/search"
)

func cmdRun(args []string, cfgPath string, format string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	limit := fs.Int("l", 10, "max results (default 10)")
	enginesStr := fs.String("e", "", "comma-separated engines (brave,duckduckgo,brave-news,bingnews)")
	maxChars := fs.Int("m", 25000, "max characters for scraped/sanitized content")
	concurrency := fs.Int("b", 3, "batch size — concurrent scrapes")
	refresh := fs.Bool("r", false, "refresh cache")
	outputFile := fs.String("o", "", "output file (default: stdout)")
	browserBin := fs.String("browser-bin", "", "path to Chrome/Chromium binary")
	raw := fs.Bool("raw", false, "include raw HTML in output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: 3s run [options] <query>\n\n")
		fmt.Fprintf(os.Stderr, "Runs search -> scrape -> sanitize in one process.\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -c <path>    Config file path (default: %s)\n", config.DefaultPath())
		fmt.Fprintf(os.Stderr, "  -f <format>  Output format: json or table (default: json)\n")
		fs.PrintDefaults()
	}

	query, flagArgs := extractQuery(args, []string{"-l", "--limit", "-e", "--engines", "-m", "--max-chars", "-b", "-o", "--output-file", "--browser-bin", "--raw"})

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if query == "" {
		fmt.Fprintf(os.Stderr, "error: search query required\n\n")
		fs.Usage()
		return 1
	}

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

	// Phase 1: Search
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.SearchTimeout) * time.Second,
	}

	reg := engine.NewRegistry(httpClient, cfg.UserAgent, c)

	var engineNames []string
	if *enginesStr != "" {
		for _, n := range strings.Split(*enginesStr, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				engineNames = append(engineNames, n)
			}
		}
	} else {
		engineNames = []string{"brave", "duckduckgo"}
	}

	selectedEngines, err := reg.SelectEngines(engineNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	searcher := search.New(reg, c, time.Duration(cfg.SearchTimeout)*time.Second)

	sp := search.SearchParams{
		Engines: selectedEngines,
		Locale:  cfg.Locale,
		Safe:    engine.SafeSearch(cfg.Safesearch),
		Count:   *limit,
		NoCache: *refresh,
	}

	ctx := context.Background()
	result, err := searcher.Search(ctx, query, sp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: search failed: %v\n", err)
		return 1
	}

	records := make([]record.Record, 0, len(result.Results))
	for _, r := range result.Results {
		published := ""
		if !r.Published.IsZero() {
			published = r.Published.Format(time.RFC3339)
		}
		records = append(records, record.Record{
			URL:         r.URL,
			Title:       r.Title,
			Snippet:     record.StrPtr(r.Snippet),
			Score:       r.Score,
			Engines:     record.NonNilEngines(r.Engines),
			Cached:      r.Cached,
			PublishedAt: record.StrPtr(published),
		})
	}

	for _, w := range result.Warnings {
		output.Stderrf("warning: %s: page %d: %v\n", w.Engine, w.Page, w.Err)
	}

	if len(records) == 0 {
		output.Stderr("no search results to scrape")
		return 0
	}

	// Phase 2: Scrape
	binPath := cfg.BrowserBinPath
	if *browserBin != "" {
		binPath = *browserBin
	}

	pool, err := scraper.NewPool(binPath, time.Duration(cfg.ScrapeTimeout)*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot start browser: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run '3s init' to set up chromium\n")
		return 1
	}
	defer func() { _ = pool.Close() }()

	output.Stderrf("scraping %d URLs (concurrency: %d)...\n", len(records), *concurrency)
	records = scraper.ScrapeRecords(records, pool, time.Duration(cfg.ScrapeTimeout)*time.Second, *concurrency, *refresh, c, cfg.ContentMinChars, cfg.ContentPollTimeout)

	// Phase 3: Sanitize
	output.Stderr("sanitizing...")
	records = sanitizer.SanitizeRecords(records, *maxChars)

	// Strip raw HTML unless --raw flag set
	if !*raw {
		for i := range records {
			records[i].RawHTML = nil
		}
	}

	// Output
	w := ioWriter(*outputFile)
	if w == nil {
		return 1
	}
	defer func() { _ = w.Close() }()

	if err := output.WriteOutput(w, records, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func ioWriter(path string) *os.File {
	if path == "" {
		return os.Stdout
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create output file: %v\n", err)
		return nil
	}
	return f
}
