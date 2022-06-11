package database

import (
	"github.com/akrylysov/pogreb"
	"github.com/go-acme/lego/v4/certificate"
)

type CertDB interface {
	Close() error
	Put(name string, cert *certificate.Resource) error
	Get(name string) (*certificate.Resource, error)
	Delete(key string) error
	Compact() (string, error)
	Items() *pogreb.ItemIterator
}
