package cmd

import (
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/fusoya59/3s/internal/config"
)

func cmdStatus(args []string, cfgPath string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "Show detailed error output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: 3s status [options]\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -c <path>    Config file path (default: %s)\n", config.DefaultPath())
		fmt.Fprintf(os.Stderr, "  --verbose    Show detailed error output\n")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	failCount := 0
	warnCount := 0
	engineNetFails := 0

	// Load config
	cfgPathResolved := cfgPath
	if cfgPathResolved == "" {
		cfgPathResolved = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPathResolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot load config: %v\n", err)
	}
	if cfg != nil {
		if err := cfg.ExpandPath(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot expand config path: %v\n", err)
		}
	}

	// --- Browser Check ---
	fmt.Println("── Browser ──")
	chromiumPath := ""
	if cfg != nil && cfg.BrowserBinPath != "" {
		chromiumPath = cfg.BrowserBinPath
	}
	if chromiumPath == "" {
		chromiumNames := []string{"google-chrome-stable", "google-chrome", "chromium-browser", "chromium", "chrome"}
		for _, name := range chromiumNames {
			if p, err := exec.LookPath(name); err == nil {
				chromiumPath = p
				break
			}
		}
	}

	if chromiumPath != "" {
		cmd := exec.Command(chromiumPath, "--version")
		if out, err := cmd.Output(); err == nil {
			fmt.Printf("  ✓ %s", string(out))
		} else {
			fmt.Printf("  ✗ binary found but version check failed: %v\n", err)
			fmt.Println("    → Chromium may be corrupted. Reinstall: sudo pacman -S chromium")
			failCount++
		}
	} else {
		fmt.Println("  ✗ chromium not found")
		fmt.Println("    → Install: sudo pacman -S chromium")
		fmt.Println("      or: npx @puppeteer/browsers install chromium")
		fmt.Println("      or set browser_bin_path in ~/.config/3s/config.json")
		failCount++
	}

	// --- Engine Check ---
	fmt.Println("\n── Engines ──")
	client := &http.Client{Timeout: 10 * time.Second}

	ua := "Mozilla/5.0 (X11; Linux x86_64; rv:135.0) Gecko/20100101 Firefox/135.0"
	if cfg != nil && cfg.UserAgent != "" {
		ua = cfg.UserAgent
	}

	// duckduckgo — POST same as real search
	func() {
		form := url.Values{}
		form.Set("q", "test")
		form.Set("kl", "wt-wt")
		req, err := http.NewRequest("POST", "https://html.duckduckgo.com/html/",
			strings.NewReader(form.Encode()))
		if err != nil {
			fmt.Printf("  ✗ duckduckgo: request error: %v\n", err)
			fmt.Println("    → Check internet connection / firewall.")
			failCount++
			engineNetFails++
			return
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.7")
		req.Header.Set("Referer", "https://html.duckduckgo.com/")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  ✗ duckduckgo: unreachable: %v\n", err)
			fmt.Println("    → Check internet connection / firewall.")
			failCount++
			engineNetFails++
			return
		}
		defer func() { _ = resp.Body.Close() }()
		icon, tip := engineHTTPStatus("duckduckgo", resp, *verbose)
		fmt.Printf("  %s duckduckgo (HTTP %d)\n", icon, resp.StatusCode)
		if tip != "" {
			fmt.Println("    → " + tip)
		}
		switch icon {
		case "✗":
			failCount++
		case "⚠":
			warnCount++
		}
	}()

	// brave — GET search page
	func() {
		u := "https://search.brave.com/search?q=test&source=web"
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			fmt.Printf("  ✗ brave: request error: %v\n", err)
			fmt.Println("    → Check internet connection / firewall.")
			failCount++
			engineNetFails++
			return
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-us,en;q=0.5")
		req.Header.Set("Cookie", "safesearch=off; useLocation=0; summarizer=0; country=us; ui_lang=en-us")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  ✗ brave: unreachable: %v\n", err)
			fmt.Println("    → Check internet connection / firewall.")
			failCount++
			engineNetFails++
			return
		}
		defer func() { _ = resp.Body.Close() }()
		icon, tip := engineHTTPStatus("brave", resp, *verbose)
		fmt.Printf("  %s brave (HTTP %d)\n", icon, resp.StatusCode)
		if tip != "" {
			fmt.Println("    → " + tip)
		}
		switch icon {
		case "✗":
			failCount++
		case "⚠":
			warnCount++
		}
	}()

	// bingnews — GET news AJAX endpoint
	func() {
		u := "https://www.bing.com/news/infinitescrollajax?q=test&InfiniteScroll=1&first=1&SFX=0&form=PTFTNR"
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			fmt.Printf("  ✗ bingnews: request error: %v\n", err)
			fmt.Println("    → Check internet connection / firewall.")
			failCount++
			engineNetFails++
			return
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  ✗ bingnews: unreachable: %v\n", err)
			fmt.Println("    → Check internet connection / firewall.")
			failCount++
			engineNetFails++
			return
		}
		defer func() { _ = resp.Body.Close() }()
		icon, tip := engineHTTPStatus("bingnews", resp, *verbose)
		fmt.Printf("  %s bingnews (HTTP %d)\n", icon, resp.StatusCode)
		if tip != "" {
			fmt.Println("    → " + tip)
		}
		switch icon {
		case "✗":
			failCount++
		case "⚠":
			warnCount++
		}
	}()

	// Consolidated engine failure message
	if engineNetFails == 3 {
		fmt.Println("\n  ⓘ All engines unreachable — check internet connection.")
	}

	// --- Cache Check ---
	fmt.Println("\n── Cache ──")
	cachePath := ""
	if cfg != nil {
		cachePath = cfg.CachePath
	}
	if cachePath == "" {
		fmt.Println("  ⚠ no cache path configured")
		fmt.Println("    → Run '3s init' to create default config with cache path.")
		warnCount++
	} else if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		fmt.Println("  ⚠ cache db not found")
		fmt.Println("    → No searches run yet. Cache is created on first search.")
		warnCount++
	} else {
		db, err := sql.Open("sqlite", cachePath)
		if err != nil {
			fmt.Printf("  ✗ cannot open cache: %v\n", err)
			if *verbose {
				fmt.Printf("    → Path: %s\n", cachePath)
			}
			if strings.Contains(err.Error(), "locked") || strings.Contains(err.Error(), "database is locked") {
				fmt.Println("    → Another 3s process may be running. Retry later.")
			} else {
				fmt.Println("    → Cache DB may be corrupted. Run '3s cache purge' to reset.")
			}
			failCount++
		} else {
			defer func() { _ = db.Close() }()

			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM cache").Scan(&count); err != nil {
				fmt.Printf("  ✗ cannot query cache: %v\n", err)
				fmt.Println("    → Cache DB may be corrupted. Run '3s cache purge' to reset.")
				failCount++
			} else {
				fmt.Printf("  ✓ cache: %d entries\n", count)
			}

			var expiredCount int
			row := db.QueryRow("SELECT COUNT(*) FROM cache WHERE expires_at > 0 AND expires_at < ?", time.Now().Unix())
			if row.Scan(&expiredCount) == nil && expiredCount > 0 {
				fmt.Printf("  ⚠ %d expired entries\n", expiredCount)
				fmt.Println("    → Run '3s cache purge' to clean up expired entries.")
				warnCount++
			}
		}
	}

	// --- Config Check ---
	fmt.Println("\n── Config ──")
	if _, err := os.Stat(cfgPathResolved); os.IsNotExist(err) {
		fmt.Println("  ✗ no config file")
		fmt.Printf("    → Run '3s init' to create default config at %s\n", cfgPathResolved)
		failCount++
	} else {
		fmt.Printf("  ✓ %s\n", cfgPathResolved)

		// Validate config values
		if cfg != nil {
			configFailures := validateConfigValues(cfg)
			for _, f := range configFailures {
				fmt.Printf("  ✗ %s\n", f.field)
				fmt.Printf("    → %s\n", f.tip)
				failCount++
			}
		}
	}

	// --- Summary ---
	fmt.Println("\n── Summary ──")
	if failCount == 0 && warnCount == 0 {
		fmt.Println("All checks passed.")
	} else {
		parts := []string{}
		if failCount > 0 {
			parts = append(parts, fmt.Sprintf("%d check(s) failed", failCount))
		}
		if warnCount > 0 {
			parts = append(parts, fmt.Sprintf("%d warning(s)", warnCount))
		}
		summary := strings.Join(parts, ", ")
		fmt.Println(summary + " — see tips above.")

		if failCount > 0 {
			fmt.Println("Run '3s status --verbose' for detailed error output.")
			return 1
		}
	}

	return 0
}

