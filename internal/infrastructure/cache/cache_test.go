package cache

import (
	"testing"
	"time"
)

func TestCacheSetAndGet(t *testing.T) {
	c := New[string](10, time.Minute)

	c.Set("key1", "value1")
	if v, ok := c.Get("key1"); !ok || v != "value1" {
		t.Errorf("expected value1, got %q (ok=%v)", v, ok)
	}
}

func TestCacheMiss(t *testing.T) {
	c := New[string](10, time.Minute)

	if _, ok := c.Get("nonexistent"); ok {
		t.Error("expected miss")
	}
}

func TestCacheExpiration(t *testing.T) {
	c := New[string](10, 10*time.Millisecond)

	c.Set("key1", "value1")
	time.Sleep(20 * time.Millisecond)

	if _, ok := c.Get("key1"); ok {
		t.Error("expected expiration")
	}
}

func TestCacheMaxSize(t *testing.T) {
	c := New[string](2, time.Minute)

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3") // Should evict key1

	if _, ok := c.Get("key1"); ok {
		t.Error("expected key1 to be evicted")
	}
	if v, ok := c.Get("key3"); !ok || v != "value3" {
		t.Errorf("expected key3, got %q (ok=%v)", v, ok)
	}
}

func TestCacheDelete(t *testing.T) {
	c := New[string](10, time.Minute)

	c.Set("key1", "value1")
	c.Delete("key1")

	if _, ok := c.Get("key1"); ok {
		t.Error("expected key to be deleted")
	}
}

func TestCacheInvalidatePrefix(t *testing.T) {
	c := New[string](10, time.Minute)

	c.Set("projects:1", "p1")
	c.Set("projects:2", "p2")
	c.Set("materials:1", "m1")

	c.InvalidatePrefix("projects:")

	if _, ok := c.Get("projects:1"); ok {
		t.Error("expected projects:1 to be invalidated")
	}
	if _, ok := c.Get("projects:2"); ok {
		t.Error("expected projects:2 to be invalidated")
	}
	if v, ok := c.Get("materials:1"); !ok || v != "m1" {
		t.Error("expected materials:1 to still exist")
	}
}

func TestCacheClear(t *testing.T) {
	c := New[string](10, time.Minute)

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Clear()

	if c.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", c.Size())
	}
}

func TestCacheSize(t *testing.T) {
	c := New[string](10, time.Minute)

	if c.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", c.Size())
	}

	c.Set("key1", "value1")
	if c.Size() != 1 {
		t.Errorf("expected size 1, got %d", c.Size())
	}
}
