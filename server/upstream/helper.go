package upstream

import (
	"time"

	"codeberg.org/codeberg/pages/server/cache"
)

type branchTimestamp struct {
	Branch    string
	Timestamp time.Time
}

// GetBranchTimestamp finds the default branch (if branch is "") and returns the last modification time of the branch
// (or nil if the branch doesn't exist)
func GetBranchTimestamp(owner, repo, branch, giteaRoot, giteaAPIToken string, branchTimestampCache cache.SetGetKey) *branchTimestamp {
	if result, ok := branchTimestampCache.Get(owner + "/" + repo + "/" + branch); ok {
		if result == nil {
			return nil
		}
		return result.(*branchTimestamp)
	}
	result := &branchTimestamp{
		Branch: branch,
	}
	if len(branch) == 0 {
		// Get default branch
		defaultBranch, err := giteaGetRepoDefaultBranch(giteaRoot, owner, repo, giteaAPIToken)
		if err != nil {
			_ = branchTimestampCache.Set(owner+"/"+repo+"/", nil, defaultBranchCacheTimeout)
			return nil
		}
		result.Branch = defaultBranch
	}

	timestamp, err := giteaGetRepoBranchTimestamp(giteaRoot, owner, repo, branch, giteaAPIToken)
	if err != nil {
		return nil
	}
	result.Timestamp = timestamp
	_ = branchTimestampCache.Set(owner+"/"+repo+"/"+branch, result, branchExistenceCacheTimeout)
	return result
}

type fileResponse struct {
	exists   bool
	mimeType string
	body     []byte
}
