package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimHostPort(t *testing.T) {
	assert.EqualValues(t, "aa", TrimHostPort([]byte("aa")))
	assert.EqualValues(t, "", TrimHostPort([]byte(":")))
	assert.EqualValues(t, "example.com", TrimHostPort([]byte("example.com:80")))
}
