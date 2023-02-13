package certificates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codeberg.org/codeberg/pages/server/database"
)

func TestMockCert(t *testing.T) {
	db := database.NewMockCertDB(t)
	db.Mock.On("Put", mock.Anything, mock.Anything).Return(nil)

	cert, err := mockCert("example.com", "some error msg", "codeberg.page", db)
	assert.NoError(t, err)
	if assert.NotEmpty(t, cert) {
		assert.NotEmpty(t, cert.Certificate)
	}
}
