package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fusoya59/3s/internal/browser"
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

	// Step 1: Resolve config path and ensure config dir exists
	cfgPathResolved := cfgPath
	if cfgPathResolved == "" {
		cfgPathResolved = config.DefaultPath()
	}

	cfgDir := filepath.Dir(cfgPathResolved)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create config directory %s: %v\n", cfgDir, err)
		return 1
	}
	output.Stderrf("config directory: %s\n", cfgDir)

	// Step 2: Resolve or download Chromium
	binPath, needsConfigUpdate, err := browser.ResolveWithConfig(cfgPathResolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: install chromium manually (sudo pacman -S chromium) and re-run '3s init'\n")
		return 1
	}
	output.Stderrf("chromium: %s\n", binPath)

	// Step 3: Update config if needed (download happened or path changed)
	if needsConfigUpdate {
		cfg, loadErr := config.Load(cfgPathResolved)
		if loadErr == nil {
			if err := cfg.ExpandPath(); err == nil {
				cfg.BrowserBinPath = binPath
				data, marshalErr := json.MarshalIndent(cfg, "", "  ")
				if marshalErr == nil {
					_ = os.WriteFile(cfgPathResolved, data, 0644)
					output.Stderrf("config updated: browser_bin_path = %s\n", binPath)
				}
			}
		}
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

	cmd := exec.Command(binPath, "--version")
	if out, err := cmd.Output(); err == nil {
		output.Stderrf("✓ chromium version: %s", string(out))
	} else {
		output.Stderrf("✗ chromium version check failed: %v\n", err)
	}

	output.Stderrf("✓ config directory: %s\n", cfgDir)
	output.Stderrf("✓ cache directory: %s\n", cacheDir)

	output.Stderr("\nSetup complete! Run '3s status' for full status.")

	return 0
}
