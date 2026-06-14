package commute

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// cache is the persisted leg cache: a commute leg's key maps to its value in
// whole minutes (or "" for a legitimate no-route). Only successful lookups are
// stored, so a transient API failure is retried on the next run rather than
// poisoned with a blank.
type cache struct {
	mu      sync.Mutex
	entries map[string]string
	dirty   bool
}

// loadCache reads the JSON cache at path. A missing or empty file yields an
// empty cache, not an error, so the first run starts clean.
func loadCache(path string) (*cache, error) {
	c := &cache{entries: map[string]string{}}
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read commute cache %s: %w", path, err)
	}
	if len(data) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(data, &c.entries); err != nil {
		return nil, fmt.Errorf("parse commute cache %s: %w", path, err)
	}
	return c, nil
}

func (c *cache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	return v, ok
}

func (c *cache) put(key, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.entries[key]; ok && old == val {
		return
	}
	c.entries[key] = val
	c.dirty = true
}

// flush writes the cache to path atomically (temp file + rename) when something
// changed. A no-op when nothing was added, so cache-only runs touch no disk.
func (c *cache) flush(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty || path == "" {
		return nil
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create commute cache dir: %w", err)
		}
	}
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode commute cache: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write commute cache: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit commute cache: %w", err)
	}
	c.dirty = false
	return nil
}
