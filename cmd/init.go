package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fusoya59/3s/internal/config"
	"github.com/fusoya59/3s/internal/output"
)

func cmdInit(args []string, cfgPath string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: 3s init [options]\n\n")
		fmt.Fprintf(os.Stderr, "Sets up 3s: creates config/cache dirs, checks chromium, health check.\n\nOptions:\n")
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

	// Step 1: Check chromium
	chromiumPath := ""
	chromiumNames := []string{"google-chrome-stable", "google-chrome", "chromium-browser", "chromium", "chrome"}
	for _, name := range chromiumNames {
		if p, err := exec.LookPath(name); err == nil {
			chromiumPath = p
			break
		}
	}

	if chromiumPath == "" {
		// Check common paths
		commonPaths := []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/snap/bin/chromium",
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				chromiumPath = p
				break
			}
		}
	}

	if chromiumPath != "" {
		output.Stderrf("chromium found: %s\n", chromiumPath)
	} else {
		output.Stderr("chromium not found. Install one of: google-chrome-stable, chromium-browser, chromium")
		output.Stderr("  You can also set browser_bin_path in config or use --browser-bin flag")
		output.Stderr("  To download chromium automatically: npx @puppeteer/browsers install chromium")
	}

	// Step 2: Determine config path and directory
	resolveCfgPath := cfgPath
	if resolveCfgPath == "" {
		resolveCfgPath = filepath.Join(configDir(), "config.json")
	}
	cfgDir := filepath.Dir(resolveCfgPath)

	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create config directory %s: %v\n", cfgDir, err)
		return 1
	}
	output.Stderrf("config directory: %s\n", cfgDir)

	// Step 3: Write default config if missing
	if _, err := os.Stat(resolveCfgPath); os.IsNotExist(err) {
		defCfg := config.DefaultConfig()
		if chromiumPath != "" {
			defCfg.BrowserBinPath = chromiumPath
		}
		data, err := json.MarshalIndent(defCfg, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: marshal default config: %v\n", err)
			return 1
		}
		if err := os.WriteFile(resolveCfgPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write default config: %v\n", err)
			return 1
		}
		output.Stderrf("default config written: %s\n", resolveCfgPath)
	} else {
		output.Stderrf("config exists: %s\n", resolveCfgPath)
	}

	// Step 4: Create cache directory
	cacheDir := cacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create cache directory %s: %v\n", cacheDir, err)
		return 1
	}
	output.Stderrf("cache directory: %s\n", cacheDir)

	// Step 5: Health check
	output.Stderr("\n--- Health Check ---")

	if chromiumPath != "" {
		output.Stderrf("✓ chromium: %s\n", chromiumPath)

		// Test launch
		cmd := exec.Command(chromiumPath, "--version")
		if out, err := cmd.Output(); err == nil {
			output.Stderrf("✓ chromium version: %s", string(out))
		} else {
			output.Stderrf("✗ chromium version check failed: %v\n", err)
		}
	} else {
		output.Stderr("✗ chromium: not found")
	}

	output.Stderrf("✓ config directory: %s\n", cfgDir)
	output.Stderrf("✓ cache directory: %s\n", cacheDir)

	output.Stderr("\nSetup complete! Run '3s status' for full status.")

	return 0
}
