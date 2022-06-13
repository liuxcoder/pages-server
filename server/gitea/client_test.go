package gitea

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinURL(t *testing.T) {
	baseURL := ""
	assert.EqualValues(t, "/", joinURL(baseURL))
	assert.EqualValues(t, "/", joinURL(baseURL, "", ""))

	baseURL = "http://wwow.url.com"
	assert.EqualValues(t, "http://wwow.url.com/a/b/c/d", joinURL(baseURL, "a", "b/c/", "d"))

	baseURL = "http://wow.url.com/subpath/2"
	assert.EqualValues(t, "http://wow.url.com/subpath/2/content.pdf", joinURL(baseURL, "/content.pdf"))
	assert.EqualValues(t, "http://wow.url.com/subpath/2/wonderful.jpg", joinURL(baseURL, "wonderful.jpg"))
	assert.EqualValues(t, "http://wow.url.com/subpath/2/raw/wonderful.jpg?ref=main", joinURL(baseURL, "raw", "wonderful.jpg"+"?ref="+url.QueryEscape("main")))
	assert.EqualValues(t, "http://wow.url.com/subpath/2/raw/wonderful.jpg%3Fref=main", joinURL(baseURL, "raw", "wonderful.jpg%3Fref=main"))
}
