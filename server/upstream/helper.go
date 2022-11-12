package upstream

import (
	"errors"

	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/gitea"
)

// GetBranchTimestamp finds the default branch (if branch is "") and returns the last modification time of the branch
// (or nil if the branch doesn't exist)
func GetBranchTimestamp(giteaClient *gitea.Client, owner, repo, branch string) *gitea.BranchTimestamp {
	log := log.With().Strs("BranchInfo", []string{owner, repo, branch}).Logger()

	if len(branch) == 0 {
		// Get default branch
		defaultBranch, err := giteaClient.GiteaGetRepoDefaultBranch(owner, repo)
		if err != nil {
			log.Err(err).Msg("Could't fetch default branch from repository")
			return nil
		}
		log.Debug().Msgf("Succesfully fetched default branch %q from Gitea", defaultBranch)
		branch = defaultBranch
	}

	timestamp, err := giteaClient.GiteaGetRepoBranchTimestamp(owner, repo, branch)
	if err != nil {
		if !errors.Is(err, gitea.ErrorNotFound) {
			log.Error().Err(err).Msg("Could not get latest commit's timestamp from branch")
		}
		return nil
	}
	log.Debug().Msgf("Succesfully fetched latest commit's timestamp from branch: %#v", timestamp)
	return timestamp
}
