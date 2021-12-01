package main

import (
	"bytes"
	"encoding/gob"
	"github.com/akrylysov/pogreb"
)

// GetHSTSHeader returns a HSTS header with includeSubdomains & preload for MainDomainSuffix and RawDomain, or an empty
// string for custom domains.
func GetHSTSHeader(host []byte) string {
	if bytes.HasSuffix(host, MainDomainSuffix) || bytes.Equal(host, RawDomain) {
		return "max-age=63072000; includeSubdomains; preload"
	} else {
		return ""
	}
}

func TrimHostPort(host []byte) []byte {
	i := bytes.IndexByte(host, ':')
	if i >= 0 {
		return host[:i]
	}
	return host
}

func PogrebPut(db *pogreb.DB, name []byte, obj interface{}) {
	var resGob bytes.Buffer
	resEnc := gob.NewEncoder(&resGob)
	err := resEnc.Encode(obj)
	if err != nil {
		panic(err)
	}
	err = db.Put(name, resGob.Bytes())
	if err != nil {
		panic(err)
	}
}

func PogrebGet(db *pogreb.DB, name []byte, obj interface{}) bool {
	resBytes, err := db.Get(name)
	if err != nil {
		panic(err)
	}
	if resBytes == nil {
		return false
	}

	resGob := bytes.NewBuffer(resBytes)
	resDec := gob.NewDecoder(resGob)
	err = resDec.Decode(obj)
	if err != nil {
		panic(err)
	}
	return true
}
