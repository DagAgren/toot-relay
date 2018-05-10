# toot-relay #

This is a small service, written in Go, that can be set as an endpoint for web
push notifications, and forwards the encrypted payloads it receives to an iOS
app through APNs.

The client needs to implement a user notification service extension that can
decrypt the payloads once they arrive. The original payload is z85-encoded and
stored in the "p" property of the notification.

## Usage ##

Run `go build`, run `./toot-relay`. It will listen on port 42069. Subscribe to web
pushes using the endpoint `http://<your-domain-name>:42069/relay-to/<device-token>`.

You will need a push notification certificate, which should be put in the same
directory, named `toot-relay.p12`.

## Status ##

This is currently basically just a proof of concept. It does not implement the
web push protocol correctly, but it will still work for forwarding pushes.

## Regarding HTTPS ##

Mastodon, and possibly others, force SSL when connecting to the push endpoint.
The service does have rudimentary support; put files named "toot-relay.crt" and
"toot-relay.key" in the same directory, and those will be loaded and used to
serve HTTPS instead of HTTP.

In practice, it may be easier to use ngnix or another service to handle HTTPS
traffic for you, and forward it to the service as plain HTTP.

## License ##

This code is released into the public domain with no warranties. If that is not
suitable, it is also available under the
[CC0 license](http://creativecommons.org/publicdomain/zero/1.0/).

