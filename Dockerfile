FROM techknowlogick/xgo as build

WORKDIR /workspace

COPY . .
RUN CGO_ENABLED=1 go build -tags 'sqlite sqlite_unlock_notify netgo' -ldflags '-s -w -extldflags "-static" -linkmode external' .

FROM scratch
COPY --from=build /workspace/pages /pages
COPY --from=build \
    /etc/ssl/certs/ca-certificates.crt \
    /etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["/pages"]
