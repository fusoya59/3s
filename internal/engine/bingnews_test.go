package engine

import (
	"path/filepath"
	"testing"
)

func TestParseBingNewsResults_Basic(t *testing.T) {
	be := &BingNewsEngine{}
	testParseGolden(t,
		filepath.Join("testdata", "bingnews", "news_basic.html"),
		filepath.Join("testdata", "bingnews", "news_basic.golden"),
		be.parseResults)
}

func TestParseBingNewsResults_NoResults(t *testing.T) {
	be := &BingNewsEngine{}
	testParseGolden(t,
		filepath.Join("testdata", "bingnews", "news_no_results.html"),
		filepath.Join("testdata", "bingnews", "news_no_results.golden"),
		be.parseResults)
}

func TestParseBingNewsResults_RelativeThumbnail(t *testing.T) {
	be := &BingNewsEngine{}
	testParseGolden(t,
		filepath.Join("testdata", "bingnews", "news_relative_thumbnail.html"),
		filepath.Join("testdata", "bingnews", "news_relative_thumbnail.golden"),
		be.parseResults)
}

func TestTimeRangeToBingNews(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"day", "4"},
		{"week", "7"},
		{"month", "9"},
		{"year", "9"},
		{"", ""},
		{"bogus", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := timeRangeToBingNews(tt.input)
			if got != tt.want {
				t.Errorf("timeRangeToBingNews(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
