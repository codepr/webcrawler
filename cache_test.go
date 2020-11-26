package crawler

import "testing"

func TestCacheSet(t *testing.T) {
	cache := newMemoryCache()
	cache.Set("test", "hello")
	if !cache.Contains("test", "hello") {
		t.Errorf("TestCacheSet#Set failed: expected true got false")
	}
}

func TestCacheContains(t *testing.T) {
	cache := newMemoryCache()
	cache.Set("test", "hello")
	if !cache.Contains("test", "hello") {
		t.Errorf("TestCacheSet#Set failed: expected true got false")
	}
	if cache.Contains("test", "world") {
		t.Errorf("TestCacheSet#Set failed: expected false got true")
	}
}
