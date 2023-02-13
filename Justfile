CGO_FLAGS := '-extldflags "-static" -linkmode external'
TAGS      := 'sqlite sqlite_unlock_notify netgo'

dev:
    #!/usr/bin/env bash
    set -euxo pipefail
    export ACME_API=https://acme.mock.directory
    export ACME_ACCEPT_TERMS=true
    export PAGES_DOMAIN=localhost.mock.directory
    export RAW_DOMAIN=raw.localhost.mock.directory
    export PORT=4430
    export LOG_LEVEL=trace
    go run -tags '{{TAGS}}' .

build:
    CGO_ENABLED=1 go build -tags '{{TAGS}}' -ldflags '-s -w {{CGO_FLAGS}}' -v -o build/codeberg-pages-server ./

build-tag VERSION:
    CGO_ENABLED=1 go build -tags '{{TAGS}}' -ldflags '-s -w -X "codeberg.org/codeberg/pages/server/version.Version={{VERSION}}" {{CGO_FLAGS}}' -v -o build/codeberg-pages-server ./

lint: tool-golangci tool-gofumpt
    golangci-lint run --timeout 5m --build-tags integration
    # TODO: run editorconfig-checker

fmt: tool-gofumpt
    gofumpt -w --extra .

clean:
    go clean ./...
    rm -rf build/ integration/certs.sqlite integration/acme-account.json

tool-golangci:
    @hash golangci-lint> /dev/null 2>&1; if [ $? -ne 0 ]; then \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
    fi

tool-gofumpt:
    @hash gofumpt> /dev/null 2>&1; if [ $? -ne 0 ]; then \
    go install mvdan.cc/gofumpt@latest; \
    fi

test:
    go test -race -cover -tags '{{TAGS}}' codeberg.org/codeberg/pages/server/... codeberg.org/codeberg/pages/html/

test-run TEST:
    go test -race -tags '{{TAGS}}' -run "^{{TEST}}$" codeberg.org/codeberg/pages/server/... codeberg.org/codeberg/pages/html/

integration:
    go test -race -tags 'integration {{TAGS}}' codeberg.org/codeberg/pages/integration/...

integration-run TEST:
    go test -race -tags 'integration {{TAGS}}' -run "^{{TEST}}$" codeberg.org/codeberg/pages/integration/...

docker:
    docker run --rm -it --user $(id -u) -v $(pwd):/work --workdir /work -e HOME=/work codeberg.org/6543/docker-images/golang_just
