package search

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/fusoya59/3s/internal/cache"
	"github.com/fusoya59/3s/internal/engine"
)

// EngineWarning records a non-fatal error from a single engine.
type EngineWarning struct {
	Engine string
	Page   int
	Err    error
}

// SearchResult holds the aggregated search results.
type SearchResult struct {
	Results  []engine.Result
	Warnings []EngineWarning
	Total    int
}

// Searcher orchestrates parallel searches across engines.
type Searcher struct {
	registry *engine.Registry
	cache    *cache.Cache
	timeout  time.Duration
}

// New creates a new Searcher.
func New(reg *engine.Registry, c *cache.Cache, timeout time.Duration) *Searcher {
	return &Searcher{
		registry: reg,
		cache:    c,
		timeout:  timeout,
	}
}

// SearchParams configures a search.
type SearchParams struct {
	Engines []engine.Engine
	Time    string
	Safe    engine.SafeSearch
	Locale  string
	Count   int
	NoCache bool
}

// Search executes a search across engines in parallel.
func (s *Searcher) Search(ctx context.Context, query string, opts SearchParams) (*SearchResult, error) {
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	engines := opts.Engines
	if len(engines) == 0 {
		engines = s.registry.All()
	}

	count := opts.Count
	if count <= 0 {
		count = 20
	}
	pageSize := 10

	pagesPerEngine := int(math.Ceil(float64(count) / float64(pageSize)))
	if pagesPerEngine < 1 {
		pagesPerEngine = 1
	}
	if pagesPerEngine > 5 {
		pagesPerEngine = 5
	}

	type pageTask struct {
		engine engine.Engine
		page   int
	}

	// Build tasks grouped by page so we can serialize page order.
	// Page 1 must complete before page 2+ for engines that need VQD tokens
	// or other first-page state (e.g. DuckDuckGo).
	var pageTasks [][]pageTask
	for p := 1; p <= pagesPerEngine; p++ {
		var batch []pageTask
		for _, eng := range engines {
			batch = append(batch, pageTask{engine: eng, page: p})
		}
		pageTasks = append(pageTasks, batch)
	}

	resultCh := make(chan pageResult, len(engines)*pagesPerEngine)
	var warnings []EngineWarning
	var mu sync.Mutex

	// Process pages sequentially — all engines for page N run in parallel,
	// but page N+1 does not start until page N is fully complete.
	for _, batch := range pageTasks {
		g, gctx := errgroup.WithContext(ctx)

		for _, task := range batch {
			task := task
			g.Go(func() error {
				select {
				case <-gctx.Done():
					return gctx.Err()
				default:
				}

				eng := task.engine
				page := task.page
				searchOpts := engine.SearchOptions{
					SafeSearch: opts.Safe,
					TimeRange:  opts.Time,
					Locale:     opts.Locale,
					Page:       page,
				}

				cacheKey := cache.Key("result", eng.Name(), query,
					opts.Time, fmt.Sprintf("%d", opts.Safe),
					opts.Locale, fmt.Sprintf("%d", page))

				if !opts.NoCache && s.cache != nil {
					if data, ok := s.cache.Get(cacheKey); ok {
						if results, err := deserializeResults(data); err == nil {
							for i := range results {
								results[i].Positions = []int{i + 1}
								results[i].Engines = []string{eng.Name()}
								results[i].Cached = true
							}
							resultCh <- pageResult{engine: eng.Name(), page: page, results: results}
							return nil
						}
					}
				}

				results, err := eng.Search(gctx, query, searchOpts)
				if err != nil {
					mu.Lock()
					warnings = append(warnings, EngineWarning{
						Engine: eng.Name(),
						Page:   page,
						Err:    err,
					})
					mu.Unlock()
					return nil
				}

				for i := range results {
					results[i].Positions = []int{i + 1}
					results[i].Engines = []string{eng.Name()}
				}

				if !opts.NoCache && s.cache != nil && len(results) > 0 {
					if data, err := serializeResults(results); err == nil {
						s.cache.Set(cacheKey, data)
					}
				}

				resultCh <- pageResult{engine: eng.Name(), page: page, results: results}
				return nil
			})
		}

		if err := g.Wait(); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			close(resultCh)
			return nil, fmt.Errorf("search: unexpected error: %w", err)
		}
	}
	close(resultCh)

	var allResults []engine.Result
	for pr := range resultCh {
		allResults = append(allResults, pr.results...)
	}

	allResults = mergeResults(allResults)

	sort.SliceStable(allResults, func(i, j int) bool {
		if allResults[i].Score != allResults[j].Score {
			return allResults[i].Score > allResults[j].Score
		}
		return len(allResults[i].Engines) > len(allResults[j].Engines)
	})

	total := len(allResults)

	if len(allResults) > count {
		allResults = allResults[:count]
	}

	return &SearchResult{
		Results:  allResults,
		Warnings: warnings,
		Total:    total,
	}, nil
}

