dev:
    #!/usr/bin/env bash
    set -euxo pipefail
    export ACME_API=https://acme.mock.directory
    export ACME_ACCEPT_TERMS=true
    export PAGES_DOMAIN=localhost.mock.directory
    export RAW_DOMAIN=raw.localhost.mock.directory
    export PORT=4430
    go run . --verbose

build:
    CGO_ENABLED=0 go build -ldflags '-s -w' -v -o build/codeberg-pages-server ./

lint: tool-golangci tool-gofumpt
    [ $(gofumpt -extra -l . | wc -l) != 0 ] && { echo 'code not formated'; exit 1; }; \
    golangci-lint run

tool-golangci:
    @hash golangci-lint> /dev/null 2>&1; if [ $? -ne 0 ]; then \
    ggo install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
    fi

tool-gofumpt:
    @hash gofumpt> /dev/null 2>&1; if [ $? -ne 0 ]; then \
    go install mvdan.cc/gofumpt@latest; \
    fi
