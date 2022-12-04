package html

import (
	"net/http"
	"strings"
	"testing"
)

func TestValidMessage(t *testing.T) {
	testString := "requested blacklisted path"
	statusCode := http.StatusForbidden

	expected := strings.ReplaceAll(
		strings.ReplaceAll(ErrorPage, "%message%", testString),
		"%status%",
		http.StatusText(statusCode))
	actual := generateResponse(testString, statusCode)

	if expected != actual {
		t.Errorf("generated response did not match: expected: '%s', got: '%s'", expected, actual)
	}
}

func TestMessageWithHtml(t *testing.T) {
	testString := `abc<img src=1 onerror=alert("xss");`
	escapedString := "abc&lt;img src=1 onerror=alert(&#34;xss&#34;);"
	statusCode := http.StatusNotFound

	expected := strings.ReplaceAll(
		strings.ReplaceAll(ErrorPage, "%message%", escapedString),
		"%status%",
		http.StatusText(statusCode))
	actual := generateResponse(testString, statusCode)

	if expected != actual {
		t.Errorf("generated response did not match: expected: '%s', got: '%s'", expected, actual)
	}
}
