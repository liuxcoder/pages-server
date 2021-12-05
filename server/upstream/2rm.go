package upstream

import "github.com/OrlovEvgeny/go-mcache"

// branchTimestampCache stores branch timestamps for faster cache checking
var branchTimestampCache = mcache.New()

// fileResponseCache stores responses from the Gitea server
// TODO: make this an MRU cache with a size limit
var fileResponseCache = mcache.New()
