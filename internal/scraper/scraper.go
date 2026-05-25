package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/fusoya59/3s/internal/cache"
	"github.com/fusoya59/3s/internal/pipe"
	"github.com/fusoya59/3s/internal/record"
)

// ScrapePipe reads NDJSON from stdin, scrapes each URL, writes enriched NDJSON to stdout.
func ScrapePipe(r io.Reader, w io.Writer, pool *Pool, timeout time.Duration, noCache bool, c *cache.Cache, minChars int, pollTimeout int) error {
	records, err := pipe.ReadNDJSON(r)
	if err != nil {
		return fmt.Errorf("scrape pipe: read input: %w", err)
	}

	enc := json.NewEncoder(w)
	for i := range records {
		rec := &records[i]

		if rec.URL == "" {
			// No URL to scrape
			if err := enc.Encode(rec); err != nil {
				return fmt.Errorf("scrape pipe: write: %w", err)
			}
			continue
		}

		// Check cache
		if !noCache && c != nil {
			cacheKey := cache.Key("scrape", rec.URL)
			if data, ok := c.Get(cacheKey); ok {
				var cached struct {
					RawHTML   string `json:"raw_html"`
					Title     string `json:"title"`
					ScrapedAt string `json:"scraped_at"`
				}
				if json.Unmarshal(data, &cached) == nil {
					rec.RawHTML = record.StrPtr(cached.RawHTML)
					rec.Title = cached.Title
					rec.ScrapedAt = record.StrPtr(cached.ScrapedAt)
					if err := enc.Encode(rec); err != nil {
						return fmt.Errorf("scrape pipe: write: %w", err)
					}
					continue
				}
			}
		}

		// Acquire page, scrape
		sp, err := pool.Acquire()
		if err != nil {
			rec.Error = record.StrPtr(fmt.Sprintf("scrape: acquire page: %v", err))
			if err := enc.Encode(rec); err != nil {
				return fmt.Errorf("scrape pipe: write: %w", err)
			}
			continue
		}

		result := FetchPage(sp, rec.URL, timeout, minChars, pollTimeout)

		pool.Release(sp)

		rec.RawHTML = record.StrPtr(result.RawHTML)
		rec.ScrapedAt = record.StrPtr(result.FetchedAt)
		if result.Title != "" {
			rec.Title = result.Title
		}
		if result.Error != "" {
			rec.Error = record.StrPtr(result.Error)
		}

		// Store in cache
		if !noCache && c != nil && result.Error == "" && result.RawHTML != "" {
			cacheKey := cache.Key("scrape", rec.URL)
			cacheData, _ := json.Marshal(map[string]string{
				"raw_html":   result.RawHTML,
				"title":      result.Title,
				"scraped_at": result.FetchedAt,
			})
			c.Set(cacheKey, cacheData)
		}

		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("scrape pipe: write: %w", err)
		}
	}

	return nil
}

// ScrapeRecords scrapes all records in-process (for `run` command).
// Cache-hit records are processed in parallel; cache-misses are serialized.
func ScrapeRecords(records []record.Record, pool *Pool, timeout time.Duration, concurrency int, noCache bool, c *cache.Cache, minChars int, pollTimeout int) []record.Record {
	out := make([]record.Record, len(records))
	sem := make(chan struct{}, concurrency)
	done := make(chan int, len(records))

	for i := range records {
		go func(idx int) {
			rec := records[idx]
			out[idx] = rec

			if rec.URL == "" {
				done <- idx
				return
			}

			// Cache check outside semaphore (parallel-safe)
			hitCache := false
			if !noCache && c != nil {
				cacheKey := cache.Key("scrape", rec.URL)
				if data, ok := c.Get(cacheKey); ok {
					var cached struct {
						RawHTML   string `json:"raw_html"`
						Title     string `json:"title"`
						ScrapedAt string `json:"scraped_at"`
					}
					if json.Unmarshal(data, &cached) == nil {
						out[idx] = record.Record{
							URL:         rec.URL,
							Title:       cached.Title,
							Snippet:     rec.Snippet,
							Score:       rec.Score,
							Engines:     record.NonNilEngines(rec.Engines),
							Cached:      rec.Cached,
							PublishedAt: rec.PublishedAt,
							RawHTML:     record.StrPtr(cached.RawHTML),
							ScrapedAt:   record.StrPtr(cached.ScrapedAt),
						}
						hitCache = true
					}
				}
			}

			if hitCache {
				done <- idx
				return
			}

			// Cache miss: acquire semaphore (serialized CDP)
			sem <- struct{}{}
			sp, err := pool.Acquire()
			if err != nil {
				out[idx].Error = record.StrPtr(fmt.Sprintf("scrape: acquire page: %v", err))
				<-sem
				done <- idx
				return
			}

			result := FetchPage(sp, rec.URL, timeout, minChars, pollTimeout)
			pool.Release(sp)
			<-sem

			out[idx].RawHTML = record.StrPtr(result.RawHTML)
			out[idx].ScrapedAt = record.StrPtr(result.FetchedAt)
			if result.Title != "" {
				out[idx].Title = result.Title
			}
			if result.Error != "" {
				out[idx].Error = record.StrPtr(result.Error)
			}

			// Store in cache
			if !noCache && c != nil && result.Error == "" && result.RawHTML != "" {
				cacheKey := cache.Key("scrape", rec.URL)
				cacheData, _ := json.Marshal(map[string]string{
					"raw_html":   result.RawHTML,
					"title":      result.Title,
					"scraped_at": result.FetchedAt,
				})
				c.Set(cacheKey, cacheData)
			}

			done <- idx
		}(i)
	}

	// Wait for all
	for i := 0; i < len(records); i++ {
		<-done
	}

	return out
}

// FetchSingle scrapes a single URL (for `scrape` command in single mode).
func FetchSingle(pool *Pool, url string, timeout time.Duration, minChars int, pollTimeout int, noCache bool, c *cache.Cache) record.Record {
	rec := record.Record{URL: url, Engines: []string{}}

	// Check cache
	if !noCache && c != nil {
		cacheKey := cache.Key("scrape", url)
		if data, ok := c.Get(cacheKey); ok {
			var cached struct {
				RawHTML   string `json:"raw_html"`
				Title     string `json:"title"`
				ScrapedAt string `json:"scraped_at"`
			}
			if json.Unmarshal(data, &cached) == nil {
				rec.RawHTML = record.StrPtr(cached.RawHTML)
				rec.Title = cached.Title
				rec.ScrapedAt = record.StrPtr(cached.ScrapedAt)
				return rec
			}
		}
	}

	sp, err := pool.Acquire()
	if err != nil {
		rec.Error = record.StrPtr(fmt.Sprintf("scrape: acquire page: %v", err))
		return rec
	}
	defer pool.Release(sp)

	result := FetchPage(sp, url, timeout, minChars, pollTimeout)

	rec.RawHTML = record.StrPtr(result.RawHTML)
	rec.ScrapedAt = record.StrPtr(result.FetchedAt)
	if result.Title != "" {
		rec.Title = result.Title
	}
	if result.Error != "" {
		rec.Error = record.StrPtr(result.Error)
	}

	// Store in cache
	if !noCache && c != nil && result.Error == "" && result.RawHTML != "" {
		cacheKey := cache.Key("scrape", url)
		cacheData, _ := json.Marshal(map[string]string{
			"raw_html":   result.RawHTML,
			"title":      result.Title,
			"scraped_at": result.FetchedAt,
		})
		c.Set(cacheKey, cacheData)
	}

	return rec
}
