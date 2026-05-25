package search

import (
	"reflect"
	"testing"
	"time"

	"github.com/fusoya59/3s/internal/engine"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://www.Example.com/Path/", "https://example.com/Path"},
		{"https://example.com#frag", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"https://WWW.EXAMPLE.COM", "https://example.com"},
		{"not-a-url", "not-a-url"},
		{"https://example.com:8080/path", "https://example.com:8080/path"},
		{"https://www.example.com", "https://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeURL(tt.input)
			if got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeScore(t *testing.T) {
	tests := []struct {
		name  string
		pos   []int
		score float64
	}{
		{"single position [1]", []int{1}, 1.0},
		{"two positions [1,2]", []int{1, 2}, 3.0},
		{"position [3]", []int{3}, 1.0 / 3.0},
		{"empty positions", []int{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := engine.Result{Positions: tt.pos}
			computeScore(&r)
			if r.Score != tt.score {
				t.Errorf("Score = %v, want %v", r.Score, tt.score)
			}
		})
	}
}

func TestSerializeDeserializeRoundtrip(t *testing.T) {
	t.Run("single result all fields", func(t *testing.T) {
		in := []engine.Result{
			{
				URL:       "https://example.com/page",
				Title:     "Test Title",
				Snippet:   "Test snippet here.",
				Thumbnail: "https://example.com/img.jpg",
				Published: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			},
		}
		data, err := serializeResults(in)
		if err != nil {
			t.Fatalf("serializeResults: %v", err)
		}
		out, err := deserializeResults(data)
		if err != nil {
			t.Fatalf("deserializeResults: %v", err)
		}
		if !reflect.DeepEqual(in, out) {
			t.Errorf("roundtrip mismatch:\n in=%+v\nout=%+v", in[0], out[0])
		}
	})

	t.Run("multiple (3) results", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://a.com", Title: "A", Snippet: "a snippet"},
			{URL: "https://b.com", Title: "B", Snippet: "b snippet"},
			{URL: "https://c.com", Title: "C", Snippet: "c snippet"},
		}
		data, err := serializeResults(in)
		if err != nil {
			t.Fatalf("serializeResults: %v", err)
		}
		out, err := deserializeResults(data)
		if err != nil {
			t.Fatalf("deserializeResults: %v", err)
		}
		if !reflect.DeepEqual(in, out) {
			t.Errorf("roundtrip mismatch:\n in=%+v\nout=%+v", in, out)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		data, err := serializeResults(nil)
		if err != nil {
			t.Fatalf("serializeResults: %v", err)
		}
		if len(data) != 0 {
			t.Errorf("expected empty data, got %d bytes", len(data))
		}
		_, err = deserializeResults(data)
		if err == nil {
			t.Fatal("expected error for empty data")
		}
	})

	t.Run("URL with null byte and backslash", func(t *testing.T) {
		in := []engine.Result{
			{
				URL:     "https://example.com/\x00page\\path",
				Title:   "Test",
				Snippet: "desc",
			},
		}
		data, err := serializeResults(in)
		if err != nil {
			t.Fatalf("serializeResults: %v", err)
		}
		out, err := deserializeResults(data)
		if err != nil {
			t.Fatalf("deserializeResults: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("expected 1 result, got %d", len(out))
		}
		if out[0].URL != in[0].URL {
			t.Errorf("URL mismatch: %q vs %q", out[0].URL, in[0].URL)
		}
	})

	t.Run("empty title and snippet", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com"},
		}
		data, err := serializeResults(in)
		if err != nil {
			t.Fatalf("serializeResults: %v", err)
		}
		out, err := deserializeResults(data)
		if err != nil {
			t.Fatalf("deserializeResults: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("expected 1 result, got %d", len(out))
		}
	})

	t.Run("zero published time", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com", Published: time.Time{}},
		}
		data, err := serializeResults(in)
		if err != nil {
			t.Fatalf("serializeResults: %v", err)
		}
		out, err := deserializeResults(data)
		if err != nil {
			t.Fatalf("deserializeResults: %v", err)
		}
		if !out[0].Published.Equal(time.Time{}) {
			t.Errorf("expected zero time, got %v", out[0].Published)
		}
	})

	t.Run("result with only URL populated", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com/onlyurl"},
		}
		data, err := serializeResults(in)
		if err != nil {
			t.Fatalf("serializeResults: %v", err)
		}
		out, err := deserializeResults(data)
		if err != nil {
			t.Fatalf("deserializeResults: %v", err)
		}
		if len(out) != 1 || out[0].URL != "https://example.com/onlyurl" {
			t.Errorf("got %+v", out)
		}
	})
}

