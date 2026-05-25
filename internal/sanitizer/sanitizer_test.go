package sanitizer

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/markusmobius/go-trafilatura"

	"github.com/fusoya59/3s/internal/record"
)

// mockExtractor implements Extractor for testing.
type mockExtractor struct {
	result *trafilatura.ExtractResult
	err    error
}

func (m *mockExtractor) Extract(r io.Reader, opts trafilatura.Options) (*trafilatura.ExtractResult, error) {
	return m.result, m.err
}

func TestSanitizeEmptyHTML(t *testing.T) {
	saved := DefaultExtractor
	DefaultExtractor = &mockExtractor{}
	defer func() { DefaultExtractor = saved }()

	sr := Sanitize("", 0)
	if sr.Error != "no raw HTML to sanitize" {
		t.Errorf("expected error 'no raw HTML to sanitize', got %q", sr.Error)
	}
}

func TestSanitizeMaxCharsTruncation(t *testing.T) {
	saved := DefaultExtractor
	DefaultExtractor = &mockExtractor{
		result: &trafilatura.ExtractResult{
			ContentText: strings.Repeat("a", 2000),
			Metadata: trafilatura.Metadata{
				Title: strings.Repeat("b", 50),
			},
		},
	}
	defer func() { DefaultExtractor = saved }()

	t.Run("truncated to 100", func(t *testing.T) {
		sr := Sanitize("<html></html>", 100)
		if len(sr.ContentMD) != 100 {
			t.Errorf("expected content length 100, got %d", len(sr.ContentMD))
		}
		if len(sr.Title) != 50 {
			t.Errorf("expected title length 50, got %d", len(sr.Title))
		}
	})

	t.Run("maxChars=0 no truncation", func(t *testing.T) {
		sr := Sanitize("<html></html>", 0)
		if len(sr.ContentMD) != 2000 {
			t.Errorf("expected content length 2000, got %d", len(sr.ContentMD))
		}
		if len(sr.Title) != 50 {
			t.Errorf("expected title length 50, got %d", len(sr.Title))
		}
	})
}

func TestSanitizeExtractorError(t *testing.T) {
	saved := DefaultExtractor
	DefaultExtractor = &mockExtractor{err: fmt.Errorf("boom")}
	defer func() { DefaultExtractor = saved }()

	sr := Sanitize("<html></html>", 0)
	if sr.Error != "sanitize: boom" {
		t.Errorf("expected 'sanitize: boom', got %q", sr.Error)
	}
	if sr.ContentMD != "" {
		t.Errorf("expected empty ContentMD, got %q", sr.ContentMD)
	}
}

func TestSanitizeSuccess(t *testing.T) {
	saved := DefaultExtractor
	DefaultExtractor = &mockExtractor{
		result: &trafilatura.ExtractResult{
			ContentText: "markdown here",
			Metadata: trafilatura.Metadata{
				Title: "My Title",
			},
		},
	}
	defer func() { DefaultExtractor = saved }()

	sr := Sanitize("<html></html>", 0)
	if sr.ContentMD != "markdown here" {
		t.Errorf("ContentMD = %q, want 'markdown here'", sr.ContentMD)
	}
	if sr.Title != "My Title" {
		t.Errorf("Title = %q, want 'My Title'", sr.Title)
	}
	if sr.Error != "" {
		t.Errorf("Error = %q, want empty", sr.Error)
	}
}

