package sanitizer

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/markusmobius/go-trafilatura"

	"github.com/fusoya59/3s/internal/pipe"
	"github.com/fusoya59/3s/internal/record"
)

// Extractor wraps trafilatura.Extract for testability.
type Extractor interface {
	Extract(r io.Reader, opts trafilatura.Options) (*trafilatura.ExtractResult, error)
}

type defaultExtractor struct{}

func (defaultExtractor) Extract(r io.Reader, opts trafilatura.Options) (*trafilatura.ExtractResult, error) {
	return trafilatura.Extract(r, opts)
}

// DefaultExtractor is the Extractor used by Sanitize. Tests may replace it.
var DefaultExtractor Extractor = defaultExtractor{}

// SanitizeResult holds the result of HTML sanitization.
type SanitizeResult struct {
	ContentMD   string
	Title       string
	PublishedAt string // RFC3339 format, empty if not found
	Error       string
}

// Sanitize converts raw HTML to markdown using go-trafilatura.
func Sanitize(rawHTML string, maxChars int) SanitizeResult {
	if rawHTML == "" {
		return SanitizeResult{
			Error: "no raw HTML to sanitize",
		}
	}

	result, err := DefaultExtractor.Extract(strings.NewReader(rawHTML), trafilatura.Options{
		ExcludeTables: false,
		IncludeLinks:  true,
		IncludeImages: false,
		EnableLog:     false,
	})
	if err != nil {
		return SanitizeResult{
			Error: fmt.Sprintf("sanitize: %v", err),
		}
	}

	content := result.ContentText
	title := result.Metadata.Title

	var publishedAt string
	if !result.Metadata.Date.IsZero() {
		publishedAt = result.Metadata.Date.Format(time.RFC3339)
	}

	// Truncate to maxChars if exceeded
	if maxChars > 0 && len(content) > maxChars {
		content = content[:maxChars]
	}
	if maxChars > 0 && len(title) > maxChars {
		title = title[:maxChars]
	}

	return SanitizeResult{
		ContentMD:   content,
		Title:       title,
		PublishedAt: publishedAt,
	}
}

// SanitizePipe reads NDJSON from stdin, sanitizes each raw_html, writes enriched NDJSON to stdout.
// If keepRaw is false, raw_html is stripped from output records.
func SanitizePipe(r io.Reader, w io.Writer, maxChars int, keepRaw bool) error {
	records, err := pipe.ReadNDJSON(r)
	if err != nil {
		return fmt.Errorf("sanitize pipe: read input: %w", err)
	}

	enc := json.NewEncoder(w)
	for i := range records {
		rec := &records[i]

		if rec.Error != nil && *rec.Error != "" {
			// Skip if there's already an error from scrape
			if err := enc.Encode(rec); err != nil {
				return fmt.Errorf("sanitize pipe: write: %w", err)
			}
			continue
		}

		if rec.RawHTML == nil || *rec.RawHTML == "" {
			if err := enc.Encode(rec); err != nil {
				return fmt.Errorf("sanitize pipe: write: %w", err)
			}
			continue
		}

		sr := Sanitize(*rec.RawHTML, maxChars)
		rec.ContentMD = record.StrPtr(sr.ContentMD)
		if sr.Error != "" {
			rec.Error = record.StrPtr(sr.Error)
		}
		if rec.PublishedAt == nil && sr.PublishedAt != "" {
			rec.PublishedAt = record.StrPtr(sr.PublishedAt)
		}
		if !keepRaw {
			rec.RawHTML = nil
		}

		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("sanitize pipe: write: %w", err)
		}
	}

	return nil
}

// SanitizeRecords sanitizes all records in-process (for `run` command).
func SanitizeRecords(records []record.Record, maxChars int) []record.Record {
	out := make([]record.Record, len(records))
	for i, rec := range records {
		out[i] = rec

		if rec.Error != nil && *rec.Error != "" {
			continue
		}
		if rec.RawHTML == nil || *rec.RawHTML == "" {
			continue
		}

		sr := Sanitize(*rec.RawHTML, maxChars)
		out[i].ContentMD = record.StrPtr(sr.ContentMD)
		if sr.Error != "" {
			out[i].Error = record.StrPtr(sr.Error)
		}
		if out[i].PublishedAt == nil && sr.PublishedAt != "" {
			out[i].PublishedAt = record.StrPtr(sr.PublishedAt)
		}
	}
	return out
}
