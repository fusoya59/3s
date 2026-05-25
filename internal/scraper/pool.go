package scraper

import (
	"fmt"
	"sync"
	"time"

	"github.com/joakimcarlsson/bonk"
)

// Pool manages a single bonk browser instance with shared context.
type Pool struct {
	browser *bonk.Browser
	ctx     *bonk.BrowserContext
	mu      sync.Mutex
	binPath string
	timeout time.Duration
}

// ScrapePage wraps a bonk page with mutex-serialized CDP calls.
type ScrapePage struct {
	page *bonk.Page
	pool *Pool
}

// NewPool creates a new browser pool.
func NewPool(binPath string, timeout time.Duration) (*Pool, error) {
	opts := []bonk.LaunchOption{
		bonk.Headless(true),
		bonk.Stealth(true),
		bonk.Args("--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"),
	}
	if binPath != "" {
		opts = append(opts, bonk.ChromePath(binPath))
	}

	browser, err := bonk.Launch(opts...)
	if err != nil {
		return nil, fmt.Errorf("scraper pool: launch browser: %w", err)
	}

	bctx, err := browser.NewContext()
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("scraper pool: create context: %w", err)
	}

	return &Pool{
		browser: browser,
		ctx:     bctx,
		binPath: binPath,
		timeout: timeout,
	}, nil
}

// Acquire creates a new page in the shared context.
func (p *Pool) Acquire() (*ScrapePage, error) {
	p.mu.Lock()
	page, err := p.ctx.NewPage()
	p.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("scraper pool: acquire page: %w", err)
	}
	// Set page timeout
	page.SetDefaultNavigationTimeout(p.timeout)
	return &ScrapePage{page: page, pool: p}, nil
}

// Release closes a page and returns it to the pool.
func (p *Pool) Release(sp *ScrapePage) {
	if sp == nil || sp.page == nil {
		return
	}
	p.mu.Lock()
	_ = sp.page.Close()
	p.mu.Unlock()
}

// Close shuts down the browser pool.
func (p *Pool) Close() error {
	if p.ctx != nil {
		_ = p.ctx.Close()
	}
	if p.browser != nil {
		return p.browser.Close()
	}
	return nil
}

// HealthCheck verifies the browser is functional.
func (p *Pool) HealthCheck() error {
	sp, err := p.Acquire()
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer p.Release(sp)

	_, err = sp.Evaluate("1+1")
	if err != nil {
		return fmt.Errorf("health check: browser evaluate: %w", err)
	}
	return nil
}

// Navigate navigates to a URL. All CDP methods lock pool.mu.
func (sp *ScrapePage) Navigate(url string) error {
	sp.pool.mu.Lock()
	defer sp.pool.mu.Unlock()
	return sp.page.Navigate(url, bonk.WithWaitUntil(bonk.WaitDOMContentLoaded))
}

// Evaluate executes JavaScript. All CDP methods lock pool.mu.
func (sp *ScrapePage) Evaluate(js string) (any, error) {
	sp.pool.mu.Lock()
	defer sp.pool.mu.Unlock()
	return sp.page.Evaluate(js)
}

// Title returns the page title. All CDP methods lock pool.mu.
func (sp *ScrapePage) Title() (string, error) {
	sp.pool.mu.Lock()
	defer sp.pool.mu.Unlock()
	return sp.page.Title()
}

// Content returns full page HTML. All CDP methods lock pool.mu.
func (sp *ScrapePage) Content() (string, error) {
	sp.pool.mu.Lock()
	defer sp.pool.mu.Unlock()
	return sp.page.Content()
}

// Sleep pauses execution. Does NOT lock pool.mu.
func (sp *ScrapePage) Sleep(d time.Duration) {
	time.Sleep(d)
}
