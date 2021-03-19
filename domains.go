package main

import "github.com/valyala/fasthttp"

// getTargetFromDNS searches for CNAME entries on the request domain, optionally with a "www." prefix, and checks if
// the domain is included in the repository's "domains.txt" file. If everything is fine, it returns the target data.
// TODO: use TXT records with A/AAAA/ALIAS
func getTargetFromDNS(ctx *fasthttp.RequestCtx) (targetOwner, targetRepo, targetBranch, targetPath string) {
	// TODO: read CNAME record for host and "www.{host}" to get those values
	// TODO: check domains.txt
	return
}

// TODO: cache domains.txt for 15 minutes
// TODO: canonical domains - redirect to first domain if domains.txt exists, also make sure owner.codeberg.page/pages/... redirects to /...
