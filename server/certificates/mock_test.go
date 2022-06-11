package certificates

import (
	"testing"

	"codeberg.org/codeberg/pages/server/database"
	"github.com/stretchr/testify/assert"
)

func TestMockCert(t *testing.T) {
	db, err := database.NewTmpDB()
	assert.NoError(t, err)
	cert := mockCert("example.com", "some error msg", "codeberg.page", db)
	if assert.NotEmpty(t, cert) {
		assert.NotEmpty(t, cert.Certificate)
	}
}
