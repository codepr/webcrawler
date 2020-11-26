// Package crawler containing the crawling logics and utilities to scrape
// remote resources on the web
package crawler

import "sync"

// memoryCache is just a simple in-memory thread-safe map to track multiple
// sets of keys
type memoryCache struct {
	mutex sync.RWMutex
	cache map[string]map[string]bool
}

// newMemoryCache creates and return a pointer to a memoryCache object, it
// also inits the outer map, each new key inserted will lazily init the set it
// refers to
func newMemoryCache() *memoryCache {
	return &memoryCache{cache: make(map[string]map[string]bool)}
}

// Set add a new entry to the map and, if it's a new key it also init the set
// it points to, otherwise just add the key to the set
func (c *memoryCache) Set(namespace, key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, ok := c.cache[namespace]
	if !ok {
		c.cache[namespace] = make(map[string]bool)
	}
	c.cache[namespace][key] = true
}

// Contains check if a key is already stored in the cache, to be true the
// cache must contain the namespace key on the outer map and also the key in
// the set referred.
func (c *memoryCache) Contains(namespace, key string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	inner, ok := c.cache[namespace]
	if !ok {
		return false
	}
	return inner[key]
}
