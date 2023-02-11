package gitea

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/cache"
)

const (
	// defaultBranchCacheTimeout specifies the timeout for the default branch cache. It can be quite long.
	defaultBranchCacheTimeout = 15 * time.Minute

	// branchExistenceCacheTimeout specifies the timeout for the branch timestamp & existence cache. It should be shorter
	// than fileCacheTimeout, as that gets invalidated if the branch timestamp has changed. That way, repo changes will be
	// picked up faster, while still allowing the content to be cached longer if nothing changes.
	branchExistenceCacheTimeout = 5 * time.Minute

	// fileCacheTimeout specifies the timeout for the file content cache - you might want to make this quite long, depending
	// on your available memory.
	// TODO: move as option into cache interface
	fileCacheTimeout = 5 * time.Minute

	// fileCacheSizeLimit limits the maximum file size that will be cached, and is set to 1 MB by default.
	fileCacheSizeLimit = int64(1000 * 1000)
)

type FileResponse struct {
	Exists    bool
	IsSymlink bool
	ETag      string
	MimeType  string
	Body      []byte
}

func (f FileResponse) IsEmpty() bool {
	return len(f.Body) != 0
}

func (f FileResponse) createHttpResponse(cacheKey string) (header http.Header, statusCode int) {
	header = make(http.Header)

	if f.Exists {
		statusCode = http.StatusOK
	} else {
		statusCode = http.StatusNotFound
	}

	if f.IsSymlink {
		header.Set(giteaObjectTypeHeader, objTypeSymlink)
	}
	header.Set(ETagHeader, f.ETag)
	header.Set(ContentTypeHeader, f.MimeType)
	header.Set(ContentLengthHeader, fmt.Sprintf("%d", len(f.Body)))
	header.Set(PagesCacheIndicatorHeader, "true")

	log.Trace().Msgf("fileCache for %q used", cacheKey)
	return header, statusCode
}

type BranchTimestamp struct {
	Branch    string
	Timestamp time.Time
	notFound  bool
}

type writeCacheReader struct {
	originalReader io.ReadCloser
	buffer         *bytes.Buffer
	rileResponse   *FileResponse
	cacheKey       string
	cache          cache.SetGetKey
	hasError       bool
}

func (t *writeCacheReader) Read(p []byte) (n int, err error) {
	n, err = t.originalReader.Read(p)
	if err != nil && err != io.EOF {
		log.Trace().Err(err).Msgf("[cache] original reader for %q has returned an error", t.cacheKey)
		t.hasError = true
	} else if n > 0 {
		_, _ = t.buffer.Write(p[:n])
	}
	return
}

func (t *writeCacheReader) Close() error {
	if !t.hasError {
		fc := *t.rileResponse
		fc.Body = t.buffer.Bytes()
		_ = t.cache.Set(t.cacheKey, fc, fileCacheTimeout)
	}
	log.Trace().Msgf("cacheReader for %q saved=%t closed", t.cacheKey, !t.hasError)
	return t.originalReader.Close()
}

func (f FileResponse) CreateCacheReader(r io.ReadCloser, cache cache.SetGetKey, cacheKey string) io.ReadCloser {
	if r == nil || cache == nil || cacheKey == "" {
		log.Error().Msg("could not create CacheReader")
		return nil
	}

	return &writeCacheReader{
		originalReader: r,
		buffer:         bytes.NewBuffer(make([]byte, 0)),
		rileResponse:   &f,
		cache:          cache,
		cacheKey:       cacheKey,
	}
}
