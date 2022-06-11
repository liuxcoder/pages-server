package gitea

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

const giteaAPIRepos = "/api/v1/repos/"

var ErrorNotFound = errors.New("not found")

type Client struct {
	giteaRoot      string
	giteaAPIToken  string
	fastClient     *fasthttp.Client
	infoTimeout    time.Duration
	contentTimeout time.Duration
}

type FileResponse struct {
	Exists   bool
	MimeType string
	Body     []byte
}

func joinURL(giteaRoot string, paths ...string) string { return giteaRoot + path.Join(paths...) }

func (f FileResponse) IsEmpty() bool { return len(f.Body) != 0 }

func NewClient(giteaRoot, giteaAPIToken string) *Client {
	return &Client{
		giteaRoot:      giteaRoot,
		giteaAPIToken:  giteaAPIToken,
		infoTimeout:    5 * time.Second,
		contentTimeout: 10 * time.Second,
		fastClient:     getFastHTTPClient(),
	}
}

func (client *Client) GiteaRawContent(targetOwner, targetRepo, ref, resource string) ([]byte, error) {
	url := joinURL(client.giteaRoot, giteaAPIRepos, targetOwner, targetRepo, "raw", resource+"?ref="+url.QueryEscape(ref))
	res, err := client.do(client.contentTimeout, url)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode() {
	case fasthttp.StatusOK:
		return res.Body(), nil
	case fasthttp.StatusNotFound:
		return nil, ErrorNotFound
	default:
		return nil, fmt.Errorf("unexpected status code '%d'", res.StatusCode())
	}
}

func (client *Client) ServeRawContent(uri string) (*fasthttp.Response, error) {
	url := joinURL(client.giteaRoot, giteaAPIRepos, uri)
	res, err := client.do(client.contentTimeout, url)
	if err != nil {
		return nil, err
	}
	// resp.SetBodyStream(&strings.Reader{}, -1)

	if err != nil {
		return nil, err
	}

	switch res.StatusCode() {
	case fasthttp.StatusOK:
		return res, nil
	case fasthttp.StatusNotFound:
		return nil, ErrorNotFound
	default:
		return nil, fmt.Errorf("unexpected status code '%d'", res.StatusCode())
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
