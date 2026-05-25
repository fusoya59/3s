package cache

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Cache is a thread-safe SQLite key-value store with TTL.
type Cache struct {
	db   *sql.DB
	mu   sync.Mutex
	ttl  time.Duration
	done chan struct{}
}

// Open creates or opens a SQLite cache database at path.
func Open(path string, ttl time.Duration) (*Cache, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("cache open: %w", err)
	}

	// WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cache wal: %w", err)
	}

	// Create tables
	createSQL := `CREATE TABLE IF NOT EXISTS cache (
		key TEXT PRIMARY KEY,
		value BLOB NOT NULL,
		expires_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache(expires_at);`

	if _, err := db.Exec(createSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cache create table: %w", err)
	}

	c := &Cache{
		db:   db,
		ttl:  ttl,
		done: make(chan struct{}),
	}

	// Start periodic purge goroutine
	go c.periodicPurge()

	return c, nil
}

// Close stops the purge goroutine and closes the database.
func (c *Cache) Close() error {
	close(c.done)
	return c.db.Close()
}

// Get retrieves a value by key. Returns nil, false if missing or expired.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var value []byte
	var expiresAt sql.NullInt64

	err := c.db.QueryRow(
		"SELECT value, expires_at FROM cache WHERE key = ?", key,
	).Scan(&value, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		return nil, false
	}

	// Check expiry
	if expiresAt.Valid && expiresAt.Int64 > 0 && time.Now().Unix() > expiresAt.Int64 {
		// Expired — delete and return miss
		_, _ = c.db.Exec("DELETE FROM cache WHERE key = ?", key)
		return nil, false
	}

	return value, true
}

// Set stores a value with the configured TTL.
func (c *Cache) Set(key string, value []byte) {
	c.SetTTL(key, value, c.ttl)
}

// SetTTL stores a value with a specific TTL.
func (c *Cache) SetTTL(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt int64
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).Unix()
	}

	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO cache (key, value, expires_at) VALUES (?, ?, ?)",
		key, value, expiresAt,
	)
	if err != nil {
		return
	}
}

// Delete removes a key from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = c.db.Exec("DELETE FROM cache WHERE key = ?", key)
}

// PurgeExpired removes all expired entries.
func (c *Cache) PurgeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = c.db.Exec("DELETE FROM cache WHERE expires_at > 0 AND expires_at < ?", time.Now().Unix())
}

// VQDGet retrieves a DuckDuckGo VQD token from cache.
func (c *Cache) VQDGet(query, ua string) (string, bool) {
	if c == nil {
		return "", false
	}
	key := vqdCacheKey(query, ua)
	data, ok := c.Get(key)
	if !ok {
		return "", false
	}
	return string(data), true
}

// VQDSet stores a DuckDuckGo VQD token with 1-hour TTL.
func (c *Cache) VQDSet(query, ua, vqd string) {
	if c == nil {
		return
	}
	key := vqdCacheKey(query, ua)
	c.SetTTL(key, []byte(vqd), 1*time.Hour)
}

func vqdCacheKey(query, ua string) string {
	h := sha256.Sum256([]byte(query + "\x00" + ua))
	return "vqd:ddg:" + fmt.Sprintf("%x", h)
}

// Key creates a deterministic cache key from parts using SHA-256.
func Key(parts ...string) string {
	data := strings.Join(parts, "\x00")
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h)
}

func (c *Cache) periodicPurge() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.PurgeExpired()
		case <-c.done:
			return
		}
	}
}
