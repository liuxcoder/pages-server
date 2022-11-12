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
