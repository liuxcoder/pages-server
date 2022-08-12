package gitea

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

const (
	giteaAPIRepos         = "/api/v1/repos/"
	giteaObjectTypeHeader = "X-Gitea-Object-Type"
)

var ErrorNotFound = errors.New("not found")

type Client struct {
	giteaRoot      string
	giteaAPIToken  string
	fastClient     *fasthttp.Client
	infoTimeout    time.Duration
	contentTimeout time.Duration

	followSymlinks bool
	supportLFS     bool
}

// TODO: once golang v1.19 is min requirement, we can switch to 'JoinPath()' of 'net/url' package
func joinURL(baseURL string, paths ...string) string {
	p := make([]string, 0, len(paths))
	for i := range paths {
		path := strings.TrimSpace(paths[i])
		path = strings.Trim(path, "/")
		if len(path) != 0 {
			p = append(p, path)
		}
	}

	return baseURL + "/" + strings.Join(p, "/")
}

func NewClient(giteaRoot, giteaAPIToken string, followSymlinks, supportLFS bool) (*Client, error) {
	rootURL, err := url.Parse(giteaRoot)
	giteaRoot = strings.Trim(rootURL.String(), "/")

	return &Client{
		giteaRoot:      giteaRoot,
		giteaAPIToken:  giteaAPIToken,
		infoTimeout:    5 * time.Second,
		contentTimeout: 10 * time.Second,
		fastClient:     getFastHTTPClient(),

		followSymlinks: followSymlinks,
		supportLFS:     supportLFS,
	}, err
}

func (client *Client) GiteaRawContent(targetOwner, targetRepo, ref, resource string) ([]byte, error) {
	resp, err := client.ServeRawContent(targetOwner, targetRepo, ref, resource)
	if err != nil {
		return nil, err
	}
	return resp.Body(), nil
}

func (client *Client) ServeRawContent(targetOwner, targetRepo, ref, resource string) (*fasthttp.Response, error) {
	var apiURL string
	if client.supportLFS {
		apiURL = joinURL(client.giteaRoot, giteaAPIRepos, targetOwner, targetRepo, "media", resource+"?ref="+url.QueryEscape(ref))
	} else {
		apiURL = joinURL(client.giteaRoot, giteaAPIRepos, targetOwner, targetRepo, "raw", resource+"?ref="+url.QueryEscape(ref))
	}
	resp, err := client.do(client.contentTimeout, apiURL)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	switch resp.StatusCode() {
	case fasthttp.StatusOK:
		objType := string(resp.Header.Peek(giteaObjectTypeHeader))
		log.Trace().Msgf("server raw content object: %s", objType)
		if client.followSymlinks && objType == "symlink" {
			// TODO: limit to 1000 chars if we switched to std
			linkDest := strings.TrimSpace(string(resp.Body()))
			log.Debug().Msgf("follow symlink from '%s' to '%s'", resource, linkDest)
			return client.ServeRawContent(targetOwner, targetRepo, ref, linkDest)
		}

		return resp, nil

	case fasthttp.StatusNotFound:
		return nil, ErrorNotFound

	default:
		return nil, fmt.Errorf("unexpected status code '%d'", resp.StatusCode())
	}
}

func (client *Client) GiteaGetRepoBranchTimestamp(repoOwner, repoName, branchName string) (time.Time, error) {
	url := joinURL(client.giteaRoot, giteaAPIRepos, repoOwner, repoName, "branches", branchName)
	res, err := client.do(client.infoTimeout, url)
	if err != nil {
		return time.Time{}, err
	}
	if res.StatusCode() != fasthttp.StatusOK {
		return time.Time{}, fmt.Errorf("unexpected status code '%d'", res.StatusCode())
	}
	return time.Parse(time.RFC3339, fastjson.GetString(res.Body(), "commit", "timestamp"))
}

func (client *Client) GiteaGetRepoDefaultBranch(repoOwner, repoName string) (string, error) {
	url := joinURL(client.giteaRoot, giteaAPIRepos, repoOwner, repoName)
	res, err := client.do(client.infoTimeout, url)
	if err != nil {
		return "", err
	}
	if res.StatusCode() != fasthttp.StatusOK {
		return "", fmt.Errorf("unexpected status code '%d'", res.StatusCode())
	}
	return fastjson.GetString(res.Body(), "default_branch"), nil
}

func (client *Client) do(timeout time.Duration, url string) (*fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()

	req.SetRequestURI(url)
	req.Header.Set(fasthttp.HeaderAuthorization, "token "+client.giteaAPIToken)
	res := fasthttp.AcquireResponse()

	err := client.fastClient.DoTimeout(req, res, timeout)

	return res, err
}
