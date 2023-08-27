package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimHostPort(t *testing.T) {
	assert.EqualValues(t, "aa", TrimHostPort("aa"))
	assert.EqualValues(t, "", TrimHostPort(":"))
	assert.EqualValues(t, "example.com", TrimHostPort("example.com:80"))
}

// TestCleanPath is mostly copied from fasthttp, to keep the behaviour we had before migrating away from it.
// Source (MIT licensed): https://github.com/valyala/fasthttp/blob/v1.48.0/uri_test.go#L154
// Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
func TestCleanPath(t *testing.T) {
	// double slash
	testURIPathNormalize(t, "/aa//bb", "/aa/bb")

	// triple slash
	testURIPathNormalize(t, "/x///y/", "/x/y/")

	// multi slashes
	testURIPathNormalize(t, "/abc//de///fg////", "/abc/de/fg/")

	// encoded slashes
	testURIPathNormalize(t, "/xxxx%2fyyy%2f%2F%2F", "/xxxx/yyy/")

	// dotdot
	testURIPathNormalize(t, "/aaa/..", "/")

	// dotdot with trailing slash
	testURIPathNormalize(t, "/xxx/yyy/../", "/xxx/")

	// multi dotdots
	testURIPathNormalize(t, "/aaa/bbb/ccc/../../ddd", "/aaa/ddd")

	// dotdots separated by other data
	testURIPathNormalize(t, "/a/b/../c/d/../e/..", "/a/c/")

	// too many dotdots
	testURIPathNormalize(t, "/aaa/../../../../xxx", "/xxx")
	testURIPathNormalize(t, "/../../../../../..", "/")
	testURIPathNormalize(t, "/../../../../../../", "/")

	// encoded dotdots
	testURIPathNormalize(t, "/aaa%2Fbbb%2F%2E.%2Fxxx", "/aaa/xxx")

	// double slash with dotdots
	testURIPathNormalize(t, "/aaa////..//b", "/b")

	// fake dotdot
	testURIPathNormalize(t, "/aaa/..bbb/ccc/..", "/aaa/..bbb/")

	// single dot
	testURIPathNormalize(t, "/a/./b/././c/./d.html", "/a/b/c/d.html")
	testURIPathNormalize(t, "./foo/", "/foo/")
	testURIPathNormalize(t, "./../.././../../aaa/bbb/../../../././../", "/")
	testURIPathNormalize(t, "./a/./.././../b/./foo.html", "/b/foo.html")
}

func testURIPathNormalize(t *testing.T, requestURI, expectedPath string) {
	cleanedPath := CleanPath(requestURI)
	if cleanedPath != expectedPath {
		t.Fatalf("Unexpected path %q. Expected %q. requestURI=%q", cleanedPath, expectedPath, requestURI)
	}
}
