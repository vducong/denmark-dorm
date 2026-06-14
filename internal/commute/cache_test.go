package commute

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCache_missingFileEmpty(t *testing.T) {
	c, err := loadCache(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("loadCache err = %v", err)
	}
	if _, ok := c.get("x"); ok {
		t.Errorf("expected empty cache")
	}
}

func TestCache_roundTripAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "commute_cache.json")
	c, _ := loadCache(path)
	c.put("walk||x|y", "12")
	if err := c.flush(path); err != nil {
		t.Fatalf("flush err = %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file should not be left behind")
	}

	c2, err := loadCache(path)
	if err != nil {
		t.Fatalf("reload err = %v", err)
	}
	if v, ok := c2.get("walk||x|y"); !ok || v != "12" {
		t.Errorf("reloaded = %q ok=%v, want 12", v, ok)
	}
}

func TestCache_flushNoopWhenClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	c, _ := loadCache(path)
	if err := c.flush(path); err != nil {
		t.Fatalf("flush err = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a clean flush should not create a file")
	}
}
