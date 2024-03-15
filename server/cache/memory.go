package cache

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OrlovEvgeny/go-mcache"
	"github.com/rs/zerolog/log"
)

type Cache struct {
	mcache      *mcache.CacheDriver
	memoryLimit int
	lastCheck   time.Time
}

// NewInMemoryCache returns a new mcache that can grow infinitely.
func NewInMemoryCache() ICache {
	return mcache.New()
}

// NewInMemoryCache returns a new mcache with a memory limit.
// If the limit is exceeded, the cache will be cleared.
func NewInMemoryCacheWithLimit(memoryLimit int) ICache {
	return &Cache{
		mcache:      mcache.New(),
		memoryLimit: memoryLimit,
	}
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) error {
	now := time.Now()

	// we don't want to do it too often
	if now.Sub(c.lastCheck) > (time.Second * 3) {
		if c.memoryLimitOvershot() {
			log.Info().Msg("[cache] memory limit exceeded, clearing cache")
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
	mem := getCurrentMemory()

	log.Debug().Int("kB", mem).Msg("[cache] current memory usage")

	return mem > c.memoryLimit
}

// getCurrentMemory returns the current memory in KB
func getCurrentMemory() int {
	b, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		log.Error().Err(err).Msg("[cache] could not read /proc/self/statm")
		return 0
	}

	str := string(b)
	arr := strings.Split(str, " ")

	// convert to pages
	res, err := strconv.Atoi(arr[1])
	if err != nil {
		log.Error().Err(err).Msg("[cache] could not convert string to int")
		return 0
	}

	// convert to KB
	return (res * os.Getpagesize()) / 1024
}
