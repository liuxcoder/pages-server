package upstream

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/gitea"
)

// GetBranchTimestamp finds the default branch (if branch is "") and save branch and it's last modification time to Options
func (o *Options) GetBranchTimestamp(giteaClient *gitea.Client) (bool, error) {
	log := log.With().Strs("BranchInfo", []string{o.TargetOwner, o.TargetRepo, o.TargetBranch}).Logger()

	if len(o.TargetBranch) == 0 {
		// Get default branch
		defaultBranch, err := giteaClient.GiteaGetRepoDefaultBranch(o.TargetOwner, o.TargetRepo)
		if err != nil {
			log.Err(err).Msg("Could't fetch default branch from repository")
			return false, err
		}
		log.Debug().Msgf("Succesfully fetched default branch %q from Gitea", defaultBranch)
		o.TargetBranch = defaultBranch
	}

	timestamp, err := giteaClient.GiteaGetRepoBranchTimestamp(o.TargetOwner, o.TargetRepo, o.TargetBranch)
	if err != nil {
		if !errors.Is(err, gitea.ErrorNotFound) {
			log.Error().Err(err).Msg("Could not get latest commit's timestamp from branch")
		}
		return false, err
	}

	if timestamp == nil || timestamp.Branch == "" {
		return false, fmt.Errorf("empty response")
	}

	log.Debug().Msgf("Succesfully fetched latest commit's timestamp from branch: %#v", timestamp)
	o.BranchTimestamp = timestamp.Timestamp
	o.TargetBranch = timestamp.Branch
	return true, nil
}

func (o *Options) ContentWebLink(giteaClient *gitea.Client) string {
	return giteaClient.ContentWebLink(o.TargetOwner, o.TargetRepo, o.TargetBranch, o.TargetPath) + "; rel=\"canonical\""
}
