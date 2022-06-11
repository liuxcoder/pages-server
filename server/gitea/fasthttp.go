package gitea

import (
	"time"

	"github.com/valyala/fasthttp"
)

func getFastHTTPClient() *fasthttp.Client {
	return &fasthttp.Client{
		MaxConnDuration:    60 * time.Second,
		MaxConnWaitTimeout: 1000 * time.Millisecond,
		MaxConnsPerHost:    128 * 16, // TODO: adjust bottlenecks for best performance with Gitea!
	}
}
