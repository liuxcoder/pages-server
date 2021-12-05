package certificates

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"

	"codeberg.org/codeberg/pages/server/database"
)

func mockCert(domain, msg, mainDomainSuffix string, keyDatabase database.CertDB) tls.Certificate {
	key, err := certcrypto.GeneratePrivateKey(certcrypto.RSA2048)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   domain,
			Organization: []string{"Codeberg Pages Error Certificate (couldn't obtain ACME certificate)"},
			OrganizationalUnit: []string{
				"Will not try again for 6 hours to avoid hitting rate limits for your domain.",
				"Check https://docs.codeberg.org/codeberg-pages/troubleshooting/ for troubleshooting tips, and feel " +
					"free to create an issue at https://codeberg.org/Codeberg/pages-server if you can't solve it.\n",
				"Error message: " + msg,
			},
		},

		// certificates younger than 7 days are renewed, so this enforces the cert to not be renewed for a 6 hours
		NotAfter:  time.Now().Add(time.Hour*24*7 + time.Hour*6),
		NotBefore: time.Now(),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.(*rsa.PrivateKey).PublicKey,
		key,
	)
	if err != nil {
		panic(err)
	}

	out := &bytes.Buffer{}
	err = pem.Encode(out, &pem.Block{
		Bytes: certBytes,
		Type:  "CERTIFICATE",
	})
	if err != nil {
		panic(err)
	}
	outBytes := out.Bytes()
	res := &certificate.Resource{
		PrivateKey:        certcrypto.PEMEncode(key),
		Certificate:       outBytes,
		IssuerCertificate: outBytes,
		Domain:            domain,
	}
	databaseName := domain
	if domain == "*"+mainDomainSuffix || domain == mainDomainSuffix[1:] {
		databaseName = mainDomainSuffix
	}
	database.PogrebPut(keyDatabase, []byte(databaseName), res)

	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		panic(err)
	}
	return tlsCertificate
}
