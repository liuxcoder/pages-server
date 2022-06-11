//go:build integration
// +build integration

package integration

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestGetRedirect(t *testing.T) {
	log.Printf("== TestGetRedirect ==\n")
	// test custom domain redirect
	resp, err := getTestHTTPSClient().Get("https://calciumdibromid.localhost.mock.directory:4430")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusTemporaryRedirect, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "https://www.cabr2.de/", resp.Header["Location"][0])
	assert.EqualValues(t, 0, getSize(resp.Body))
}

func TestGetContent(t *testing.T) {
	log.Printf("== TestGetContent ==\n")
	// test get image
	resp, err := getTestHTTPSClient().Get("https://magiclike.localhost.mock.directory:4430/images/827679288a.jpg")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "image/jpeg", resp.Header["Content-Type"][0])
	assert.EqualValues(t, "124635", resp.Header["Content-Length"][0])
	assert.EqualValues(t, 124635, getSize(resp.Body))

	// specify branch
	resp, err = getTestHTTPSClient().Get("https://momar.localhost.mock.directory:4430/pag/@master/")
	assert.NoError(t, err)
	if !assert.EqualValues(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header["Content-Type"][0])
	assert.True(t, getSize(resp.Body) > 1000)
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

func getSize(stream io.Reader) int {
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(stream)
	return buf.Len()
}
