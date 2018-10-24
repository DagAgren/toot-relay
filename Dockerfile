FROM golang:alpine as base
WORKDIR /go/src/toot-relay
COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -ldflags "-s -w" -o toot-relay toot-relay.go

FROM scratch
COPY --from=base /go/src/toot-relay/toot-relay /go/src/toot-relay/toot-relay.p12* /
CMD ["/toot-relay"]
