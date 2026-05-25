package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/fusoya59/3s/internal/record"
)

// IsTerminal returns true if stdout is a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// IsStdinTerminal returns true if stdin is a terminal.
func IsStdinTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// WriteJSON writes records as JSON array to writer.
func WriteJSON(w io.Writer, records []record.Record) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(records)
}

// WriteNDJSON writes records as NDJSON (one JSON object per line).
func WriteNDJSON(w io.Writer, records []record.Record) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "")
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

// WriteTable writes records as a terminal table.
func WriteTable(w io.Writer, records []record.Record, color bool) error {
	if len(records) == 0 {
		_, _ = fmt.Fprintln(w, "No results.")
		return nil
	}

	width := terminalWidth()

	for i, r := range records {
		num := i + 1
		titleWidth := width - 6
		if titleWidth < 40 {
			titleWidth = 40
		}
		if titleWidth > 200 {
			titleWidth = 200
		}

		title := truncateText(r.Title, titleWidth)
		if color {
			_, _ = fmt.Fprintf(w, "\033[1;34m[%d]\033[0m \033[1m%s\033[0m\n", num, title)
		} else {
			_, _ = fmt.Fprintf(w, "[%d] %s\n", num, title)
		}

		_, _ = fmt.Fprintf(w, "    %s\n", r.URL)

		if r.Snippet != nil && *r.Snippet != "" {
			snippet := truncateText(*r.Snippet, width-4)
			_, _ = fmt.Fprintf(w, "    %s\n", snippet)
		}

		src := "-"
		if len(r.Engines) > 0 {
			src = r.Engines[0]
		}
		meta := fmt.Sprintf("── %s", src)
		if r.PublishedAt != nil && *r.PublishedAt != "" {
			if t, err := time.Parse(time.RFC3339, *r.PublishedAt); err == nil {
				meta += fmt.Sprintf(" · %s", t.Format("2006-01-02"))
			}
		}
		if r.Cached {
			meta += " · cached"
		}
		if r.Score > 0 {
			meta += fmt.Sprintf(" · score: %.2f", r.Score)
		}
		if len(r.Engines) > 1 {
			meta += fmt.Sprintf(" · %d engines", len(r.Engines))
		}
		if r.Error != nil && *r.Error != "" {
			meta += fmt.Sprintf(" · error: %s", *r.Error)
		}
		meta += " ──"
		_, _ = fmt.Fprintf(w, "    %s\n", meta)

		_, _ = fmt.Fprintln(w)
	}

	return nil
}

// WriteOutput dispatches to the appropriate output format.
// Format is "json" or "table".
// Terminal mode: json → JSON array, table → terminal table.
// Pipe mode: json → NDJSON, table → error.
func WriteOutput(w io.Writer, records []record.Record, format string) error {
	isTTY := IsTerminal()

	if format == "table" && !isTTY {
		return fmt.Errorf("table format cannot be piped, use -f json")
	}

	if format == "table" {
		return WriteTable(w, records, true)
	}

	// JSON format
	if isTTY {
		return WriteJSON(w, records)
	}
	return WriteNDJSON(w, records)
}

func terminalWidth() int {
	width := 80
	fd := int(os.Stdout.Fd())
	if term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			width = w
		}
	}
	if width < 80 {
		width = 80
	}
	if width > 200 {
		width = 200
	}
	return width
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// Stderr writes a message to stderr.
func Stderr(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// Stderrf writes a formatted message to stderr.
func Stderrf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}
