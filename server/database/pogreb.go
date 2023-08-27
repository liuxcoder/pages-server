package database

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/akrylysov/pogreb"
	"github.com/akrylysov/pogreb/fs"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/rs/zerolog/log"
)

var _ CertDB = aDB{}

type aDB struct {
	ctx          context.Context
	cancel       context.CancelFunc
	intern       *pogreb.DB
	syncInterval time.Duration
}

func (p aDB) Close() error {
	p.cancel()
	return p.intern.Sync()
}

func (p aDB) Put(name string, cert *certificate.Resource) error {
	var resGob bytes.Buffer
	if err := gob.NewEncoder(&resGob).Encode(cert); err != nil {
		return err
	}
	return p.intern.Put([]byte(name), resGob.Bytes())
}

func (p aDB) Get(name string) (*certificate.Resource, error) {
	cert := &certificate.Resource{}
	resBytes, err := p.intern.Get([]byte(name))
	if err != nil {
		return nil, err
	}
	if resBytes == nil {
		return nil, nil
	}
	if err := gob.NewDecoder(bytes.NewBuffer(resBytes)).Decode(cert); err != nil {
		return nil, err
	}
	return cert, nil
}

func (p aDB) Delete(key string) error {
	return p.intern.Delete([]byte(key))
}

func (p aDB) Compact() (string, error) {
	result, err := p.intern.Compact()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%+v", result), nil
}

func (p aDB) Items(_, _ int) ([]*Cert, error) {
	items := make([]*Cert, 0, p.intern.Count())
	iterator := p.intern.Items()
	for {
		key, resBytes, err := iterator.Next()
		if err != nil {
			if errors.Is(err, pogreb.ErrIterationDone) {
				break
			}
			return nil, err
		}

		res := &certificate.Resource{}
		if err := gob.NewDecoder(bytes.NewBuffer(resBytes)).Decode(res); err != nil {
			return nil, err
		}

		cert, err := toCert(string(key), res)
		if err != nil {
			return nil, err
		}

		items = append(items, cert)
	}

	return items, nil
}

var _ CertDB = &aDB{}

func (p aDB) sync() {
	for {
		err := p.intern.Sync()
		if err != nil {
			log.Error().Err(err).Msg("Syncing cert database failed")
		}
		select {
		case <-p.ctx.Done():
			return
		case <-time.After(p.syncInterval):
		}
	}
}

func NewPogreb(path string) (CertDB, error) {
	if path == "" {
		return nil, fmt.Errorf("path not set")
	}
	db, err := pogreb.Open(path, &pogreb.Options{
		BackgroundSyncInterval:       30 * time.Second,
		BackgroundCompactionInterval: 6 * time.Hour,
		FileSystem:                   fs.OSMMap,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := &aDB{
		ctx:          ctx,
		cancel:       cancel,
		intern:       db,
		syncInterval: 5 * time.Minute,
	}

	go result.sync()

	return result, nil
}