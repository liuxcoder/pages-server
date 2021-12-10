package html

import (
	"bytes"
	"strconv"

	"github.com/valyala/fasthttp"
)

// ReturnErrorPage sets the response status code and writes NotFoundPage to the response body, with "%status" replaced
// with the provided status code.
func ReturnErrorPage(ctx *fasthttp.RequestCtx, code int) {
	ctx.Response.SetStatusCode(code)
	ctx.Response.Header.SetContentType("text/html; charset=utf-8")
	message := fasthttp.StatusMessage(code)
	if code == fasthttp.StatusMisdirectedRequest {
		message += " - domain not specified in <code>.domains</code> file"
	}
	if code == fasthttp.StatusFailedDependency {
		message += " - target repo/branch doesn't exist or is private"
	}
	// TODO: use template engine?
	ctx.Response.SetBody(bytes.ReplaceAll(NotFoundPage, []byte("%status"), []byte(strconv.Itoa(code)+" "+message)))
}
