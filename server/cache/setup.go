package cache

import "github.com/OrlovEvgeny/go-mcache"

func NewKeyValueCache() SetGetKey {
	return mcache.New()
}
