package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.1.0-beta"

// Run is the main entry point for the CLI.
func Run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 1
	}

	cmd := args[1]

	// Global flags before subcommand dispatch
	// h and version remain global
	switch cmd {
	case "-h", "--help":
		printUsage()
		return 0
	case "-v", "--version":
		fmt.Printf("3s version %s\n", version)
		return 0
	}

	// Extract -c and -f from remaining args (now per-command flags)
	cfgPath, format, remaining := extractGlobalFlags(args[1:])

	if len(remaining) == 0 {
		printUsage()
		return 1
	}

	cmd = remaining[0]
	subArgs := remaining[1:]

	switch cmd {
	case "search":
		return cmdSearch(subArgs, cfgPath, format)
	case "scrape":
		return cmdScrape(subArgs, cfgPath)
	case "sanitize":
		return cmdSanitize(subArgs, cfgPath)
	case "run":
		return cmdRun(subArgs, cfgPath, format)
	case "init":
		return cmdInit(subArgs, cfgPath)
	case "status":
		return cmdStatus(subArgs, cfgPath)
	case "cache":
		return cmdCache(subArgs, cfgPath)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", cmd)
		printUsage()
		return 1
	}
}

// extractGlobalFlags pulls -c and -f from args before subcommand dispatch.
// Returns (configPath, format, remaining args).
func extractGlobalFlags(args []string) (string, string, []string) {
	cfgPath := ""
	format := "json"
	var remaining []string

	skipNext := false
	for i, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "-c" && i+1 < len(args) {
			cfgPath = args[i+1]
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "-c=") {
			cfgPath = a[3:]
			continue
		}
		if a == "-f" && i+1 < len(args) {
			format = args[i+1]
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "-f=") {
			format = a[3:]
			continue
		}
		remaining = append(remaining, a)
	}
	if format != "json" && format != "table" {
		format = "json"
	}
	return cfgPath, format, remaining
}

// configDir returns ~/.config/3s
func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "3s")
}

// cacheDir returns ~/.cache/3s
func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "3s")
}

func printUsage() {
	fmt.Print(`3s - Search, Scrape, Sanitize

Usage:
  3s [global flags] <command> [command flags] [arguments]

Global Flags:
  -h           Show help
  -v           Show version

Commands:
  search <query>     Search engines for query
  scrape [url]       Scrape URL(s) - single URL arg or pipe NDJSON
  sanitize [html]    Sanitize HTML - single HTML arg or pipe NDJSON
  run <query>        Search -> Scrape -> Sanitize in one go
  init               Download chromium, create config, health check
  status             Engine chrome cache status
  cache <subcmd>     Cache operations (purge)

Use "3s <command> -h" for command-specific help.
`)
}

// isFlagValue checks if args[pos] is a value for a preceding flag.
func isFlagValue(args []string, pos int, valueFlags []string) bool {
	if pos == 0 {
		return false
	}
	prev := args[pos-1]
	if !strings.HasPrefix(prev, "-") {
		return false
	}
	// Check if prev is one of the value-taking flags
	for _, f := range valueFlags {
		if strings.HasPrefix(prev, f+"=") || prev == f {
			return true
		}
	}
	return false
}

// extractQuery separates positional args (query) from flag args.
func extractQuery(args []string, valueFlags []string) (query string, flags []string) {
	var q []string
	for i, a := range args {
		if !strings.HasPrefix(a, "-") && !isFlagValue(args, i, valueFlags) {
			q = append(q, a)
		} else if i > 0 || strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		}
	}
	return strings.Join(q, " "), flags
}
