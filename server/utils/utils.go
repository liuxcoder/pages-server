package utils

import "bytes"

func TrimHostPort(host []byte) []byte {
	i := bytes.IndexByte(host, ':')
	if i >= 0 {
		return host[:i]
	}
	return host
}
