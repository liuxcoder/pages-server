package cache

import (
	"runtime"
	"time"

	"github.com/OrlovEvgeny/go-mcache"
	"github.com/rs/zerolog/log"
)

type Cache struct {
	mcache      *mcache.CacheDriver
	memoryLimit uint64
	lastCheck   time.Time
}

// NewInMemoryCache returns a new mcache that can grow infinitely.
func NewInMemoryCache() ICache {
	return mcache.New()
}

// NewInMemoryCache returns a new mcache with a memory limit.
// If the limit is exceeded, the cache will be cleared.
func NewInMemoryCacheWithLimit(memoryLimit uint64) ICache {
	return &Cache{
		mcache:      mcache.New(),
		memoryLimit: memoryLimit,
	}
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) error {
	now := time.Now()

	// checking memory limit is a "stop the world" operation
	// so we don't want to do it too often
	if now.Sub(c.lastCheck) > (time.Second * 3) {
		if c.memoryLimitOvershot() {
			log.Debug().Msg("memory limit exceeded, clearing cache")
			c.mcache.Truncate()
		}
		c.lastCheck = now
	}

	return c.mcache.Set(key, value, ttl)
}

func (c *Cache) Get(key string) (interface{}, bool) {
	return c.mcache.Get(key)
}

func (c *Cache) Remove(key string) {
	c.mcache.Remove(key)
}

func (c *Cache) memoryLimitOvershot() bool {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	log.Debug().Uint64("bytes", stats.HeapAlloc).Msg("current memory usage")

	return stats.HeapAlloc > c.memoryLimit
}
