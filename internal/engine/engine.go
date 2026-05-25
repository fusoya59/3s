package engine

import (
	"context"
	"fmt"
	"time"
)

// Result represents a single search result.
type Result struct {
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Snippet   string    `json:"snippet"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	Published time.Time `json:"published_at"`
	Cached    bool      `json:"cached"`

	Score     float64  `json:"score,omitempty"`
	Engines   []string `json:"engines,omitempty"`
	Positions []int    `json:"-"`
}

// SafeSearch levels.
type SafeSearch int

const (
	SafeSearchOff      SafeSearch = 0
	SafeSearchModerate SafeSearch = 1
	SafeSearchStrict   SafeSearch = 2
)

// SearchOptions configures a search request.
type SearchOptions struct {
	SafeSearch SafeSearch
	TimeRange  string
	Locale     string
	Page       int
}

// Engine defines the interface for a search engine backend.
type Engine interface {
	Name() string
	Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error)
}

// ErrCaptcha is returned when the engine detects a CAPTCHA challenge.
type ErrCaptcha struct {
	Engine string
}

func (e *ErrCaptcha) Error() string {
	return fmt.Sprintf("%s: captcha challenge detected", e.Engine)
}

// ErrQueryTooLong is returned when the query exceeds engine limits.
type ErrQueryTooLong struct {
	Engine string
	MaxLen int
}

func (e *ErrQueryTooLong) Error() string {
	return fmt.Sprintf("%s: query too long (max %d characters)", e.Engine, e.MaxLen)
}

// ErrVQDMissing is returned when DuckDuckGo pagination needs a VQD token.
type ErrVQDMissing struct {
	Engine string
}

func (e *ErrVQDMissing) Error() string {
	return fmt.Sprintf("%s: VQD token missing for pagination", e.Engine)
}
