package certificates

import (
	"testing"

	"codeberg.org/codeberg/pages/server/database"
	"github.com/stretchr/testify/assert"
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
