package upstream

import "time"

// DefaultBranchCacheTimeout specifies the timeout for the default branch cache. It can be quite long.
var DefaultBranchCacheTimeout = 15 * time.Minute

// BranchExistanceCacheTimeout specifies the timeout for the branch timestamp & existance cache. It should be shorter
// than FileCacheTimeout, as that gets invalidated if the branch timestamp has changed. That way, repo changes will be
// picked up faster, while still allowing the content to be cached longer if nothing changes.
var BranchExistanceCacheTimeout = 5 * time.Minute

// FileCacheTimeout specifies the timeout for the file content cache - you might want to make this quite long, depending
// on your available memory.
var FileCacheTimeout = 5 * time.Minute

// FileCacheSizeLimit limits the maximum file size that will be cached, and is set to 1 MB by default.
var FileCacheSizeLimit = 1024 * 1024

// CanonicalDomainCacheTimeout specifies the timeout for the canonical domain cache.
var CanonicalDomainCacheTimeout = 15 * time.Minute
