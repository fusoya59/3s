package scraper

import (
	"fmt"
	"strings"
	"time"
)

// FetchResult holds the scraped page data.
type FetchResult struct {
	Title     string
	RawHTML   string
	FetchedAt string
	Error     string
}

// FetchPage navigates to a URL, scrolls, waits for content, and extracts HTML + shadow DOM.
func FetchPage(sp *ScrapePage, url string, timeout time.Duration, minChars int, pollTimeout int) FetchResult {
	fetchedAt := time.Now().UTC().Format(time.RFC3339)

	// Navigate
	if err := sp.Navigate(url); err != nil {
		return FetchResult{
			Error:     fmt.Sprintf("navigate: %v", err),
			FetchedAt: fetchedAt,
		}
	}

	// Initial sleep for JS rendering
	sp.Sleep(500 * time.Millisecond)

	// Scroll to trigger lazy loading
	_, _ = sp.Evaluate("window.scrollTo(0, document.body.scrollHeight * 0.3)")
	sp.Sleep(300 * time.Millisecond)
	_, _ = sp.Evaluate("window.scrollTo(0, document.body.scrollHeight * 0.7)")
	sp.Sleep(300 * time.Millisecond)

	// Content poll: wait until textContent > minChars
	pollJS := `(() => {
		const check = () => document.body && document.body.textContent ? document.body.textContent.length : 0;
		return check();
	})()`

	pollDeadline := time.Now().Add(time.Duration(pollTimeout) * time.Second)
	for time.Now().Before(pollDeadline) {
		val, err := sp.Evaluate(pollJS)
		if err == nil {
			if length, ok := val.(float64); ok && int(length) >= minChars {
				break
			}
		}
		sp.Sleep(500 * time.Millisecond)
	}

	// Extract title
	title, err := sp.Title()
	if err != nil {
		title = ""
	}

	// Extract raw HTML
	rawHTML, err := sp.Content()
	if err != nil {
		return FetchResult{
			Title:     title,
			Error:     fmt.Sprintf("extract html: %v", err),
			FetchedAt: fetchedAt,
		}
	}

	// Extract shadow DOM content
	shadowJS := `(() => {
		function getText(root) {
			let text = '';
			try {
				const iter = document.createNodeIterator(root, NodeFilter.SHOW_TEXT, {
					acceptNode: (n) => {
						if (n.parentElement && n.parentElement.tagName === 'STYLE')
							return NodeFilter.FILTER_REJECT;
						return NodeFilter.FILTER_ACCEPT;
					}
				});
				let node;
				while ((node = iter.nextNode())) text += node.textContent + ' ';
			} catch(e) {}
			return text.replace(/\s+/g, ' ').trim();
		}
		let parts = [];
		const walk = (root) => {
			const it = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT, null, false);
			let node;
			while ((node = it.nextNode())) {
				try {
					if (node.shadowRoot) {
						const text = getText(node.shadowRoot);
						if (text.length > 100) parts.push(text);
						walk(node.shadowRoot);
					}
				} catch(e) {}
			}
		};
		walk(document);
		return parts.join('\n\n');
	})()`

	shadowText, err := sp.Evaluate(shadowJS)
	if err == nil {
		if text, ok := shadowText.(string); ok && text != "" {
			// Merge shadow content into HTML before </body>
			shadowDiv := fmt.Sprintf("\n<!-- shadow-dom-content -->\n<pre>%s</pre>\n", text)
			if idx := strings.LastIndex(rawHTML, "</html>"); idx >= 0 {
				rawHTML = rawHTML[:idx] + shadowDiv + "\n</body></html>"
			}
		}
	}

	return FetchResult{
		Title:     title,
		RawHTML:   rawHTML,
		FetchedAt: fetchedAt,
	}
}
