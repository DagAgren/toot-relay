# toot-relay #

This is a small service, written in Go, that can be set as an endpoint for web
push notifications, and forwards the encrypted payloads it receives to an iOS
app through APNs.

The client needs to implement a user notification service extension that can
decrypt the payloads once they arrive. The original payload is z85-encoded and
stored in the "p" property of the notification.

## Usage ##

Run `go build`, run `./toot-relay`. It will listen on port 8080. Subscribe to web
pushes using the endpoint `http://<your-domain-name>:8080/relay-to/<device-token>".

## Status ##

This is currently basically just a proof of concept. It does not implement the
web push protocol correctly, but it will still work for forwarding pushes.

It also currently doesn't use HTTPS, which probably breaks completely. This will
be fixed very soon.

## License ##

This code is released into the public domain with no warranties. If that is not
suitable, it is also available under the
[CC0 license](http://creativecommons.org/publicdomain/zero/1.0/).

