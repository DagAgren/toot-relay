FROM golang:1.22-bookworm as build-env
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

ARG GIT_REPOSITORY_URL
ARG GIT_COMMIT_SHA
ARG VERSION
ENV DD_GIT_REPOSITORY_URL=${GIT_REPOSITORY_URL}
ENV DD_GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
ENV DD_VERSION=${VERSION}

EXPOSE 42069

ENTRYPOINT [ "/webpush-apn-relay" ]
