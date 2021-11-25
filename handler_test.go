package main

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"testing"
	"time"
)

func TestHandlerPerformance(t *testing.T) {
	ctx := &fasthttp.RequestCtx{
		Request:  *fasthttp.AcquireRequest(),
		Response: *fasthttp.AcquireResponse(),
	}
	ctx.Request.SetRequestURI("http://mondstern.codeberg.page/")
	fmt.Printf("Start: %v\n", time.Now())
	start := time.Now()
	handler(ctx)
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
	handler(ctx)
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
	handler(ctx)
	end = time.Now()
	fmt.Printf("Done: %v\n", time.Now())
	if ctx.Response.StatusCode() != 200 || len(ctx.Response.Body()) < 1 {
		t.Errorf("request failed with status code %d and body length %d", ctx.Response.StatusCode(), len(ctx.Response.Body()))
	} else {
		t.Logf("request took %d milliseconds", end.Sub(start).Milliseconds())
	}
}
