package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/fusoya59/3s/internal/output"
	"github.com/fusoya59/3s/internal/sanitizer"
)

func cmdSanitize(args []string, cfgPath string) int {
	_ = cfgPath // sanitize is stateless; accept for interface consistency

	fs := flag.NewFlagSet("sanitize", flag.ContinueOnError)
	maxChars := fs.Int("m", 25000, "max characters for sanitized content")
	raw := fs.Bool("raw", false, "include raw HTML in output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: 3s sanitize [options] [rawhtml]\n\n")
		fmt.Fprintf(os.Stderr, "If rawhtml is provided, sanitizes that single HTML string and outputs markdown.\n")
		fmt.Fprintf(os.Stderr, "If stdin is a pipe, reads NDJSON records and sanitizes each raw_html field.\n\nOptions:\n")
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

	htmlArg := fs.Arg(0)

	if htmlArg != "" {
		// Single HTML arg mode
		result := sanitizer.Sanitize(htmlArg, *maxChars)
		if result.Error != "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", result.Error)
			return 1
		}
		fmt.Println(result.ContentMD)
	} else {
		// Pipe mode: read NDJSON from stdin
		if output.IsStdinTerminal() {
			fmt.Fprintf(os.Stderr, "error: no HTML provided and stdin is a terminal\n\n")
			fs.Usage()
			return 1
		}

		if err := sanitizer.SanitizePipe(os.Stdin, os.Stdout, *maxChars, *raw); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}

	return 0
}
