package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// BingNewsEngine searches Bing News via AJAX endpoint.
type BingNewsEngine struct {
	client *http.Client
	ua     string
}

// NewBingNewsEngine creates a new Bing News engine.
func NewBingNewsEngine(client *http.Client, ua string) *BingNewsEngine {
	return &BingNewsEngine{client: client, ua: ua}
}

// Name returns "bingnews".
func (e *BingNewsEngine) Name() string { return "bingnews" }

// Search executes a search against Bing News.
func (e *BingNewsEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("bingnews: %w", err)
	}

	first := (opts.Page-1)*10 + 1
	sfx := opts.Page - 1

	u := "https://www.bing.com/news/infinitescrollajax?q=" + url.QueryEscape(query) +
		"&InfiniteScroll=1&first=" + strconv.Itoa(first) +
		"&SFX=" + strconv.Itoa(sfx) +
		"&form=PTFTNR"

	if opts.TimeRange != "" {
		param := timeRangeToBingNews(opts.TimeRange)
		if param != "" {
			u += "&qft=interval=\"" + param + "\""
		}
	}

	if opts.Locale != "" {
		u += "&mkt=" + url.QueryEscape(opts.Locale)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("bingnews: create request: %w", err)
	}
	e.setHeaders(req, opts.Locale)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bingnews: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bingnews: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bingnews: read body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("bingnews: parse HTML: %w", err)
	}

	return e.parseResults(doc), nil
}

func (e *BingNewsEngine) parseResults(doc *goquery.Document) []Result {
	var results []Result
	doc.Find("div.newsitem").Each(func(i int, s *goquery.Selection) {
		r := Result{}

		titleLink := s.Find("a.title[href]")
		if href, ok := titleLink.Attr("href"); ok {
			r.URL = href
		}

		r.Title = strings.TrimSpace(titleLink.Text())
		r.Snippet = strings.TrimSpace(s.Find("div.snippet").Text())

		thumbImg := s.Find("a.imagelink img")
		if src, ok := thumbImg.Attr("src"); ok {
			if strings.HasPrefix(src, "/") {
				src = "https://www.bing.com" + src
			}
			r.Thumbnail = src
		}

		if r.URL == "" {
			return
		}
		results = append(results, r)
	})
	return results
}

func timeRangeToBingNews(tr string) string {
	switch tr {
	case "day":
		return "4"
	case "week":
		return "7"
	case "month":
		return "9"
	case "year":
		return "9"
	default:
		return ""
	}
}

func (e *BingNewsEngine) setHeaders(req *http.Request, locale string) {
	if locale == "" {
		locale = "en-US"
	}
	req.Header.Set("User-Agent", e.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", locale)
}
