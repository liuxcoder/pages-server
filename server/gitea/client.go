package gitea

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/config"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/version"
)

var ErrorNotFound = errors.New("not found")

const (
	// cache key prefixes
	branchTimestampCacheKeyPrefix = "branchTime"
	defaultBranchCacheKeyPrefix   = "defaultBranch"
	rawContentCacheKeyPrefix      = "rawContent"

	// pages server
	PagesCacheIndicatorHeader = "X-Pages-Cache"
	symlinkReadLimit          = 10000

	// gitea
	giteaObjectTypeHeader = "X-Gitea-Object-Type"
	objTypeSymlink        = "symlink"

	// std
	ETagHeader          = "ETag"
	ContentTypeHeader   = "Content-Type"
	ContentLengthHeader = "Content-Length"
)

type Client struct {
	sdkClient     *gitea.Client
	responseCache cache.ICache

	giteaRoot string

	followSymlinks bool
	supportLFS     bool

	forbiddenMimeTypes map[string]bool
	defaultMimeType    string
}

func NewClient(cfg config.GiteaConfig, respCache cache.ICache) (*Client, error) {
	rootURL, err := url.Parse(cfg.Root)
	if err != nil {
		return nil, err
	}
	giteaRoot := strings.Trim(rootURL.String(), "/")

	stdClient := http.Client{Timeout: 10 * time.Second}

	forbiddenMimeTypes := make(map[string]bool, len(cfg.ForbiddenMimeTypes))
	for _, mimeType := range cfg.ForbiddenMimeTypes {
		forbiddenMimeTypes[mimeType] = true
	}

	defaultMimeType := cfg.DefaultMimeType
	if defaultMimeType == "" {
		defaultMimeType = "application/octet-stream"
	}

	sdk, err := gitea.NewClient(
		giteaRoot,
		gitea.SetHTTPClient(&stdClient),
		gitea.SetToken(cfg.Token),
		gitea.SetUserAgent("pages-server/"+version.Version),
	)

	return &Client{
		sdkClient:     sdk,
		responseCache: respCache,

		giteaRoot: giteaRoot,

		followSymlinks: cfg.FollowSymlinks,
		supportLFS:     cfg.LFSEnabled,

		forbiddenMimeTypes: forbiddenMimeTypes,
		defaultMimeType:    defaultMimeType,
	}, err
}

func (client *Client) ContentWebLink(targetOwner, targetRepo, branch, resource string) string {
	return path.Join(client.giteaRoot, targetOwner, targetRepo, "src/branch", branch, resource)
}

func (client *Client) GiteaRawContent(targetOwner, targetRepo, ref, resource string) ([]byte, error) {
	reader, _, _, err := client.ServeRawContent(targetOwner, targetRepo, ref, resource)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (client *Client) ServeRawContent(targetOwner, targetRepo, ref, resource string) (io.ReadCloser, http.Header, int, error) {
	cacheKey := fmt.Sprintf("%s/%s/%s|%s|%s", rawContentCacheKeyPrefix, targetOwner, targetRepo, ref, resource)
	log := log.With().Str("cache_key", cacheKey).Logger()
	log.Trace().Msg("try file in cache")
	// handle if cache entry exist
	if cache, ok := client.responseCache.Get(cacheKey); ok {
		cache := cache.(FileResponse)
		cachedHeader, cachedStatusCode := cache.createHttpResponse(cacheKey)
		// TODO: check against some timestamp mismatch?!?
		if cache.Exists {
			log.Debug().Msg("[cache] exists")
			if cache.IsSymlink {
				linkDest := string(cache.Body)
				log.Debug().Msgf("[cache] follow symlink from %q to %q", resource, linkDest)
				return client.ServeRawContent(targetOwner, targetRepo, ref, linkDest)
			} else if !cache.IsEmpty() {
				log.Debug().Msgf("[cache] return %d bytes", len(cache.Body))
				return io.NopCloser(bytes.NewReader(cache.Body)), cachedHeader, cachedStatusCode, nil
			} else if cache.IsEmpty() {
				log.Debug().Msg("[cache] is empty")
			}
		}
	}
	log.Trace().Msg("file not in cache")
	// not in cache, open reader via gitea api
	reader, resp, err := client.sdkClient.GetFileReader(targetOwner, targetRepo, ref, resource, client.supportLFS)
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusOK:
			// first handle symlinks
			{
				objType := resp.Header.Get(giteaObjectTypeHeader)
				log.Trace().Msgf("server raw content object %q", objType)
				if client.followSymlinks && objType == objTypeSymlink {
					defer reader.Close()
					// read limited chars for symlink
					linkDestBytes, err := io.ReadAll(io.LimitReader(reader, symlinkReadLimit))
					if err != nil {
						return nil, nil, http.StatusInternalServerError, err
					}
					linkDest := strings.TrimSpace(string(linkDestBytes))

					// handle relative links
					// we first remove the link from the path, and make a relative join (resolve parent paths like "/../" too)
					linkDest = path.Join(path.Dir(resource), linkDest)

					// we store symlink not content to reduce duplicates in cache
					fileResponse := FileResponse{
						Exists:    true,
						IsSymlink: true,
						Body:      []byte(linkDest),
						ETag:      resp.Header.Get(ETagHeader),
					}
					log.Trace().Msgf("file response has %d bytes", len(fileResponse.Body))
					if err := client.responseCache.Set(cacheKey, fileResponse, fileCacheTimeout); err != nil {
						log.Error().Err(err).Msg("[cache] error on cache write")
					}

					log.Debug().Msgf("follow symlink from %q to %q", resource, linkDest)
					return client.ServeRawContent(targetOwner, targetRepo, ref, linkDest)
				}
			}

			// now we are sure it's content so set the MIME type
			mimeType := client.getMimeTypeByExtension(resource)
			resp.Response.Header.Set(ContentTypeHeader, mimeType)

			if !shouldRespBeSavedToCache(resp.Response) {
				return reader, resp.Response.Header, resp.StatusCode, err
			}

			// now we write to cache and respond at the same time
			fileResp := FileResponse{
				Exists:   true,
				ETag:     resp.Header.Get(ETagHeader),
				MimeType: mimeType,
			}
			return fileResp.CreateCacheReader(reader, client.responseCache, cacheKey), resp.Response.Header, resp.StatusCode, nil

		case http.StatusNotFound:
			if err := client.responseCache.Set(cacheKey, FileResponse{
				Exists: false,
				ETag:   resp.Header.Get(ETagHeader),
			}, fileCacheTimeout); err != nil {
				log.Error().Err(err).Msg("[cache] error on cache write")
			}

			return nil, resp.Response.Header, http.StatusNotFound, ErrorNotFound
		default:
			return nil, resp.Response.Header, resp.StatusCode, fmt.Errorf("unexpected status code '%d'", resp.StatusCode)
		}
	}
	return nil, nil, http.StatusInternalServerError, err
}

