FROM golang:1.19-buster as build-env
WORKDIR /go/src/webpush-apn-relay

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o webpush-apn-relay

FROM gcr.io/distroless/base
COPY --from=build-env /go/src/webpush-apn-relay/webpush-apn-relay /

ADD cas.crt /cas.crt
ENV CA_FILENAME /cas.crt

EXPOSE 42069

ENTRYPOINT [ "/webpush-apn-relay" ]
