package upstream

import (
	"mime"
	"path"
	"strconv"
	"strings"
	"time"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/gitea"
)

type branchTimestamp struct {
	Branch    string
	Timestamp time.Time
}

// GetBranchTimestamp finds the default branch (if branch is "") and returns the last modification time of the branch
// (or nil if the branch doesn't exist)
func GetBranchTimestamp(giteaClient *gitea.Client, owner, repo, branch string, branchTimestampCache cache.SetGetKey) *branchTimestamp {
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
		defaultBranch, err := giteaClient.GiteaGetRepoDefaultBranch(owner, repo)
		if err != nil {
			_ = branchTimestampCache.Set(owner+"/"+repo+"/", nil, defaultBranchCacheTimeout)
			return nil
		}
		result.Branch = defaultBranch
	}

	timestamp, err := giteaClient.GiteaGetRepoBranchTimestamp(owner, repo, result.Branch)
	if err != nil {
		return nil
	}
	result.Timestamp = timestamp
	_ = branchTimestampCache.Set(owner+"/"+repo+"/"+branch, result, branchExistenceCacheTimeout)
	return result
}

func (o *Options) getMimeTypeByExtension() string {
	if o.ForbiddenMimeTypes == nil {
		o.ForbiddenMimeTypes = make(map[string]bool)
	}
	mimeType := mime.TypeByExtension(path.Ext(o.TargetPath))
	mimeTypeSplit := strings.SplitN(mimeType, ";", 2)
	if o.ForbiddenMimeTypes[mimeTypeSplit[0]] || mimeType == "" {
		if o.DefaultMimeType != "" {
			mimeType = o.DefaultMimeType
		} else {
			mimeType = "application/octet-stream"
		}
	}
	return mimeType
}

func (o *Options) generateUri() string {
	return path.Join(o.TargetOwner, o.TargetRepo, "raw", o.TargetBranch, o.TargetPath)
}

func (o *Options) timestamp() string {
	return strconv.FormatInt(o.BranchTimestamp.Unix(), 10)
}
