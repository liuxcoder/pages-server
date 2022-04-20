package upstream

import (
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

const giteaAPIRepos = "/api/v1/repos/"

// TODOs:
// * own client to store token & giteaRoot
// * handle 404 -> page will show 500 atm

func giteaRawContent(targetOwner, targetRepo, ref, giteaRoot, giteaAPIToken, resource string) ([]byte, error) {
	req := fasthttp.AcquireRequest()

	req.SetRequestURI(path.Join(giteaRoot, giteaAPIRepos, targetOwner, targetRepo, "raw", resource+"?ref="+url.QueryEscape(ref)))
	req.Header.Set(fasthttp.HeaderAuthorization, giteaAPIToken)
	res := fasthttp.AcquireResponse()

	if err := getFastHTTPClient(10*time.Second).Do(req, res); err != nil {
		return nil, err
	}
	if res.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("unexpected status code '%d'", res.StatusCode())
	}
	return res.Body(), nil
}

func giteaGetRepoBranchTimestamp(giteaRoot, repoOwner, repoName, branchName, giteaAPIToken string) (time.Time, error) {
	client := getFastHTTPClient(5 * time.Second)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(path.Join(giteaRoot, giteaAPIRepos, repoOwner, repoName, "branches", branchName))
	req.Header.Set(fasthttp.HeaderAuthorization, giteaAPIToken)
	res := fasthttp.AcquireResponse()

	if err := client.Do(req, res); err != nil {
		return time.Time{}, err
	}
	if res.StatusCode() != fasthttp.StatusOK {
		return time.Time{}, fmt.Errorf("unexpected status code '%d'", res.StatusCode())
	}
	return time.Parse(time.RFC3339, fastjson.GetString(res.Body(), "commit", "timestamp"))
}

func giteaGetRepoDefaultBranch(giteaRoot, repoOwner, repoName, giteaAPIToken string) (string, error) {
	client := getFastHTTPClient(5 * time.Second)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(path.Join(giteaRoot, giteaAPIRepos, repoOwner, repoName))
	req.Header.Set(fasthttp.HeaderAuthorization, giteaAPIToken)
	res := fasthttp.AcquireResponse()

	if err := client.Do(req, res); err != nil {
		return "", err
	}
	if res.StatusCode() != fasthttp.StatusOK {
		return "", fmt.Errorf("unexpected status code '%d'", res.StatusCode())
	}
	return fastjson.GetString(res.Body(), "default_branch"), nil
}
