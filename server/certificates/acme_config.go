package certificates

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"codeberg.org/codeberg/pages/config"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/rs/zerolog/log"
)

const challengePath = "/.well-known/acme-challenge/"

func setupAcmeConfig(cfg config.ACMEConfig) (*lego.Config, error) {
	var myAcmeAccount AcmeAccount
	var myAcmeConfig *lego.Config

	if cfg.AccountConfigFile == "" {
		return nil, fmt.Errorf("invalid acme config file: '%s'", cfg.AccountConfigFile)
	}

	if account, err := os.ReadFile(cfg.AccountConfigFile); err == nil {
		log.Info().Msgf("found existing acme account config file '%s'", cfg.AccountConfigFile)
		if err := json.Unmarshal(account, &myAcmeAccount); err != nil {
			return nil, err
		}

		myAcmeAccount.Key, err = certcrypto.ParsePEMPrivateKey([]byte(myAcmeAccount.KeyPEM))
		if err != nil {
			return nil, err
		}

		myAcmeConfig = lego.NewConfig(&myAcmeAccount)
		myAcmeConfig.CADirURL = cfg.APIEndpoint
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
		Email:  cfg.Email,
		Key:    privateKey,
		KeyPEM: string(certcrypto.PEMEncode(privateKey)),
	}
	myAcmeConfig = lego.NewConfig(&myAcmeAccount)
	myAcmeConfig.CADirURL = cfg.APIEndpoint
	myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048
	tempClient, err := lego.NewClient(myAcmeConfig)
	if err != nil {
		log.Error().Err(err).Msg("Can't create ACME client, continuing with mock certs only")
	} else {
		// accept terms & log in to EAB
		if cfg.EAB_KID == "" || cfg.EAB_HMAC == "" {
			reg, err := tempClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: cfg.AcceptTerms})
			if err != nil {
				log.Error().Err(err).Msg("Can't register ACME account, continuing with mock certs only")
			} else {
				myAcmeAccount.Registration = reg
			}
		} else {
			reg, err := tempClient.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
				TermsOfServiceAgreed: cfg.AcceptTerms,
				Kid:                  cfg.EAB_KID,
				HmacEncoded:          cfg.EAB_HMAC,
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
			log.Info().Msgf("new acme account created. write to config file '%s'", cfg.AccountConfigFile)
			err = os.WriteFile(cfg.AccountConfigFile, acmeAccountJSON, 0o600)
			if err != nil {
				log.Error().Err(err).Msg("os.WriteFile failed, waiting for manual restart to avoid rate limits")
				select {}
			}
		}
	}

	return myAcmeConfig, nil
}
