package database

import (
	"errors"
	"testing"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

func newTestDB(t *testing.T) *xDB {
	e, err := xorm.NewEngine("sqlite3", ":memory:")
	assert.NoError(t, err)
	assert.NoError(t, e.Sync2(new(Cert)))
	return &xDB{engine: e}
}

func TestSanitizeWildcardCerts(t *testing.T) {
	certDB := newTestDB(t)

	_, err := certDB.Get(".not.found")
	assert.True(t, errors.Is(err, ErrNotFound))

	// TODO: cert key and domain mismatch are don not fail hard jet
	// https://codeberg.org/Codeberg/pages-server/src/commit/d8595cee882e53d7f44f1ddc4ef8a1f7b8f31d8d/server/database/interface.go#L64
	//
	// assert.Error(t, certDB.Put(".wildcard.de", &certificate.Resource{
	// 	Domain:      "*.localhost.mock.directory",
	// 	Certificate: localhost_mock_directory_certificate,
	// }))

	// insert new wildcard cert
	assert.NoError(t, certDB.Put(".wildcard.de", &certificate.Resource{
		Domain:      "*.wildcard.de",
		Certificate: localhost_mock_directory_certificate,
	}))

	// update existing cert
	assert.NoError(t, certDB.Put(".wildcard.de", &certificate.Resource{
		Domain:      "*.wildcard.de",
		Certificate: localhost_mock_directory_certificate,
	}))

	c1, err := certDB.Get(".wildcard.de")
	assert.NoError(t, err)
	c2, err := certDB.Get("*.wildcard.de")
	assert.NoError(t, err)
	assert.EqualValues(t, c1, c2)
}

var localhost_mock_directory_certificate = []byte(`-----BEGIN CERTIFICATE-----
MIIDczCCAlugAwIBAgIIJyBaXHmLk6gwDQYJKoZIhvcNAQELBQAwKDEmMCQGA1UE
AxMdUGViYmxlIEludGVybWVkaWF0ZSBDQSA0OWE0ZmIwHhcNMjMwMjEwMDEwOTA2
WhcNMjgwMjEwMDEwOTA2WjAjMSEwHwYDVQQDExhsb2NhbGhvc3QubW9jay5kaXJl
Y3RvcnkwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDIU/CjzS7t62Gj
neEMqvP7sn99ULT7AEUzEfWL05fWG2z714qcUg1hXkZLgdVDgmsCpplyddip7+2t
ZH/9rLPLMqJphzvOL4CF6jDLbeifETtKyjnt9vUZFnnNWcP3tu8lo8iYSl08qsUI
Pp/hiEriAQzCDjTbR5m9xUPNPYqxzcS4ALzmmCX9Qfc4CuuhMkdv2G4TT7rylWrA
SCSRPnGjeA7pCByfNrO/uXbxmzl3sMO3k5sqgMkx1QIHEN412V8+vtx88mt2sM6k
xjzGZWWKXlRq+oufIKX9KPplhsCjMH6E3VNAzgOPYDqXagtUcGmLWghURltO8Mt2
zwM6OgjjAgMBAAGjgaUwgaIwDgYDVR0PAQH/BAQDAgWgMB0GA1UdJQQWMBQGCCsG
AQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB0GA1UdDgQWBBSMQvlJ1755
sarf8i1KNqj7s5o/aDAfBgNVHSMEGDAWgBTcZcxJMhWdP7MecHCCpNkFURC/YzAj
BgNVHREEHDAaghhsb2NhbGhvc3QubW9jay5kaXJlY3RvcnkwDQYJKoZIhvcNAQEL
BQADggEBACcd7TT28OWwzQN2PcH0aG38JX5Wp2iOS/unDCfWjNAztXHW7nBDMxza
VtyebkJfccexpuVuOsjOX+bww0vtEYIvKX3/GbkhogksBrNkE0sJZtMnZWMR33wa
YxAy/kJBTmLi02r8fX9ZhwjldStHKBav4USuP7DXZjrgX7LFQhR4LIDrPaYqQRZ8
ltC3mM9LDQ9rQyIFP5cSBMO3RUAm4I8JyLoOdb/9G2uxjHr7r6eG1g8DmLYSKBsQ
mWGQDOYgR3cGltDe2yMxM++yHY+b1uhxGOWMrDA1+1k7yI19LL8Ifi2FMovDfu/X
JxYk1NNNtdctwaYJFenmGQvDaIq1KgE=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDUDCCAjigAwIBAgIIKBJ7IIA6W1swDQYJKoZIhvcNAQELBQAwIDEeMBwGA1UE
AxMVUGViYmxlIFJvb3QgQ0EgNTdmZjE2MCAXDTIzMDIwOTA1MzMxMloYDzIwNTMw
MjA5MDUzMzEyWjAoMSYwJAYDVQQDEx1QZWJibGUgSW50ZXJtZWRpYXRlIENBIDQ5
YTRmYjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANOvlqRx8SXQFWo2
gFCiXxls53eENcyr8+meFyjgnS853eEvplaPxoa2MREKd+ZYxM8EMMfj2XGvR3UI
aqR5QyLQ9ihuRqvQo4fG91usBHgH+vDbGPdMX8gDmm9HgnmtOVhSKJU+M2jfE1SW
UuWB9xOa3LMreTXbTNfZEMoXf+GcWZMbx5WPgEga3DvfmV+RsfNvB55eD7YAyZgF
ZnQ3Dskmnxxlkz0EGgd7rqhFHHNB9jARlL22gITADwoWZidlr3ciM9DISymRKQ0c
mRN15fQjNWdtuREgJlpXecbYQMGhdTOmFrqdHkveD1o63rGSC4z+s/APV6xIbcRp
aNpO7L8CAwEAAaOBgzCBgDAOBgNVHQ8BAf8EBAMCAoQwHQYDVR0lBBYwFAYIKwYB
BQUHAwEGCCsGAQUFBwMCMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFNxlzEky
FZ0/sx5wcIKk2QVREL9jMB8GA1UdIwQYMBaAFOqfkm9rebIz4z0SDIKW5edLg5JM
MA0GCSqGSIb3DQEBCwUAA4IBAQBRG9AHEnyj2fKzVDDbQaKHjAF5jh0gwyHoIeRK
FkP9mQNSWxhvPWI0tK/E49LopzmVuzSbDd5kZsaii73rAs6f6Rf9W5veo3AFSEad
stM+Zv0f2vWB38nuvkoCRLXMX+QUeuL65rKxdEpyArBju4L3/PqAZRgMLcrH+ak8
nvw5RdAq+Km/ZWyJgGikK6cfMmh91YALCDFnoWUWrCjkBaBFKrG59ONV9f0IQX07
aNfFXFCF5l466xw9dHjw5iaFib10cpY3iq4kyPYIMs6uaewkCtxWKKjiozM4g4w3
HqwyUyZ52WUJOJ/6G9DJLDtN3fgGR+IAp8BhYd5CqOscnt3h
-----END CERTIFICATE-----`)
