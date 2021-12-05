package database

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/akrylysov/pogreb"
	"github.com/akrylysov/pogreb/fs"
)

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

func (p aDB) Put(key []byte, value []byte) error {
	return p.intern.Put(key, value)
}

func (p aDB) Get(key []byte) ([]byte, error) {
	return p.intern.Get(key)
}

func (p aDB) Delete(key []byte) error {
	return p.intern.Delete(key)
}

func (p aDB) Compact() (pogreb.CompactionResult, error) {
	return p.intern.Compact()
}

func (p aDB) Items() *pogreb.ItemIterator {
	return p.intern.Items()
}

var _ CertDB = &aDB{}

func (p aDB) sync() {
	for {
		err := p.intern.Sync()
		if err != nil {
			log.Err(err).Msg("Syncing cert database failed")
		}
		select {
		case <-p.ctx.Done():
			return
		case <-time.After(p.syncInterval):
		}
	}
}

func (p aDB) compact() {
	for {
		err := p.intern.Sync()
		if err != nil {
			log.Err(err).Msg("Syncing cert database failed")
		}
		select {
		case <-p.ctx.Done():
			return
		case <-time.After(p.syncInterval):
		}
	}
}

func New(path string) (CertDB, error) {
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
