package upstream

import (
	"net/http"
	"time"

	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
)

// setHeader set values to response header
func (o *Options) setHeader(ctx *context.Context, header http.Header) {
	if eTag := header.Get(gitea.ETagHeader); eTag != "" {
		ctx.RespWriter.Header().Set(gitea.ETagHeader, eTag)
	}
	if cacheIndicator := header.Get(gitea.PagesCacheIndicatorHeader); cacheIndicator != "" {
		ctx.RespWriter.Header().Set(gitea.PagesCacheIndicatorHeader, cacheIndicator)
	}
	if length := header.Get(gitea.ContentLengthHeader); length != "" {
		ctx.RespWriter.Header().Set(gitea.ContentLengthHeader, length)
	}
	if mime := header.Get(gitea.ContentTypeHeader); mime == "" || o.ServeRaw {
		ctx.RespWriter.Header().Set(gitea.ContentTypeHeader, rawMime)
	} else {
		ctx.RespWriter.Header().Set(gitea.ContentTypeHeader, mime)
	}
	ctx.RespWriter.Header().Set(headerLastModified, o.BranchTimestamp.In(time.UTC).Format(time.RFC1123))
}
