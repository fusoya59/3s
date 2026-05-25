package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// BraveEngine searches Brave Search (HTML scraping).
type BraveEngine struct {
	client *http.Client
	ua     string
}

// NewBraveEngine creates a new Brave engine.
func NewBraveEngine(client *http.Client, ua string) *BraveEngine {
	return &BraveEngine{client: client, ua: ua}
}

// Name returns "brave".
func (e *BraveEngine) Name() string { return "brave" }

// Search executes a search against Brave.
func (e *BraveEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("brave: %w", err)
	}
	return e.searchGeneral(ctx, query, opts)
}

func (e *BraveEngine) searchGeneral(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if opts.Page > 10 {
		return nil, nil
	}

	u := "https://search.brave.com/search?q=" + url.QueryEscape(query) + "&source=web"
	if opts.Page > 1 {
		offset := opts.Page - 1
		u += "&offset=" + strconv.Itoa(offset)
	}
	if opts.TimeRange != "" {
		param, ok := map[string]string{
			"day":   "pd",
			"week":  "pw",
			"month": "pm",
			"year":  "py",
		}[opts.TimeRange]
		if ok {
			u += "&tf=" + param
		}
	}

	return braveFetchAndParse(ctx, u, opts, "general", e.client, e.ua)
}

func braveParseGeneral(doc *goquery.Document) []Result {
	var results []Result
	doc.Find("div.snippet").Each(func(i int, s *goquery.Selection) {
		r := Result{}

		link := s.Find("a[href]").First()
		if href, ok := link.Attr("href"); ok {
			r.URL = href
		}

		if r.URL != "" {
			if parsed, parseErr := url.Parse(r.URL); parseErr != nil || parsed.Host == "" {
				return
			}
		}

		r.Title = strings.TrimSpace(s.Find("div.title").Text())

		snippet := strings.TrimSpace(s.Find("div.content").Text())
		dateText := strings.TrimSpace(s.Find("span.t-secondary").Text())
		r.Published = time.Time{}
		if dateText != "" {
			if t, err := parseBraveDate(dateText); err == nil {
				r.Published = t
				snippet = strings.TrimLeft(snippet, dateText)
				snippet = strings.TrimLeft(snippet, " -\n\t")
			}
		}
		r.Snippet = snippet

		if thumbImg, ok := s.Find("a.thumbnail img").Attr("src"); ok {
			r.Thumbnail = thumbImg
		}

		if r.URL == "" {
			return
		}
		results = append(results, r)
	})
	return results
}

func braveParseNews(doc *goquery.Document) []Result {
	var results []Result
	doc.Find("div[data-type='news']").Each(func(i int, s *goquery.Selection) {
		r := Result{}

		link := s.Find("a.result-header[href]").First()
		if href, ok := link.Attr("href"); ok {
			r.URL = href
		}

		r.Title = strings.TrimSpace(s.Find("span.snippet-title").Text())
		r.Snippet = strings.TrimSpace(s.Find("p.desc").Text())

		if thumbImg, ok := s.Find("div.image-wrapper img").Attr("src"); ok {
			r.Thumbnail = thumbImg
		}

		if r.URL == "" {
			return
		}
		results = append(results, r)
	})
	return results
}

func braveSetHeaders(req *http.Request, opts SearchOptions, ua string) {
	safesearch := "off"
	switch opts.SafeSearch {
	case SafeSearchModerate:
		safesearch = "moderate"
	case SafeSearchStrict:
		safesearch = "strict"
	}

	country, uiLang := braveLocale(opts.Locale)

	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", uiLang+",en;q=0.5")
	req.Header.Set("Cookie", fmt.Sprintf("safesearch=%s; useLocation=0; summarizer=0; country=%s; ui_lang=%s", safesearch, country, uiLang))
}

func braveFetchAndParse(ctx context.Context, urlStr string, opts SearchOptions, mode string, client *http.Client, ua string) ([]Result, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("brave: create request: %w", err)
	}
	braveSetHeaders(req, opts, ua)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave: HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("brave: parse HTML: %w", err)
	}

	if mode == "general" {
		return braveParseGeneral(doc), nil
	}
	return braveParseNews(doc), nil
}

var braveLocaleMap = map[string]struct{ country, uiLang string }{
	"en-US": {"us", "en-us"},
	"en-GB": {"gb", "en-gb"},
	"de-DE": {"de", "de-de"},
	"fr-FR": {"fr", "fr-fr"},
	"es-ES": {"es", "es-es"},
	"ja-JP": {"jp", "ja-jp"},
	"it-IT": {"it", "it-it"},
	"pt-BR": {"br", "pt-br"},
	"ru-RU": {"ru", "ru-ru"},
	"zh-CN": {"cn", "zh-cn"},
	"zh-TW": {"tw", "zh-tw"},
	"ko-KR": {"kr", "ko-kr"},
	"nl-NL": {"nl", "nl-nl"},
	"sv-SE": {"se", "sv-se"},
	"da-DK": {"dk", "da-dk"},
	"fi-FI": {"fi", "fi-fi"},
	"nb-NO": {"no", "nb-no"},
	"pl-PL": {"pl", "pl-pl"},
	"tr-TR": {"tr", "tr-tr"},
	"ar-SA": {"sa", "ar-sa"},
	"th-TH": {"th", "th-th"},
	"vi-VN": {"vn", "vi-vn"},
	"id-ID": {"id", "id-id"},
	"ms-MY": {"my", "ms-my"},
}

func braveLocale(locale string) (country, uiLang string) {
	if locale == "" {
		return "us", "en-us"
	}
	if m, ok := braveLocaleMap[locale]; ok {
		return m.country, m.uiLang
	}
	return "us", "en-us"
}

func parseBraveDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	if t, err := time.Parse("Jan 2, 2006", s); err == nil {
		return t, nil
	}

	fields := strings.Fields(s)
	if len(fields) < 2 {
		return time.Time{}, fmt.Errorf("unrecognized date: %s", s)
	}

	num, err := strconv.Atoi(fields[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("unrecognized date: %s", s)
	}

	unit := strings.ToLower(strings.TrimRight(fields[1], "s"))

	switch unit {
	case "minute":
		return time.Now().Add(-time.Duration(num) * time.Minute), nil
	case "hour":
		return time.Now().Add(-time.Duration(num) * time.Hour), nil
	case "day":
		return time.Now().Add(-time.Duration(num) * 24 * time.Hour), nil
	case "week":
		return time.Now().Add(-time.Duration(num) * 7 * 24 * time.Hour), nil
	case "month":
		return time.Now().AddDate(0, -num, 0), nil
	case "year":
		return time.Now().AddDate(-num, 0, 0), nil
	}

	return time.Time{}, fmt.Errorf("unrecognized date: %s", s)
}

// BraveNewsEngine searches Brave News (HTML scraping).
type BraveNewsEngine struct {
	client *http.Client
	ua     string
}

// NewBraveNewsEngine creates a new Brave News engine.
func NewBraveNewsEngine(client *http.Client, ua string) *BraveNewsEngine {
	return &BraveNewsEngine{client: client, ua: ua}
}

// Name returns "brave-news".
func (e *BraveNewsEngine) Name() string { return "brave-news" }

// Search executes a search against Brave News.
func (e *BraveNewsEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("brave-news: %w", err)
	}

	u := "https://search.brave.com/news?q=" + url.QueryEscape(query) + "&source=web"
	return braveFetchAndParse(ctx, u, opts, "news", e.client, e.ua)
}