func (client *Client) GiteaGetRepoBranchTimestamp(repoOwner, repoName, branchName string) (*BranchTimestamp, error) {
	cacheKey := fmt.Sprintf("%s/%s/%s/%s", branchTimestampCacheKeyPrefix, repoOwner, repoName, branchName)

	if stamp, ok := client.responseCache.Get(cacheKey); ok && stamp != nil {
		branchTimeStamp := stamp.(*BranchTimestamp)
		if branchTimeStamp.notFound {
			log.Trace().Msgf("[cache] use branch %q not found", branchName)
			return &BranchTimestamp{}, ErrorNotFound
		}
		log.Trace().Msgf("[cache] use branch %q exist", branchName)
		return branchTimeStamp, nil
	}

	branch, resp, err := client.sdkClient.GetRepoBranch(repoOwner, repoName, branchName)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			log.Trace().Msgf("[cache] set cache branch %q not found", branchName)
			if err := client.responseCache.Set(cacheKey, &BranchTimestamp{Branch: branchName, notFound: true}, branchExistenceCacheTimeout); err != nil {
				log.Error().Err(err).Msg("[cache] error on cache write")
			}
			return &BranchTimestamp{}, ErrorNotFound
		}
		return &BranchTimestamp{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return &BranchTimestamp{}, fmt.Errorf("unexpected status code '%d'", resp.StatusCode)
	}

	stamp := &BranchTimestamp{
		Branch:    branch.Name,
		Timestamp: branch.Commit.Timestamp,
	}

	log.Trace().Msgf("set cache branch [%s] exist", branchName)
	if err := client.responseCache.Set(cacheKey, stamp, branchExistenceCacheTimeout); err != nil {
		log.Error().Err(err).Msg("[cache] error on cache write")
	}
	return stamp, nil
}

func (client *Client) GiteaGetRepoDefaultBranch(repoOwner, repoName string) (string, error) {
	cacheKey := fmt.Sprintf("%s/%s/%s", defaultBranchCacheKeyPrefix, repoOwner, repoName)

	if branch, ok := client.responseCache.Get(cacheKey); ok && branch != nil {
		return branch.(string), nil
	}

	repo, resp, err := client.sdkClient.GetRepo(repoOwner, repoName)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code '%d'", resp.StatusCode)
	}

	branch := repo.DefaultBranch
	if err := client.responseCache.Set(cacheKey, branch, defaultBranchCacheTimeout); err != nil {
		log.Error().Err(err).Msg("[cache] error on cache write")
	}
	return branch, nil
}

func (client *Client) getMimeTypeByExtension(resource string) string {
	mimeType := mime.TypeByExtension(path.Ext(resource))
	mimeTypeSplit := strings.SplitN(mimeType, ";", 2)
	if client.forbiddenMimeTypes[mimeTypeSplit[0]] || mimeType == "" {
		mimeType = client.defaultMimeType
	}
	log.Trace().Msgf("probe mime of %q is %q", resource, mimeType)
	return mimeType
}

func shouldRespBeSavedToCache(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	contentLengthRaw := resp.Header.Get(ContentLengthHeader)
	if contentLengthRaw == "" {
		return false
	}

	contentLength, err := strconv.ParseInt(contentLengthRaw, 10, 64)
	if err != nil {
		log.Error().Err(err).Msg("could not parse content length")
	}

	// if content to big or could not be determined we not cache it
	return contentLength > 0 && contentLength < fileCacheSizeLimit
}
