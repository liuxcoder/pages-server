package database

import (
	"bytes"
	"encoding/gob"
)

func PogrebPut(db KeyDB, name []byte, obj interface{}) {
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

func PogrebGet(db KeyDB, name []byte, obj interface{}) bool {
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
