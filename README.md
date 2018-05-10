# toot-relay #

This is a small service, written in Go, that can be set as an endpoint for web
push notifications, and forwards the encrypted payloads it receives to an iOS
app through APNs. It is designed to forward notifications from Mastodon to
the iOS client Toot!, but may be of use in other cases too.

The client needs to implement a user notification service extension that can
decrypt the payloads once they arrive. The original payload is z85-encoded and
stored in the "p" property of the notification. Any extra values supplied in the
push endpoint URL are passed in "x".

## Usage ##

Run `go build`, run `./toot-relay`. It will listen on port 42069. Subscribe to web
pushes using the endpoint
`http://<your-domain-name>:42069/relay-to/<device-token>[/extra]`.

You will need a push notification certificate, which should be put in the same
directory, named `toot-relay.p12`.

## Status ##

This is a fairly minimal implementation of only the parts of RFC 8030 that are
required to relay push notifications from Mastodon. It only supports the
straightforward simple POST requests, and not async requests with push receipts.

It does support the various headers, such as `TTL:`, `Urgency:`, and `Topic:`,
which are converted into expiration time, priority (`very-low` and `low` are 5,
`high` and `very-high` are 10), and collapse ID.

The returned `Location:` header is nonsensical, but contains the APNs ID. I did
not read the spec closely enough to see if this address is actually used for
anything, but I do not think it is needed by Mastodon.

The service could probably be made more efficient by queuing up APNs accesses
and not waiting for them to finish before returning from the request handler,
but this has not been implemented at the moment.

## Regarding HTTPS ##

Mastodon, and possibly others, force SSL when connecting to the push endpoint.
The service does have rudimentary support; put files named `toot-relay.crt` and
`toot-relay.key` in the same directory, and those will be loaded and used to
serve HTTPS instead of HTTP.

In practice, it may be easier to use ngnix or another service to handle HTTPS
traffic for you, and forward it to the service as plain HTTP.

## License ##

This code is released into the public domain with no warranties. If that is not
suitable, it is also available under the
[CC0 license](http://creativecommons.org/publicdomain/zero/1.0/).

