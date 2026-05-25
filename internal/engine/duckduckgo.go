package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// DuckDuckGoEngine searches DuckDuckGo (HTML scraping).
type DuckDuckGoEngine struct {
	client *http.Client
	ua     string
	vqd    VQDProvider
}

// NewDuckDuckGoEngine creates a new DuckDuckGo engine.
func NewDuckDuckGoEngine(client *http.Client, ua string, vqd VQDProvider) *DuckDuckGoEngine {
	return &DuckDuckGoEngine{client: client, ua: ua, vqd: vqd}
}

// Name returns "duckduckgo".
func (e *DuckDuckGoEngine) Name() string { return "duckduckgo" }

const ddgMaxQueryLen = 499

func ddgLocale(locale string) string {
	if locale == "" {
		return "wt-wt"
	}
	m := map[string]string{
		"en-US": "us-en", "en-GB": "uk-en", "de-DE": "de-de",
		"fr-FR": "fr-fr", "es-ES": "es-es", "ja-JP": "jp-jp",
		"it-IT": "it-it", "pt-BR": "br-pt", "nl-NL": "nl-nl",
		"zh-CN": "cn-zh", "zh-TW": "tw-tzh", "ko-KR": "kr-kr",
		"sv-SE": "se-sv", "da-DK": "dk-da", "fi-FI": "fi-fi",
		"nb-NO": "no-nb", "pl-PL": "pl-pl", "tr-TR": "tr-tr",
		"ru-RU": "ru-ru",
	}
	if v, ok := m[locale]; ok {
		return v
	}
	return "wt-wt"
}

func ddgAcceptLanguage(locale string) string {
	if locale == "" {
		return "en-US,en;q=0.7"
	}
	parts := strings.SplitN(locale, "-", 2)
	lang := strings.ToLower(parts[0])
	if len(parts) > 1 {
		return locale + "," + lang + ";q=0.7"
	}
	return lang + ";q=0.7"
}

// Search executes a search against DuckDuckGo.
func (e *DuckDuckGoEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("duckduckgo: %w", err)
	}

	if len(query) > ddgMaxQueryLen {
		return nil, &ErrQueryTooLong{Engine: "duckduckgo", MaxLen: ddgMaxQueryLen}
	}

	query = quoteDDGBangs(query)

	if opts.Page <= 1 {
		return e.searchPage1(ctx, query, opts)
	}
	return e.searchPageN(ctx, query, opts)
}

func (e *DuckDuckGoEngine) searchPage1(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	kl := ddgLocale(opts.Locale)
	form := url.Values{}
	form.Set("q", query)
	form.Set("b", "")
	form.Set("kl", kl)
	form.Set("df", timeRangeToDDG(opts.TimeRange))

	req, err := http.NewRequestWithContext(ctx, "POST", "https://html.duckduckgo.com/html/",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: create request: %w", err)
	}
	e.setHeaders(req, opts.Locale)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusSeeOther {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: read body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: parse HTML: %w", err)
	}

	if doc.Find("form#challenge-form").Length() > 0 {
		return nil, &ErrCaptcha{Engine: "duckduckgo"}
	}

	vqdToken, exists := doc.Find("input[name=\"vqd\"]").Attr("value")
	if exists && vqdToken != "" {
		e.vqd.VQDSet(query, e.ua, vqdToken)
	}

	results := parseDDGResults(doc)
	return results, nil
}

func (e *DuckDuckGoEngine) searchPageN(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	vqd, ok := e.vqd.VQDGet(query, e.ua)
	if !ok {
		return nil, &ErrVQDMissing{Engine: "duckduckgo"}
	}

	offset := 10 + (opts.Page-2)*15
	kl := ddgLocale(opts.Locale)

	form := url.Values{}
	form.Set("q", query)
	form.Set("nextParams", "")
	form.Set("api", "d.js")
	form.Set("o", "json")
	form.Set("v", "l")
	form.Set("vqd", vqd)
	form.Set("dc", fmt.Sprintf("%d", offset+1))
	form.Set("s", fmt.Sprintf("%d", offset))
	form.Set("kl", kl)
	form.Set("df", timeRangeToDDG(opts.TimeRange))

	req, err := http.NewRequestWithContext(ctx, "POST", "https://html.duckduckgo.com/html/",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: create request: %w", err)
	}
	e.setHeaders(req, opts.Locale)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusSeeOther {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: read body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: parse HTML: %w", err)
	}

	if doc.Find("form#challenge-form").Length() > 0 {
		return nil, &ErrCaptcha{Engine: "duckduckgo"}
	}

	if vqdToken, exists := doc.Find("input[name=\"vqd\"]").Attr("value"); exists && vqdToken != "" {
		e.vqd.VQDSet(query, e.ua, vqdToken)
	}

	results := parseDDGResults(doc)
	return results, nil
}

func parseDDGResults(doc *goquery.Document) []Result {
	var results []Result
	doc.Find("div#links div.web-result").Each(func(i int, s *goquery.Selection) {
		if s.HasClass("result--ad") {
			return
		}

		r := Result{}

		link := s.Find("h2 a")
		r.Title = strings.TrimSpace(link.Text())
		if href, ok := link.Attr("href"); ok {
			r.URL = href
		}

		r.Snippet = strings.TrimSpace(s.Find("a.result__snippet").Text())

		if r.URL == "" {
			return
		}
		results = append(results, r)
	})
	return results
}

func quoteDDGBangs(query string) string {
	var result strings.Builder
	i := 0
	for i < len(query) {
		if query[i] == ' ' || query[i] == '\t' {
			result.WriteByte(query[i])
			i++
			continue
		}
		j := i
		for j < len(query) && query[j] != ' ' && query[j] != '\t' {
			j++
		}
		token := query[i:j]
		if len(token) > 1 && token[0] == '!' {
			result.WriteString("'" + token + "'")
		} else {
			result.WriteString(token)
		}
		i = j
	}
	return result.String()
}

func timeRangeToDDG(tr string) string {
	switch tr {
	case "day":
		return "d"
	case "week":
		return "w"
	case "month":
		return "m"
	case "year":
		return "y"
	default:
		return ""
	}
}

func (e *DuckDuckGoEngine) setHeaders(req *http.Request, locale string) {
	req.Header.Set("User-Agent", e.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", ddgAcceptLanguage(locale))
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Referer", "https://html.duckduckgo.com/")
}
