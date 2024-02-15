package cache

import "time"

// ICache is an interface that defines how the pages server interacts with the cache.
type ICache interface {
	Set(key string, value interface{}, ttl time.Duration) error
	Get(key string) (interface{}, bool)
	Remove(key string)
}