type pageResult struct {
	engine  string
	page    int
	results []engine.Result
}

func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	u.Host = host

	path := strings.TrimSuffix(u.Path, "/")
	u.Path = path

	u.Fragment = ""

	return u.String()
}

// mergeResults merges duplicate results and computes scores.
// Engine weight is hard-coded to 1.0 (no EngineWeights config).
func mergeResults(results []engine.Result) []engine.Result {
	keyIndex := make(map[string]int)
	var out []engine.Result

	for _, r := range results {
		key := normalizeURL(r.URL)
		if idx, ok := keyIndex[key]; ok {
			existing := &out[idx]
			for _, e := range r.Engines {
				found := false
				for _, existingEng := range existing.Engines {
					if e == existingEng {
						found = true
						break
					}
				}
				if !found {
					existing.Engines = append(existing.Engines, e)
				}
			}
			existing.Positions = append(existing.Positions, r.Positions...)
			if len(r.Snippet) > len(existing.Snippet) {
				existing.Snippet = r.Snippet
			}
			if len(r.Title) > len(existing.Title) {
				existing.Title = r.Title
			}
		} else {
			keyIndex[key] = len(out)
			out = append(out, r)
		}
	}

	for i := range out {
		computeScore(&out[i])
	}

	return out
}

// computeScore applies the searxng scoring formula with hard-coded weight 1.0.
func computeScore(r *engine.Result) {
	if len(r.Positions) == 0 {
		r.Score = 0
		return
	}

	weight := 1.0
	for range r.Engines {
		weight *= 1.0
	}
	weight *= float64(len(r.Positions))

	score := 0.0
	for _, pos := range r.Positions {
		score += weight / float64(pos)
	}
	r.Score = score
}

func serializeResults(results []engine.Result) ([]byte, error) {
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(escapeNull(r.URL))
		b.WriteByte('\x00')
		b.WriteString(escapeNull(r.Title))
		b.WriteByte('\x00')
		b.WriteString(escapeNull(r.Snippet))
		b.WriteByte('\x00')
		b.WriteString(escapeNull(r.Thumbnail))
		b.WriteByte('\x00')
		b.WriteString(r.Published.Format(time.RFC3339))
		b.WriteByte('\x00')
		b.WriteString("")
	}
	return []byte(b.String()), nil
}

func deserializeResults(data []byte) ([]engine.Result, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	lines := strings.Split(string(data), "\n")
	results := make([]engine.Result, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) < 4 {
			continue
		}
		r := engine.Result{
			URL:     unescapeNull(parts[0]),
			Title:   unescapeNull(parts[1]),
			Snippet: unescapeNull(parts[2]),
		}

		if len(parts) >= 6 {
			r.Thumbnail = unescapeNull(parts[3])
			if parts[4] != "" {
				if t, err := time.Parse(time.RFC3339, parts[4]); err == nil {
					r.Published = t
				}
			}
		}
		results = append(results, r)
	}
	return results, nil
}

func escapeNull(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\x00", "\\0")
	return s
}

func unescapeNull(s string) string {
	s = strings.ReplaceAll(s, "\\0", "\x00")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