func TestSanitizeRecordsSkipsErrors(t *testing.T) {
	errStr := "prev error"
	recs := []record.Record{
		{URL: "https://ok.com", RawHTML: record.StrPtr("<html></html>"), Title: "OK"},
		{URL: "https://error.com", RawHTML: record.StrPtr("<html></html>"), Title: "Err", Error: &errStr},
		{URL: "https://ok2.com", RawHTML: record.StrPtr("<html></html>"), Title: "OK2"},
	}

	saved := DefaultExtractor
	DefaultExtractor = &mockExtractor{
		result: &trafilatura.ExtractResult{
			ContentText: "processed",
			Metadata:    trafilatura.Metadata{Title: "Sanitized"},
		},
	}
	defer func() { DefaultExtractor = saved }()

	out := SanitizeRecords(recs, 0)

	if out[0].ContentMD == nil || *out[0].ContentMD != "processed" {
		t.Errorf("expected ContentMD 'processed', got %v", out[0].ContentMD)
	}
	if out[1].Error == nil || *out[1].Error != "prev error" {
		t.Errorf("expected error preserved, got %v", out[1].Error)
	}
	if out[1].ContentMD != nil && *out[1].ContentMD != "" {
		t.Errorf("expected empty ContentMD for error record, got %q", *out[1].ContentMD)
	}
	if out[2].ContentMD == nil || *out[2].ContentMD != "processed" {
		t.Errorf("expected ContentMD 'processed', got %v", out[2].ContentMD)
	}
}

func TestSanitizeRecordsSkipsEmptyHTML(t *testing.T) {
	recs := []record.Record{
		{URL: "https://empty.com", RawHTML: nil},
	}
	out := SanitizeRecords(recs, 0)
	if out[0].ContentMD != nil && *out[0].ContentMD != "" {
		t.Errorf("expected empty ContentMD, got %q", *out[0].ContentMD)
	}
}

func TestSanitizeRecordsMaxChars(t *testing.T) {
	saved := DefaultExtractor
	DefaultExtractor = &mockExtractor{
		result: &trafilatura.ExtractResult{
			ContentText: strings.Repeat("x", 500),
			Metadata:    trafilatura.Metadata{Title: "Title"},
		},
	}
	defer func() { DefaultExtractor = saved }()

	recs := []record.Record{
		{URL: "https://example.com", RawHTML: record.StrPtr("<html></html>")},
	}
	out := SanitizeRecords(recs, 10)
	if out[0].ContentMD == nil || len(*out[0].ContentMD) != 10 {
		t.Errorf("expected ContentMD length 10, got %v", out[0].ContentMD)
	}
}

func TestSanitizeRealTrafilatura_Smoke(t *testing.T) {
	saved := DefaultExtractor
	DefaultExtractor = defaultExtractor{}
	defer func() { DefaultExtractor = saved }()

	t.Run("simple HTML", func(t *testing.T) {
		sr := Sanitize("<h1>Hello</h1><p>World</p>", 0)
		if sr.Error != "" {
			t.Logf("expected no error, got: %v (may be OK on some systems)", sr.Error)
		}
	})

	t.Run("article HTML", func(t *testing.T) {
		html := `<html><body><article><h1>Test</h1><p>Para content.</p></article></body></html>`
		sr := Sanitize(html, 0)
		if sr.Error != "" {
			t.Logf("expected no error, got: %v (may be OK on some systems)", sr.Error)
		}
		if sr.ContentMD != "" && !strings.Contains(sr.ContentMD, "Para") {
			t.Errorf("ContentMD does not contain 'Para': %q", sr.ContentMD)
		}
	})
}

func TestSanitizePipe(t *testing.T) {
	saved := DefaultExtractor
	DefaultExtractor = &mockExtractor{
		result: &trafilatura.ExtractResult{
			ContentText: "pipe content",
			Metadata:    trafilatura.Metadata{Title: "Pipe Title"},
		},
	}
	defer func() { DefaultExtractor = saved }()

	input := `{"url":"https://example.com","raw_html":"<html></html>","title":"Orig"}
{"url":"https://example.org","raw_html":"<html><p>test</p></html>","title":"Orig2"}
`
	var buf strings.Builder
	err := SanitizePipe(strings.NewReader(input), &buf, 0, false)
	if err != nil {
		t.Fatalf("SanitizePipe: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "pipe content") {
		t.Errorf("line 0 should contain 'pipe content': %s", lines[0])
	}
	if strings.Contains(lines[0], "raw_html") {
		t.Errorf("line 0 should not contain raw_html (keepRaw=false): %s", lines[0])
	}
}
