package main

import "github.com/valyala/fasthttp"

// getTargetFromDNS searches for CNAME entries on the request domain, optionally with a "www." prefix, and checks if
// the domain is included in the repository's "domains.txt" file. If everything is fine, it returns the target data.
func getTargetFromDNS(ctx *fasthttp.RequestCtx) (targetOwner, targetRepo, targetBranch, targetPath string) {
	// TODO: read CNAME record for host and "www.{host}" to get those values
	// TODO: check domains.txt
	return
}