// engineHTTPStatus returns an icon and recovery tip for an engine HTTP response.
func engineHTTPStatus(name string, resp *http.Response, verbose bool) (icon, tip string) {
	code := resp.StatusCode
	switch {
	case code == http.StatusOK:
		return "✓", ""
	case code == http.StatusTooManyRequests:
		tip = "Rate limited — retry in ~60s."
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if sec, err := strconv.Atoi(retryAfter); err == nil {
				tip = fmt.Sprintf("Rate limited — retry in ~%ds.", sec)
			}
		}
		return "⚠", tip
	case code == http.StatusForbidden:
		return "✗", "Access blocked. May need different user-agent or region. Try '3s init' to refresh config."
	case code >= 500:
		return "⚠", fmt.Sprintf("Server error (HTTP %d) — try again later.", code)
	default:
		if verbose {
			return "⚠", fmt.Sprintf("Unexpected HTTP %d.", code)
		}
		return "⚠", fmt.Sprintf("Unexpected response (HTTP %d). Run '3s status --verbose' for details.", code)
	}
}

type configFailure struct {
	field string
	tip   string
}

// validateConfigValues checks all config fields and returns a list of failures with tips.
// WARNING: Must stay in sync with config.Validate() in internal/config/config.go.
func validateConfigValues(cfg *config.Config) []configFailure {
	var failures []configFailure

	if cfg.SearchTimeout < 5 || cfg.SearchTimeout > 60 {
		failures = append(failures, configFailure{
			field: fmt.Sprintf("search_timeout: %d (must be 5–60)", cfg.SearchTimeout),
			tip:   "Edit config — set search_timeout between 5 and 60.",
		})
	}
	if cfg.ScrapeTimeout < 10 || cfg.ScrapeTimeout > 120 {
		failures = append(failures, configFailure{
			field: fmt.Sprintf("scrape_timeout: %d (must be 10–120)", cfg.ScrapeTimeout),
			tip:   "Edit config — set scrape_timeout between 10 and 120.",
		})
	}
	if cfg.ContentMinChars < 100 || cfg.ContentMinChars > 100000 {
		failures = append(failures, configFailure{
			field: fmt.Sprintf("content_min_chars: %d (must be 100–100000)", cfg.ContentMinChars),
			tip:   "Edit config — set content_min_chars between 100 and 100000.",
		})
	}
	if cfg.ContentPollTimeout < 1 || cfg.ContentPollTimeout > 30 {
		failures = append(failures, configFailure{
			field: fmt.Sprintf("content_poll_timeout: %d (must be 1–30)", cfg.ContentPollTimeout),
			tip:   "Edit config — set content_poll_timeout between 1 and 30.",
		})
	}
	if cfg.CacheTTL < 0 {
		failures = append(failures, configFailure{
			field: fmt.Sprintf("cache_ttl: %d (must be >= 0)", cfg.CacheTTL),
			tip:   "Edit config — set cache_ttl to 0 or higher.",
		})
	}
	if cfg.Safesearch < 0 || cfg.Safesearch > 2 {
		failures = append(failures, configFailure{
			field: fmt.Sprintf("safesearch: %d (must be 0, 1, or 2)", cfg.Safesearch),
			tip:   "Edit config — set safesearch to 0 (off), 1 (moderate), or 2 (strict).",
		})
	}

	return failures
}
