package database

import "github.com/akrylysov/pogreb"

type KeyDB interface {
	Sync() error
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	Compact() (pogreb.CompactionResult, error)
	Items() *pogreb.ItemIterator
}
