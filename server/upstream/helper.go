package upstream

import (
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"codeberg.org/codeberg/pages/server/cache"
)

type branchTimestamp struct {
	Branch    string
	Timestamp time.Time
}

// GetBranchTimestamp finds the default branch (if branch is "") and returns the last modification time of the branch
// (or nil if the branch doesn't exist)
func GetBranchTimestamp(owner, repo, branch, giteaRoot, giteaApiToken string, branchTimestampCache cache.SetGetKey) *branchTimestamp {
	if result, ok := branchTimestampCache.Get(owner + "/" + repo + "/" + branch); ok {
		if result == nil {
			return nil
		}
		return result.(*branchTimestamp)
	}
	result := &branchTimestamp{}
	result.Branch = branch
	if branch == "" {
		// Get default branch
		body := make([]byte, 0)
		// TODO: use header for API key?
		status, body, err := fasthttp.GetTimeout(body, giteaRoot+"/api/v1/repos/"+owner+"/"+repo+"?access_token="+giteaApiToken, 5*time.Second)
		if err != nil || status != 200 {
			_ = branchTimestampCache.Set(owner+"/"+repo+"/"+branch, nil, defaultBranchCacheTimeout)
			return nil
		}
		result.Branch = fastjson.GetString(body, "default_branch")
	}

	body := make([]byte, 0)
	status, body, err := fasthttp.GetTimeout(body, giteaRoot+"/api/v1/repos/"+owner+"/"+repo+"/branches/"+branch+"?access_token="+giteaApiToken, 5*time.Second)
	if err != nil || status != 200 {
		return nil
	}

	result.Timestamp, _ = time.Parse(time.RFC3339, fastjson.GetString(body, "commit", "timestamp"))
	_ = branchTimestampCache.Set(owner+"/"+repo+"/"+branch, result, branchExistenceCacheTimeout)
	return result
}

type fileResponse struct {
	exists   bool
	mimeType string
	body     []byte
}
