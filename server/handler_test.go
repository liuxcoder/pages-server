package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/server/cache"
)

func TestHandlerPerformance(t *testing.T) {
	testHandler := Handler(
		[]byte("codeberg.page"),
		[]byte("raw.codeberg.org"),
		"https://codeberg.org",
		"https://docs.codeberg.org/pages/raw-content/",
		"",
		[][]byte{[]byte("/.well-known/acme-challenge/")},
		[][]byte{[]byte("raw.codeberg.org"), []byte("fonts.codeberg.org"), []byte("design.codeberg.org")},
		cache.NewKeyValueCache(),
		cache.NewKeyValueCache(),
		cache.NewKeyValueCache(),
		cache.NewKeyValueCache(),
	)

	ctx := &fasthttp.RequestCtx{
		Request:  *fasthttp.AcquireRequest(),
		Response: *fasthttp.AcquireResponse(),
	}
	ctx.Request.SetRequestURI("http://mondstern.codeberg.page/")
	fmt.Printf("Start: %v\n", time.Now())
	start := time.Now()
	testHandler(ctx)
	end := time.Now()
	fmt.Printf("Done: %v\n", time.Now())
	if ctx.Response.StatusCode() != 200 || len(ctx.Response.Body()) < 2048 {
		t.Errorf("request failed with status code %d and body length %d", ctx.Response.StatusCode(), len(ctx.Response.Body()))
	} else {
		t.Logf("request took %d milliseconds", end.Sub(start).Milliseconds())
	}

	ctx.Response.Reset()
	ctx.Response.ResetBody()
	fmt.Printf("Start: %v\n", time.Now())
	start = time.Now()
	testHandler(ctx)
	end = time.Now()
	fmt.Printf("Done: %v\n", time.Now())
	if ctx.Response.StatusCode() != 200 || len(ctx.Response.Body()) < 2048 {
		t.Errorf("request failed with status code %d and body length %d", ctx.Response.StatusCode(), len(ctx.Response.Body()))
	} else {
		t.Logf("request took %d milliseconds", end.Sub(start).Milliseconds())
	}

	ctx.Response.Reset()
	ctx.Response.ResetBody()
	ctx.Request.SetRequestURI("http://example.momar.xyz/")
	fmt.Printf("Start: %v\n", time.Now())
	start = time.Now()
	testHandler(ctx)
	end = time.Now()
	fmt.Printf("Done: %v\n", time.Now())
	if ctx.Response.StatusCode() != 200 || len(ctx.Response.Body()) < 1 {
		t.Errorf("request failed with status code %d and body length %d", ctx.Response.StatusCode(), len(ctx.Response.Body()))
	} else {
		t.Logf("request took %d milliseconds", end.Sub(start).Milliseconds())
	}
}
