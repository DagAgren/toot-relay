FROM golang:1.11 as build-env
WORKDIR /go/src/toot-relay
COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -ldflags "-s -w" -o toot-relay toot-relay.go

FROM debian:latest as cert-env
RUN DEBIAN_FRONTEND=noninteractive apt-get update && apt-get install -y --no-install-recommends ca-certificates
ADD AAACertificateServices.crt /user/local/share/ca-certificates/AAACertificateServices.crt
ADD GeoTrust.crt /user/local/share/ca-certificates/GeoTrust.crt
RUN update-ca-certificates

FROM gcr.io/distroless/base
COPY --from=build-env /go/src/toot-relay/toot-relay /
COPY --from=cert-env /etc/ssl/certs /etc/ssl/certs
CMD ["/toot-relay"]
