package utils

import (
	"net/url"
	"path"
	"strings"
)

func TrimHostPort(host string) string {
	i := strings.IndexByte(host, ':')
	if i >= 0 {
		return host[:i]
	}
	return host
}

func CleanPath(uriPath string) string {
	unescapedPath, _ := url.PathUnescape(uriPath)
	cleanedPath := path.Join("/", unescapedPath)

	// If the path refers to a directory, add a trailing slash.
	if !strings.HasSuffix(cleanedPath, "/") && (strings.HasSuffix(unescapedPath, "/") || strings.HasSuffix(unescapedPath, "/.") || strings.HasSuffix(unescapedPath, "/..")) {
		cleanedPath += "/"
	}

	return cleanedPath
}
