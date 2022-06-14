package upstream

import "time"

// defaultBranchCacheTimeout specifies the timeout for the default branch cache. It can be quite long.
var defaultBranchCacheTimeout = 15 * time.Minute

// branchExistenceCacheTimeout specifies the timeout for the branch timestamp & existence cache. It should be shorter
// than fileCacheTimeout, as that gets invalidated if the branch timestamp has changed. That way, repo changes will be
// picked up faster, while still allowing the content to be cached longer if nothing changes.
var branchExistenceCacheTimeout = 5 * time.Minute

// fileCacheTimeout specifies the timeout for the file content cache - you might want to make this quite long, depending
// on your available memory.
// TODO: move as option into cache interface
var fileCacheTimeout = 5 * time.Minute

// fileCacheSizeLimit limits the maximum file size that will be cached, and is set to 1 MB by default.
var fileCacheSizeLimit = 1024 * 1024

// canonicalDomainCacheTimeout specifies the timeout for the canonical domain cache.
var canonicalDomainCacheTimeout = 15 * time.Minute

const canonicalDomainConfig = ".domains"
