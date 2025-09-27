package utils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// A simple cache entry in the in-memory cache system
type CacheEntry struct {
	Key        string        // The key of this entry
	Value      any           // The value of the cached item
	Expiration time.Time     // The expiration time
	Start      bool          // Start the expiration counter
	TimeToLive time.Duration // The time to live of this entry
	HitCount   int           // The total hit count for this item
}

func (ce *CacheEntry) String() string {
	return fmt.Sprintf(
		"Entry{ Key=%v, Expiration=%v, TTL=%v }",
		ce.Key,
		ce.Expiration.Format(time.RFC3339),
		ce.TimeToLive)
}

// Memcache is an in-memory concurrent cache
type Cache struct {
	items  map[string]*CacheEntry // A mapping of keys to cache entries
	mu     sync.RWMutex
	maxTTL time.Duration // Maximum time to live
}

// NewCache returns a new Cache object
func NewCache(max_ttl time.Duration) *Cache {
	return &Cache{
		items:  make(map[string]*CacheEntry),
		maxTTL: max_ttl,
	}
}

// ComputeNewExpiration computes the new expiration extension using a
// logarithmic decay which introduces a diminishing returns when
// the hit count becomes very high.
func ComputeNewExpiration(item *CacheEntry, max_ttl time.Duration) time.Duration {
	baseTTL := float64(item.TimeToLive)
	extension := time.Duration(baseTTL * math.Log(float64(item.HitCount)+1))
	extension = min(extension, max_ttl)
	return extension
}

// Set adds a new item into the cache with specified TTL
func (c *Cache) Set(key string, value any, ttl time.Duration, start bool) {
	c.mu.Lock()         // Lock the mutex when inserting a new element
	defer c.mu.Unlock() // When the function exits release the lock

	var exp time.Time
	ttl = min(ttl, c.maxTTL)
	if ttl > 0 && start {
		exp = time.Now().Add(ttl)
	}

	// Adds the item into the cache
	c.items[key] = &CacheEntry{key, value, exp, start, ttl, 0}

	INFO("New Cache Entry: ", c.items[key])
}

// Start starts the expiration process of the cache element
func (c *Cache) Start(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already started then do nothing and returns
	if item, exists := c.items[key]; !exists || item.Start {
		ERROR("Element ", key, " does not exists in the cache!!")
		return
	}

	ttl := c.items[key].TimeToLive
	c.items[key].Start = true
	c.items[key].Expiration = time.Now().Add(ttl)
}

// UnsafeHasExpired returns True if the element has expired, false otherwise
// It is unsafe since no locking mechanisms prevents race conditions
func (c *Cache) UnsafeHasExpired(key string) (bool, error) {
	item, exists := c.items[key]
	if !exists {
		// Cache miss
		return true, errors.New("cache miss for key: " + key)
	}

	if !item.Start {
		return false, nil
	}

	// If the current time is after the expiration date
	if time.Now().After(item.Expiration) {
		return true, nil
	}

	return false, nil
}

// HasExpired Concurrent safe version of UnsafeHasExpire
func (c *Cache) HasExpired(key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.UnsafeHasExpired(key)
}

// UnsafeGetEntry non-Concurrent safe function which returns the
// entry in the cache correspondin to the input key
func (c *Cache) UnsafeGetEntry(key string) *CacheEntry {
	// For cache miss, i.e., expiration or never existed return nil
	if result, _ := c.UnsafeHasExpired(key); result {
		return nil
	}

	// Before returning the value increase the hit count
	item := c.items[key]

	// The hit count and the ttl extension depends on the start value
	if item.Start {
		item.HitCount++

		// Increase its expiration date base on the hit count
		extension := ComputeNewExpiration(item, c.maxTTL)
		item.Expiration = time.Now().Add(extension)
	}

	return item
}

func (c *Cache) GetWithDefault(key string, value any) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item := c.UnsafeGetEntry(key); item != nil {
		return item.Value, true
	}
	return value, false
}

// Get returns the value of the key for Cache hit, nil on cache miss
func (c *Cache) Get(key string) (any, bool) {
	return c.GetWithDefault(key, nil)
}

// Modify modifies the value of an entry if it exists. In the case
// the entry exists its lifetime will be extended (Cache hit).
func (c *Cache) Modify(key string, value any) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item := c.UnsafeGetEntry(key); item != nil {
		item.Value = value
		return true
	}

	return false
}

// Remove removes an item from the cache if it exists
func (c *Cache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *Cache) EraseCache() {
	for key := range c.items {
		delete(c.items, key)
	}
}

// ExpirationRoutine checks when cache entry has expired and removes it.
func (c *Cache) ExpirationRoutine() {
	for key := range c.items {
		if result, _ := c.UnsafeHasExpired(key); result {
			INFO("Item ", key, " has expired")
			c.Remove(key)
		}
	}
}

// Run runs the cache expiration routine every interval of time
func (c *Cache) Run(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		INFO("Cache routine started")

		for {
			select {
			case <-ticker.C:
				c.ExpirationRoutine()
			case <-ctx.Done():
				// When the context is closed then exit
				INFO("Cache routine canceled: ", ctx.Err())
				c.EraseCache()
				return
			}
		}
	}()
}
