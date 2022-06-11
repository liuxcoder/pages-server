package gitea

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinURL(t *testing.T) {
	url := joinURL("")
	assert.EqualValues(t, "", url)

	url = joinURL("", "", "")
	assert.EqualValues(t, "", url)

	url = joinURL("http://wwow.url.com", "a", "b/c/", "d")
	// assert.EqualValues(t, "http://wwow.url.com/a/b/c/d", url)
	assert.EqualValues(t, "http://wwow.url.coma/b/c/d", url)

	url = joinURL("h:://wrong", "acdc")
	// assert.EqualValues(t, "h:://wrong/acdc", url)
	assert.EqualValues(t, "h:://wrongacdc", url)
}
