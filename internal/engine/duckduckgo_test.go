package engine

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

var update = flag.Bool("update", false, "update golden files")

func testParseGolden(t *testing.T, htmlPath, goldenPath string, parseFn func(*goquery.Document) []Result) {
	t.Helper()

	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read HTML: %v", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(htmlData)))
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	results := parseFn(doc)
	if results == nil {
		results = []Result{}
	}

	if *update {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			t.Fatalf("marshal results: %v", err)
		}
		if err := os.WriteFile(goldenPath, data, 0644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	wantData := strings.TrimSpace(string(goldenData))
	gotData := strings.TrimSpace(string(func() []byte {
		data, _ := json.MarshalIndent(results, "", "  ")
		return data
	}()))

	if wantData != gotData {
		t.Errorf("golden mismatch\n  got:  %s\n  want: %s\n\nRun with -update to regenerate", gotData, wantData)
	}
}

func TestParseDDGResults_Basic(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "duckduckgo", "search_basic.html"),
		filepath.Join("testdata", "duckduckgo", "search_basic.golden"),
		parseDDGResults)
}

func TestParseDDGResults_NoResults(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "duckduckgo", "search_no_results.html"),
		filepath.Join("testdata", "duckduckgo", "search_no_results.golden"),
		parseDDGResults)
}

func TestParseDDGResults_SkipsAds(t *testing.T) {
	testParseGolden(t,
		filepath.Join("testdata", "duckduckgo", "search_with_ad.html"),
		filepath.Join("testdata", "duckduckgo", "search_with_ad.golden"),
		parseDDGResults)
}

func TestQuoteDDGBangs(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"!g", "'!g'"},
		{"!g test", "'!g' test"},
		{"test !g query", "test '!g' query"},
		{"!a !b !c", "'!a' '!b' '!c'"},
		{"no bangs here", "no bangs here"},
		{"!", "!"},
		{"!!", "'!!'"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := quoteDDGBangs(tt.input)
			if got != tt.want {
				t.Errorf("quoteDDGBangs(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDDGLocale(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "wt-wt"},
		{"en-US", "us-en"},
		{"de-DE", "de-de"},
		{"ja-JP", "jp-jp"},
		{"zz-ZZ", "wt-wt"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ddgLocale(tt.input)
			if got != tt.want {
				t.Errorf("ddgLocale(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTimeRangeToDDG(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"day", "d"},
		{"week", "w"},
		{"month", "m"},
		{"year", "y"},
		{"", ""},
		{"bogus", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := timeRangeToDDG(tt.input)
			if got != tt.want {
				t.Errorf("timeRangeToDDG(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
