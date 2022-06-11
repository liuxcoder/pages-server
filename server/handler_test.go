package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/gitea"
)

func TestHandlerPerformance(t *testing.T) {
	giteaRoot := "https://codeberg.org"
	giteaClient := gitea.NewClient(giteaRoot, "")
	testHandler := Handler(
		[]byte("codeberg.page"), []byte("raw.codeberg.org"),
		giteaClient,
		giteaRoot, "https://docs.codeberg.org/pages/raw-content/",
		[][]byte{[]byte("/.well-known/acme-challenge/")},
		[][]byte{[]byte("raw.codeberg.org"), []byte("fonts.codeberg.org"), []byte("design.codeberg.org")},
		cache.NewKeyValueCache(),
		cache.NewKeyValueCache(),
		cache.NewKeyValueCache(),
		cache.NewKeyValueCache(),
	)

	testCase := func(uri string, status int) {
		ctx := &fasthttp.RequestCtx{
			Request:  *fasthttp.AcquireRequest(),
			Response: *fasthttp.AcquireResponse(),
		}
		ctx.Request.SetRequestURI(uri)
		fmt.Printf("Start: %v\n", time.Now())
		start := time.Now()
		testHandler(ctx)
		end := time.Now()
		fmt.Printf("Done: %v\n", time.Now())
		if ctx.Response.StatusCode() != status {
			t.Errorf("request failed with status code %d", ctx.Response.StatusCode())
		} else {
			t.Logf("request took %d milliseconds", end.Sub(start).Milliseconds())
		}
	}

	testCase("https://mondstern.codeberg.page/", 424) // TODO: expect 200
	testCase("https://mondstern.codeberg.page/", 424) // TODO: expect 200
	testCase("https://example.momar.xyz/", 424)       // TODO: expect 200
	testCase("https://codeberg.page/", 424)           // TODO: expect 200
}
