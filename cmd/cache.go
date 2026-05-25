package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/fusoya59/3s/internal/config"
)

func cmdCache(args []string, cfgPath string) int {
	_ = cfgPath // passed through to subcommands

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: 3s cache <subcommand> [options]\n\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  purge    Delete the cache database\n")
		fmt.Fprintf(os.Stderr, "\nUse \"3s cache <subcommand> -h\" for subcommand-specific help.\n")
		return 1
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "purge":
		return cmdCachePurge(subArgs, cfgPath)
	case "-h", "--help":
		fmt.Fprintf(os.Stderr, "Usage: 3s cache <subcommand> [options]\n\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  purge    Delete the cache database\n")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: unknown cache subcommand %q\n", subCmd)
		return 1
	}
}

func cmdCachePurge(args []string, cfgPath string) int {
	fs := flag.NewFlagSet("cache purge", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: 3s cache purge [options]\n\n")
		fmt.Fprintf(os.Stderr, "Deletes the cache database file.\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -c <path>    Config file path (default: ~/.config/3s/config.json)\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	cfgPathResolved := cfgPath
	if cfgPathResolved == "" {
		cfgPathResolved = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPathResolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if err := cfg.ExpandPath(); err != nil {
		fmt.Fprintf(os.Stderr, "error expanding config path: %v\n", err)
		return 1
	}

	if cfg.CachePath == "" {
		fmt.Fprintf(os.Stderr, "error: no cache path configured\n")
		return 1
	}

	if _, err := os.Stat(cfg.CachePath); os.IsNotExist(err) {
		fmt.Println("cache does not exist, nothing to purge")
		return 0
	}

	if err := os.Remove(cfg.CachePath); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot delete cache: %v\n", err)
		return 1
	}

	fmt.Printf("cache deleted: %s\n", cfg.CachePath)
	return 0
}
