package database

import (
	"fmt"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/rs/zerolog/log"
)

type CertDB interface {
	Close() error
	Put(name string, cert *certificate.Resource) error
	Get(name string) (*certificate.Resource, error)
	Delete(key string) error
	Items(page, pageSize int) ([]*Cert, error)
	// Compact deprecated // TODO: remove in next version
	Compact() (string, error)
}

type Cert struct {
	Domain    string `xorm:"pk      NOT NULL UNIQUE    'domain'"`
	Created   int64  `xorm:"created NOT NULL DEFAULT 0 'created'"`
	Updated   int64  `xorm:"updated NOT NULL DEFAULT 0 'updated'"`
	ValidTill int64  `xorm:"        NOT NULL DEFAULT 0 'valid_till'"`
	// certificate.Resource
	CertURL           string `xorm:"'cert_url'"`
	CertStableURL     string `xorm:"'cert_stable_url'"`
	PrivateKey        []byte `xorm:"'private_key'"`
	Certificate       []byte `xorm:"'certificate'"`
	IssuerCertificate []byte `xorm:"'issuer_certificate'"`
}

func (c Cert) Raw() *certificate.Resource {
	return &certificate.Resource{
		Domain:            c.Domain,
		CertURL:           c.CertURL,
		CertStableURL:     c.CertStableURL,
		PrivateKey:        c.PrivateKey,
		Certificate:       c.Certificate,
		IssuerCertificate: c.IssuerCertificate,
	}
}

func toCert(name string, c *certificate.Resource) (*Cert, error) {
	tlsCertificates, err := certcrypto.ParsePEMBundle(c.Certificate)
	if err != nil {
		return nil, err
	}
	if len(tlsCertificates) == 0 || tlsCertificates[0] == nil {
		err := fmt.Errorf("parsed cert resource has no cert")
		log.Error().Err(err).Str("domain", c.Domain).Msgf("cert: %v", c)
		return nil, err
	}
	validTill := tlsCertificates[0].NotAfter.Unix()

	// handle wildcard certs
	if name[:1] == "." {
		name = "*" + name
	}
	if name != c.Domain {
		err := fmt.Errorf("domain key '%s' and cert domain '%s' not equal", name, c.Domain)
		log.Error().Err(err).Msg("toCert conversion did discover mismatch")
		// TODO: fail hard: return nil, err
	}

	return &Cert{
		Domain:    c.Domain,
		ValidTill: validTill,

		CertURL:           c.CertURL,
		CertStableURL:     c.CertStableURL,
		PrivateKey:        c.PrivateKey,
		Certificate:       c.Certificate,
		IssuerCertificate: c.IssuerCertificate,
	}, nil
}
