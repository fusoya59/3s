package record

// Record represents a single result flowing through the pipeline.
type Record struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Snippet     *string  `json:"snippet"`
	Score       float64  `json:"score"`
	Engines     []string `json:"engines"`
	Cached      bool     `json:"cached"`
	PublishedAt *string  `json:"published_at"`
	RawHTML     *string  `json:"raw_html,omitempty"`
	ScrapedAt   *string  `json:"scraped_at"`
	ContentMD   *string  `json:"content_md"`
	Error       *string  `json:"error"`
}

// StrPtr returns a pointer to s, or nil if s is empty.
func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// NonNilEngines returns e if non-nil, or an empty slice if nil.
// Ensures JSON serialization produces [] not null.
func NonNilEngines(e []string) []string {
	if e == nil {
		return []string{}
	}
	return e
}
