package html

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizerSimpleString(t *testing.T) {
	str := "simple text message without any html elements"

	assert.Equal(t, str, sanitizer.Sanitize(str))
}

func TestSanitizerStringWithCodeTag(t *testing.T) {
	str := "simple text message with <code>html</code> tag"

	assert.Equal(t, str, sanitizer.Sanitize(str))
}

func TestSanitizerStringWithCodeTagWithAttribute(t *testing.T) {
	str := "simple text message with <code id=\"code\">html</code> tag"
	expected := "simple text message with <code>html</code> tag"

	assert.Equal(t, expected, sanitizer.Sanitize(str))
}

func TestSanitizerStringWithATag(t *testing.T) {
	str := "simple text message with <a>a link to another page</a>"
	expected := "simple text message with a link to another page"

	assert.Equal(t, expected, sanitizer.Sanitize(str))
}

func TestSanitizerStringWithATagAndHref(t *testing.T) {
	str := "simple text message with <a href=\"http://evil.site\">a link to another page</a>"
	expected := "simple text message with a link to another page"

	assert.Equal(t, expected, sanitizer.Sanitize(str))
}

func TestSanitizerStringWithImgTag(t *testing.T) {
	str := "simple text message with a <img alt=\"not found\" src=\"http://evil.site\">"
	expected := "simple text message with a "

	assert.Equal(t, expected, sanitizer.Sanitize(str))
}

func TestSanitizerStringWithImgTagAndOnerrorAttribute(t *testing.T) {
	str := "simple text message with a <img alt=\"not found\" src=\"http://evil.site\" onerror=\"alert(secret)\">"
	expected := "simple text message with a "

	assert.Equal(t, expected, sanitizer.Sanitize(str))
}
