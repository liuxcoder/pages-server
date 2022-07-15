FROM golang:alpine as build

WORKDIR /workspace

RUN apk add ca-certificates
COPY . .
RUN CGO_ENABLED=0 go build .

FROM scratch
COPY --from=build /workspace/pages /pages
COPY --from=build \
    /etc/ssl/certs/ca-certificates.crt \
    /etc/ssl/certs/ca-certificates.crt
    
ENTRYPOINT ["/pages"]