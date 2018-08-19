FROM golang:alpine as base
WORKDIR /go/src/toot-relay
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o toot-relay toot-relay.go

FROM scratch
COPY --from=base /go/src/toot-relay/toot-relay /toot-relay
COPY --from=base /go/src/toot-relay/toot-relay.p12 /toot-relay.p12
CMD ["/toot-relay"]