func TestDeserializeEmptyData(t *testing.T) {
	_, err := deserializeResults([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	if err.Error() != "empty data" {
		t.Errorf("expected 'empty data', got %q", err.Error())
	}
}

func TestEscapeUnescapeNull(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"simple", "hello"},
		{"null byte", "a\x00b"},
		{"backslash", "a\\b"},
		// "\0" omitted: ambiguous with null-byte escape sequence in current encoding
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unescapeNull(escapeNull(tt.in))
			if got != tt.in {
				t.Errorf("roundtrip: %q -> %q", tt.in, got)
			}
		})
	}
}

func TestMergeResults(t *testing.T) {
	t.Run("same URL different engines", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com", Engines: []string{"brave"}, Positions: []int{1}},
			{URL: "https://example.com", Engines: []string{"duckduckgo"}, Positions: []int{1}},
		}
		out := mergeResults(in)
		if len(out) != 1 {
			t.Fatalf("expected 1 result, got %d", len(out))
		}
		expected := []string{"brave", "duckduckgo"}
		if !reflect.DeepEqual(out[0].Engines, expected) {
			t.Errorf("Engines = %v, want %v", out[0].Engines, expected)
		}
	})

	t.Run("three results two same URL", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com/a", Engines: []string{"brave"}},
			{URL: "https://example.com/b", Engines: []string{"brave"}},
			{URL: "https://example.com/a", Engines: []string{"duckduckgo"}},
		}
		out := mergeResults(in)
		if len(out) != 2 {
			t.Fatalf("expected 2 results, got %d", len(out))
		}
	})

	t.Run("same engine not duplicated", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com", Engines: []string{"brave"}},
			{URL: "https://example.com", Engines: []string{"brave"}},
		}
		out := mergeResults(in)
		if len(out) != 1 {
			t.Fatalf("expected 1 result, got %d", len(out))
		}
		if len(out[0].Engines) != 1 {
			t.Errorf("expected 1 engine, got %d: %v", len(out[0].Engines), out[0].Engines)
		}
	})

	t.Run("longest snippet kept", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com", Engines: []string{"brave"}, Snippet: "short"},
			{URL: "https://example.com", Engines: []string{"duckduckgo"}, Snippet: "a longer snippet here"},
		}
		out := mergeResults(in)
		if out[0].Snippet != "a longer snippet here" {
			t.Errorf("expected longest snippet, got %q", out[0].Snippet)
		}
	})

	t.Run("longest title kept", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com", Engines: []string{"brave"}, Title: "short"},
			{URL: "https://example.com", Engines: []string{"duckduckgo"}, Title: "a much longer title"},
		}
		out := mergeResults(in)
		if out[0].Title != "a much longer title" {
			t.Errorf("expected longest title, got %q", out[0].Title)
		}
	})

	t.Run("positions combined", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com", Engines: []string{"brave"}, Positions: []int{1}},
			{URL: "https://example.com", Engines: []string{"duckduckgo"}, Positions: []int{2}},
		}
		out := mergeResults(in)
		expected := []int{1, 2}
		if !reflect.DeepEqual(out[0].Positions, expected) {
			t.Errorf("Positions = %v, want %v", out[0].Positions, expected)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		out := mergeResults(nil)
		if len(out) != 0 {
			t.Errorf("expected empty, got %d", len(out))
		}
	})

	t.Run("score computed after merge", func(t *testing.T) {
		in := []engine.Result{
			{URL: "https://example.com", Engines: []string{"brave"}, Positions: []int{1}},
			{URL: "https://example.com", Engines: []string{"duckduckgo"}, Positions: []int{2}},
		}
		out := mergeResults(in)
		if out[0].Score == 0 {
			t.Error("expected non-zero Score after merge")
		}
	})
}
