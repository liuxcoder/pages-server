package certificates

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/rs/zerolog/log"
)

const challengePath = "/.well-known/acme-challenge/"

func setupAcmeConfig(configFile, acmeAPI, acmeMail, acmeEabHmac, acmeEabKID string, acmeAcceptTerms bool) (*lego.Config, error) {
	var myAcmeAccount AcmeAccount
	var myAcmeConfig *lego.Config

	if account, err := os.ReadFile(configFile); err == nil {
		log.Info().Msgf("found existing acme account config file '%s'", configFile)
		if err := json.Unmarshal(account, &myAcmeAccount); err != nil {
			return nil, err
		}
		myAcmeAccount.Key, err = certcrypto.ParsePEMPrivateKey([]byte(myAcmeAccount.KeyPEM))
		if err != nil {
			return nil, err
		}
		myAcmeConfig = lego.NewConfig(&myAcmeAccount)
		myAcmeConfig.CADirURL = acmeAPI
		myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048

		// Validate Config
		_, err := lego.NewClient(myAcmeConfig)
		if err != nil {
			log.Info().Err(err).Msg("config validation failed, you might just delete the config file and let it recreate")
			return nil, fmt.Errorf("acme config validation failed: %w", err)
		}
		return myAcmeConfig, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	log.Info().Msgf("no existing acme account config found, try to create a new one")

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	myAcmeAccount = AcmeAccount{
		Email:  acmeMail,
		Key:    privateKey,
		KeyPEM: string(certcrypto.PEMEncode(privateKey)),
	}
	myAcmeConfig = lego.NewConfig(&myAcmeAccount)
	myAcmeConfig.CADirURL = acmeAPI
	myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048
	tempClient, err := lego.NewClient(myAcmeConfig)
	if err != nil {
		log.Error().Err(err).Msg("Can't create ACME client, continuing with mock certs only")
	} else {
		// accept terms & log in to EAB
		if acmeEabKID == "" || acmeEabHmac == "" {
			reg, err := tempClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: acmeAcceptTerms})
			if err != nil {
				log.Error().Err(err).Msg("Can't register ACME account, continuing with mock certs only")
			} else {
				myAcmeAccount.Registration = reg
			}
		} else {
			reg, err := tempClient.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
				TermsOfServiceAgreed: acmeAcceptTerms,
				Kid:                  acmeEabKID,
				HmacEncoded:          acmeEabHmac,
			})
			if err != nil {
				log.Error().Err(err).Msg("Can't register ACME account, continuing with mock certs only")
			} else {
				myAcmeAccount.Registration = reg
			}
		}

		if myAcmeAccount.Registration != nil {
			acmeAccountJSON, err := json.Marshal(myAcmeAccount)
			if err != nil {
				log.Error().Err(err).Msg("json.Marshalfailed, waiting for manual restart to avoid rate limits")
				select {}
			}
			log.Info().Msgf("new acme account created. write to config file '%s'", configFile)
			err = os.WriteFile(configFile, acmeAccountJSON, 0o600)
			if err != nil {
				log.Error().Err(err).Msg("os.WriteFile failed, waiting for manual restart to avoid rate limits")
				select {}
			}
		}
	}

	return myAcmeConfig, nil
}
