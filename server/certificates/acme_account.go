package certificates

import (
	"crypto"

	"github.com/go-acme/lego/v4/registration"
)

type AcmeAccount struct {
	Email        string
	Registration *registration.Resource
	Key          crypto.PrivateKey `json:"-"`
	KeyPEM       string            `json:"Key"`
}

// make sure AcmeAccount match User interface
var _ registration.User = &AcmeAccount{}

func (u *AcmeAccount) GetEmail() string {
	return u.Email
}
func (u AcmeAccount) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *AcmeAccount) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}
