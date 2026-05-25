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
	"github.com/fusoya59/3s/internal/search"
)

func cmdSearch(args []string, cfgPath string, format string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("l", 10, "max results")
	enginesStr := fs.String("e", "", "comma-separated engines (brave,duckduckgo,brave-news,bingnews)")
	refresh := fs.Bool("r", false, "refresh cache")
	locale := fs.String("locale", "", "search locale (e.g. en-US)")
	safesearch := fs.Int("safesearch", 0, "safe search level: 0=off (default), 1=moderate, 2=strict")
	searchTimeout := fs.Int("search-timeout", 0, "search timeout in seconds")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: 3s search [options] <query>\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -c <path>    Config file path (default: ~/.config/3s/config.json)\n")
		fmt.Fprintf(os.Stderr, "  -f <format>  Output format: json or table (default: json)\n")
		fs.PrintDefaults()
	}

	query, flagArgs := extractQuery(args, []string{"-l", "--limit", "-e", "--engines", "--locale", "--safesearch", "--search-timeout"})

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

	// Resolve timeout
	timeout := time.Duration(cfg.SearchTimeout) * time.Second
	if *searchTimeout > 0 {
		timeout = time.Duration(*searchTimeout) * time.Second
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: timeout,
	}

	// Always open cache (needed for VQD tokens, result dedup).
	// The --refresh flag only bypasses result cache via SearchParams.NoCache.
	var c *cache.Cache
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
	if c != nil {
		defer func() { _ = c.Close() }()
	}

	// Create engine registry
	reg := engine.NewRegistry(httpClient, cfg.UserAgent, c)

	// Select engines
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

	// Create searcher
	searcher := search.New(reg, c, timeout)

	// Resolve locale
	resLocale := *locale
	if resLocale == "" {
		resLocale = cfg.Locale
	}

	// Resolve safe search
	resSafe := engine.SafeSearch(cfg.Safesearch)
	if *safesearch >= 0 {
		resSafe = engine.SafeSearch(*safesearch)
	}

	sp := search.SearchParams{
		Engines: selectedEngines,
		Time:    "",
		Safe:    resSafe,
		Locale:  resLocale,
		Count:   *limit,
		NoCache: *refresh,
	}

	ctx := context.Background()
	result, err := searcher.Search(ctx, query, sp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: search failed: %v\n", err)
		return 1
	}

	// Convert engine.Result to record.Record
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

	// Dispatch output
	if err := output.WriteOutput(os.Stdout, records, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Write warnings to stderr
	for _, w := range result.Warnings {
		output.Stderrf("warning: %s: page %d: %v\n", w.Engine, w.Page, w.Err)
	}

	return 0
}
