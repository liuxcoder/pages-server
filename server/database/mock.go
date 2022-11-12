package database

import (
	"fmt"
	"time"

	"github.com/OrlovEvgeny/go-mcache"
	"github.com/akrylysov/pogreb"
	"github.com/go-acme/lego/v4/certificate"
)

var _ CertDB = tmpDB{}

type tmpDB struct {
	intern *mcache.CacheDriver
	ttl    time.Duration
}

func (p tmpDB) Close() error {
	_ = p.intern.Close()
	return nil
}

func (p tmpDB) Put(name string, cert *certificate.Resource) error {
	return p.intern.Set(name, cert, p.ttl)
}

func (p tmpDB) Get(name string) (*certificate.Resource, error) {
	cert, has := p.intern.Get(name)
	if !has {
		return nil, fmt.Errorf("cert for %q not found", name)
	}
	return cert.(*certificate.Resource), nil
}

func (p tmpDB) Delete(key string) error {
	p.intern.Remove(key)
	return nil
}

func (p tmpDB) Compact() (string, error) {
	p.intern.Truncate()
	return "Truncate done", nil
}

func (p tmpDB) Items() *pogreb.ItemIterator {
	panic("ItemIterator not implemented for tmpDB")
}

func NewTmpDB() (CertDB, error) {
	return &tmpDB{
		intern: mcache.New(),
		ttl:    time.Minute,
	}, nil
}
