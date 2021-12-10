package database

import (
	"github.com/akrylysov/pogreb"
	"github.com/go-acme/lego/v4/certificate"
)

type CertDB interface {
	Close() error
	Put(name string, cert *certificate.Resource) error
	Get(name []byte) (*certificate.Resource, error)
	Delete(key []byte) error
	Compact() (pogreb.CompactionResult, error)
	Items() *pogreb.ItemIterator
}
