FROM golang:1.24 as build-env
WORKDIR /go/src/toot-relay
COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -ldflags "-s -w" -o toot-relay toot-relay.go

FROM gcr.io/distroless/base
COPY --from=build-env /go/src/toot-relay/toot-relay /
ADD cas.crt /cas.crt
ENV CA_FILENAME /cas.crt
CMD ["/toot-relay"]
