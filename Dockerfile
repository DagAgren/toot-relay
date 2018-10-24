FROM golang:1.11 as build-env
WORKDIR /go/src/toot-relay
COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -ldflags "-s -w" -o toot-relay toot-relay.go

FROM gcr.io/distroless/base
COPY --from=build-env /go/src/toot-relay/toot-relay /
CMD ["/toot-relay"]
