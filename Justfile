dev:
    #!/usr/bin/env bash
    set -euxo pipefail
    export ACME_API=https://acme.mock.directory
    export ACME_ACCEPT_TERMS=true
    export PAGES_DOMAIN=localhost.mock.directory
    export RAW_DOMAIN=raw.localhost.mock.directory
    export PORT=4430
    go run .

build:
    CGO_ENABLED=0 go build -ldflags '-s -w' -v -o build/codeberg-pages-server ./