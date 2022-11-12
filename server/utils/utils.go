package utils

import (
	"strings"
)

func TrimHostPort(host string) string {
	i := strings.IndexByte(host, ':')
	if i >= 0 {
		return host[:i]
	}
	return host
}
