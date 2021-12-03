package database

import (
	"fmt"
	"github.com/akrylysov/pogreb"
	"github.com/akrylysov/pogreb/fs"
	"time"
)

func New(path string) (KeyDB, error) {
	if path == "" {
		return nil, fmt.Errorf("path not set")
	}
	return pogreb.Open(path, &pogreb.Options{
		BackgroundSyncInterval:       30 * time.Second,
		BackgroundCompactionInterval: 6 * time.Hour,
		FileSystem:                   fs.OSMMap,
	})
}
