package context

import (
	stdContext "context"
	"net/http"

	"codeberg.org/codeberg/pages/server/utils"
)

type Context struct {
	RespWriter http.ResponseWriter
	Req        *http.Request
	StatusCode int
}

func New(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		RespWriter: w,
		Req:        r,
		StatusCode: http.StatusOK,
	}
}

func (c *Context) Context() stdContext.Context {
	if c.Req != nil {
		return c.Req.Context()
	}
	return stdContext.Background()
}

func (c *Context) Response() *http.Response {
	if c.Req != nil && c.Req.Response != nil {
		return c.Req.Response
	}
	return nil
}

func (c *Context) String(raw string, status ...int) {
	code := http.StatusOK
	if len(status) != 0 {
		code = status[0]
	}
	c.RespWriter.WriteHeader(code)
	_, _ = c.RespWriter.Write([]byte(raw))
}

func (c *Context) Redirect(uri string, statusCode int) {
	http.Redirect(c.RespWriter, c.Req, uri, statusCode)
}

// Path returns requested path.
//
// The returned bytes are valid until your request handler returns.
func (c *Context) Path() string {
	return c.Req.URL.Path
}

func (c *Context) Host() string {
	return c.Req.URL.Host
}

func (c *Context) TrimHostPort() string {
	return utils.TrimHostPort(c.Req.Host)
}
