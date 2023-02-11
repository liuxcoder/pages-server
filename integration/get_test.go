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
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if !assert.EqualValues(t, http.StatusTemporaryRedirect, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "https://www.cabr2.de/", resp.Header.Get("Location"))
	assert.EqualValues(t, `<a href="https://www.cabr2.de/">Temporary Redirect</a>.`, strings.TrimSpace(string(getBytes(resp.Body))))
}

func TestGetContent(t *testing.T) {
	log.Println("=== TestGetContent ===")
	// test get image
	resp, err := getTestHTTPSClient().Get("https://cb_pages_tests.localhost.mock.directory:4430/images/827679288a.jpg")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "image/jpeg", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "124635", resp.Header.Get("Content-Length"))
	assert.EqualValues(t, 124635, getSize(resp.Body))
	assert.Len(t, resp.Header.Get("ETag"), 42)

	// specify branch
	resp, err = getTestHTTPSClient().Get("https://cb_pages_tests.localhost.mock.directory:4430/pag/@master/")
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.True(t, getSize(resp.Body) > 1000)
	assert.Len(t, resp.Header.Get("ETag"), 44)

	// access branch name contains '/'
	resp, err = getTestHTTPSClient().Get("https://cb_pages_tests.localhost.mock.directory:4430/blumia/@docs~main/")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.True(t, getSize(resp.Body) > 100)
	assert.Len(t, resp.Header.Get("ETag"), 44)

	// TODO: test get of non cachable content (content size > fileCacheSizeLimit)
}

func TestCustomDomain(t *testing.T) {
	log.Println("=== TestCustomDomain ===")
	resp, err := getTestHTTPSClient().Get("https://mock-pages.codeberg-test.org:4430/README.md")
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, "text/markdown; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "106", resp.Header.Get("Content-Length"))
	assert.EqualValues(t, 106, getSize(resp.Body))
}

func TestCustomDomainRedirects(t *testing.T) {
	log.Println("=== TestCustomDomainRedirects ===")
	// test redirect from default pages domain to custom domain
	resp, err := getTestHTTPSClient().Get("https://6543.localhost.mock.directory:4430/test_pages-server_custom-mock-domain/@main/README.md")
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusTemporaryRedirect, resp.StatusCode)
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	// TODO: custom port is not evaluated (witch does hurt tests & dev env only)
	// assert.EqualValues(t, "https://mock-pages.codeberg-test.org:4430/@main/README.md", resp.Header.Get("Location"))
	assert.EqualValues(t, "https://mock-pages.codeberg-test.org/@main/README.md", resp.Header.Get("Location"))
	assert.EqualValues(t, `https:/codeberg.org/6543/test_pages-server_custom-mock-domain/src/branch/main/README.md; rel="canonical"; rel="canonical"`, resp.Header.Get("Link"))

	// test redirect from an custom domain to the primary custom domain (www.example.com -> example.com)
	// regression test to https://codeberg.org/Codeberg/pages-server/issues/153
	resp, err = getTestHTTPSClient().Get("https://mock-pages-redirect.codeberg-test.org:4430/README.md")
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusTemporaryRedirect, resp.StatusCode)
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	// TODO: custom port is not evaluated (witch does hurt tests & dev env only)
	// assert.EqualValues(t, "https://mock-pages.codeberg-test.org:4430/README.md", resp.Header.Get("Location"))
	assert.EqualValues(t, "https://mock-pages.codeberg-test.org/README.md", resp.Header.Get("Location"))
}

func TestGetNotFound(t *testing.T) {
	log.Println("=== TestGetNotFound ===")
	// test custom not found pages
	resp, err := getTestHTTPSClient().Get("https://cb_pages_tests.localhost.mock.directory:4430/pages-404-demo/blah")
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusNotFound, resp.StatusCode)
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "37", resp.Header.Get("Content-Length"))
	assert.EqualValues(t, 37, getSize(resp.Body))
}

func TestFollowSymlink(t *testing.T) {
	log.Printf("=== TestFollowSymlink ===\n")

	resp, err := getTestHTTPSClient().Get("https://cb_pages_tests.localhost.mock.directory:4430/tests_for_pages-server/@main/link")
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.EqualValues(t, "4", resp.Header.Get("Content-Length"))
	body := getBytes(resp.Body)
	assert.EqualValues(t, 4, len(body))
	assert.EqualValues(t, "abc\n", string(body))
}

func TestLFSSupport(t *testing.T) {
	log.Printf("=== TestLFSSupport ===\n")

	resp, err := getTestHTTPSClient().Get("https://cb_pages_tests.localhost.mock.directory:4430/tests_for_pages-server/@main/lfs.txt")
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusOK, resp.StatusCode)
	body := strings.TrimSpace(string(getBytes(resp.Body)))
	assert.EqualValues(t, 12, len(body))
	assert.EqualValues(t, "actual value", body)
}

func TestGetOptions(t *testing.T) {
	log.Println("=== TestGetOptions ===")
	req, _ := http.NewRequest(http.MethodOptions, "https://mock-pages.codeberg-test.org:4430/README.md", http.NoBody)
	resp, err := getTestHTTPSClient().Do(req)
	assert.NoError(t, err)
	if !assert.NotNil(t, resp) {
		t.FailNow()
	}
	assert.EqualValues(t, http.StatusNoContent, resp.StatusCode)
	assert.EqualValues(t, "GET, HEAD, OPTIONS", resp.Header.Get("Allow"))
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
