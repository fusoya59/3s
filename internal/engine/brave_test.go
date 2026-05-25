package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func TestBraveParseGeneral_Basic(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "brave", "general_basic.html"),
		filepath.Join("testdata", "brave", "general_basic.golden"),
		braveParseGeneral)
}

func TestBraveParseGeneral_NoResults(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "brave", "general_no_results.html"),
		filepath.Join("testdata", "brave", "general_no_results.golden"),
		braveParseGeneral)
}

func TestBraveParseGeneral_InvalidURL(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "brave", "general_invalid_url.html"),
		filepath.Join("testdata", "brave", "general_invalid_url.golden"),
		braveParseGeneral)
}

func TestBraveParseNews_Basic(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "brave", "news_basic.html"),
		filepath.Join("testdata", "brave", "news_basic.golden"),
		braveParseNews)
}

func TestBraveParseNews_NoResults(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "brave", "news_no_results.html"),
		filepath.Join("testdata", "brave", "news_no_results.golden"),
		braveParseNews)
}

func TestParseBraveDate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "absolute date",
			input: "Mar 15, 2026",
			want:  time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "3 minutes ago",
			input: "3 minutes ago",
			want:  now.Add(-3 * time.Minute),
		},
		{
			name:  "1 hour ago",
			input: "1 hour ago",
			want:  now.Add(-1 * time.Hour),
		},
		{
			name:  "5 days ago",
			input: "5 days ago",
			want:  now.Add(-120 * time.Hour),
		},
		{
			name:  "2 weeks ago",
			input: "2 weeks ago",
			want:  now.Add(-336 * time.Hour),
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "garbage",
			input:   "garbage text",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBraveDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseBraveDate(%q) expected error, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBraveDate(%q) unexpected error: %v", tt.input, err)
			}

			if !tt.want.IsZero() {
				diff := got.Sub(tt.want)
				if diff < 0 {
					diff = -diff
				}
				if diff > 5*time.Second {
					t.Errorf("parseBraveDate(%q) = %v, want within 5s of %v", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestBraveLocale(t *testing.T) {
	tests := []struct {
		input       string
		wantCountry string
		wantLang    string
	}{
		{"", "us", "en-us"},
		{"en-US", "us", "en-us"},
		{"de-DE", "de", "de-de"},
		{"zz-ZZ", "us", "en-us"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			country, lang := braveLocale(tt.input)
			if country != tt.wantCountry {
				t.Errorf("braveLocale(%q) country = %q, want %q", tt.input, country, tt.wantCountry)
			}
			if lang != tt.wantLang {
				t.Errorf("braveLocale(%q) lang = %q, want %q", tt.input, lang, tt.wantLang)
			}
		})
	}
}

// TestBraveParseGeneral_WithDate tests inline (not golden) because relative dates depend on time.Now().
func TestBraveParseGeneral_WithDate(t *testing.T) {
	html := `<html><body><div class="snippet">
<a href="https://example.com/page">Link</a>
<div class="title">Recent</div>
<span class="t-secondary">3 minutes ago</span>
<div class="content">3 minutes ago - Recent content.</div>
</div></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	results := braveParseGeneral(doc)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	now := time.Now()
	diff := now.Sub(results[0].Published)
	if diff < 2*time.Minute || diff > 4*time.Minute {
		t.Errorf("Published = %v, expected ~3 minutes ago (diff=%v)", results[0].Published, diff)
	}

	if results[0].Snippet != "Recent content." {
		t.Errorf("Snippet = %q, want 'Recent content.'", results[0].Snippet)
	}
}

func TestBraveParseGeneral_WithRelativeDateWeek(t *testing.T) {
	html := `<html><body><div class="snippet">
<a href="https://example.com/page">Link</a>
<div class="title">Old</div>
<span class="t-secondary">2 weeks ago</span>
<div class="content">2 weeks ago - Old content.</div>
</div></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	results := braveParseGeneral(doc)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	now := time.Now()
	diff := now.Sub(results[0].Published)
	if diff < 330*time.Hour || diff > 342*time.Hour {
		t.Errorf("Published = %v, expected ~2 weeks ago (diff=%v)", results[0].Published, diff)
	}

	if results[0].Snippet != "Old content." {
		t.Errorf("Snippet = %q, want 'Old content.'", results[0].Snippet)
	}
}

// Verify golden file format matches actual MarshalIndent output
func TestBraveGoldenConsistency(t *testing.T) {
	htmlData, err := os.ReadFile(filepath.Join("testdata", "brave", "general_basic.html"))
	if err != nil {
		t.Fatalf("read HTML: %v", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(htmlData)))
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	results := braveParseGeneral(doc)
	gotJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	goldenData, err := os.ReadFile(filepath.Join("testdata", "brave", "general_basic.golden"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if string(gotJSON) != string(goldenData) {
		t.Errorf("golden file must be regenerated. Run tests with -update flag")
	}
}
