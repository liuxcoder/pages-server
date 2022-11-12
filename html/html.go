package html

import _ "embed"

//go:embed 404.html
var NotFoundPage string

//go:embed error.html
var ErrorPage string
