package cache

import "github.com/OrlovEvgeny/go-mcache"

func NewInMemoryCache() ICache {
	return mcache.New()
}
