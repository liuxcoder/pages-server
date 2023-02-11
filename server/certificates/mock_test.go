package certificates

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/codeberg/pages/server/database"
)

func TestMockCert(t *testing.T) {
	db, err := database.NewTmpDB()
	assert.NoError(t, err)
	cert, err := mockCert("example.com", "some error msg", "codeberg.page", db)
	assert.NoError(t, err)
	if assert.NotEmpty(t, cert) {
		assert.NotEmpty(t, cert.Certificate)
	}
}
