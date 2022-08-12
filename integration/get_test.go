//go:build integration
// +build integration

package integration

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRedirect(t *testing.T) {
	log.Println("=== TestGetRedirect ===")
	// test custom domain redirect
	resp, err := getTestHTTPSClient().Get("https://calciumdibromid.localhost.mock.directory:4430")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusTemporaryRedirect, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "https://www.cabr2.de/", resp.Header.Get("Location"))
	assert.EqualValues(t, 0, getSize(resp.Body))
}

func TestGetContent(t *testing.T) {
	log.Println("=== TestGetContent ===")
	// test get image
	resp, err := getTestHTTPSClient().Get("https://magiclike.localhost.mock.directory:4430/images/827679288a.jpg")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "image/jpeg", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "124635", resp.Header.Get("Content-Length"))
	assert.EqualValues(t, 124635, getSize(resp.Body))
	assert.Len(t, resp.Header.Get("ETag"), 42)

	// specify branch
	resp, err = getTestHTTPSClient().Get("https://momar.localhost.mock.directory:4430/pag/@master/")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.True(t, getSize(resp.Body) > 1000)
	assert.Len(t, resp.Header.Get("ETag"), 42)

	// access branch name contains '/'
	resp, err = getTestHTTPSClient().Get("https://blumia.localhost.mock.directory:4430/pages-server-integration-tests/@docs~main/")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.True(t, getSize(resp.Body) > 100)
	assert.Len(t, resp.Header.Get("ETag"), 42)

	// TODO: test get of non cachable content (content size > fileCacheSizeLimit)
}

func TestCustomDomain(t *testing.T) {
	log.Println("=== TestCustomDomain ===")
	resp, err := getTestHTTPSClient().Get("https://mock-pages.codeberg-test.org:4430/README.md")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "text/markdown; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "106", resp.Header.Get("Content-Length"))
	assert.EqualValues(t, 106, getSize(resp.Body))
}

func TestGetNotFound(t *testing.T) {
	log.Println("=== TestGetNotFound ===")
	// test custom not found pages
	resp, err := getTestHTTPSClient().Get("https://crystal.localhost.mock.directory:4430/pages-404-demo/blah")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusNotFound, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "37", resp.Header.Get("Content-Length"))
	assert.EqualValues(t, 37, getSize(resp.Body))
}

func TestFollowSymlink(t *testing.T) {
	log.Printf("=== TestFollowSymlink ===\n")

	resp, err := getTestHTTPSClient().Get("https://6543.localhost.mock.directory:4430/tests_for_pages-server/@main/link")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "4", resp.Header.Get("Content-Length"))
	body := getBytes(resp.Body)
	assert.EqualValues(t, 4, len(body))
	assert.EqualValues(t, "abc\n", string(body))
}

func TestLFSSupport(t *testing.T) {
	log.Printf("=== TestLFSSupport ===\n")

	resp, err := getTestHTTPSClient().Get("https://6543.localhost.mock.directory:4430/tests_for_pages-server/@main/lfs.txt")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	body := strings.TrimSpace(string(getBytes(resp.Body)))
	assert.EqualValues(t, 12, len(body))
	assert.EqualValues(t, "actual value", body)
}

func getTestHTTPSClient() *http.Client {
	cookieJar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: cookieJar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func getBytes(stream io.Reader) []byte {
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(stream)
	return buf.Bytes()
}

func getSize(stream io.Reader) int {
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(stream)
	return buf.Len()
}
