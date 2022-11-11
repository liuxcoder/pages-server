dev:
    #!/usr/bin/env bash
    set -euxo pipefail
    export ACME_API=https://acme.mock.directory
    export ACME_ACCEPT_TERMS=true
    export PAGES_DOMAIN=localhost.mock.directory
    export RAW_DOMAIN=raw.localhost.mock.directory
    export PORT=4430
    export LOG_LEVEL=trace
    go run .

build:
    CGO_ENABLED=0 go build -ldflags '-s -w' -v -o build/codeberg-pages-server ./

build-tag VERSION:
    CGO_ENABLED=0 go build -ldflags '-s -w -X "codeberg.org/codeberg/pages/server/version.Version={{VERSION}}"' -v -o build/codeberg-pages-server ./

lint: tool-golangci tool-gofumpt
    [ $(gofumpt -extra -l . | wc -l) != 0 ] && { echo 'code not formated'; exit 1; }; \
    golangci-lint run --timeout 5m --build-tags integration
    # TODO: run editorconfig-checker

fmt: tool-gofumpt
    gofumpt -w --extra .

clean:
    go clean ./...
    rm -rf build/

tool-golangci:
    @hash golangci-lint> /dev/null 2>&1; if [ $? -ne 0 ]; then \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
    fi

tool-gofumpt:
    @hash gofumpt> /dev/null 2>&1; if [ $? -ne 0 ]; then \
    go install mvdan.cc/gofumpt@latest; \
    fi

test:
    go test -race codeberg.org/codeberg/pages/server/...

test-run TEST:
    go test -race -run "^{{TEST}}$" codeberg.org/codeberg/pages/server/...

integration:
    go test -race -tags integration codeberg.org/codeberg/pages/integration/...

integration-run TEST:
    go test -race -tags integration -run "^{{TEST}}$" codeberg.org/codeberg/pages/integration/...
