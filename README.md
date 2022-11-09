# toot-relay #

This is a small service, written in Go, that can be set as an endpoint for web
push notifications, and forwards the encrypted payloads it receives to an iOS
app through APNs. It is designed to forward notifications from Mastodon to
the iOS client Toot!, but may be of use in other cases too.

## Usage ##

Run `go build`, run `./toot-relay`. It will listen on port 42069. Subscribe to web
pushes using the endpoint
`http://<your-domain-name>:42069/relay-to/<environment>/<device-token>[/extra]`,
where `<environment>` is either `development` or `production`, `<device-token>`
is the hex encoded device token for the device to push to, and `extra` is any
extra information you want relayed back to your client.

You will need a push notification certificate, which should be put in the same
directory, named `toot-relay.p12`. With a production certificate, both pushing
to production and development environments works. With a development certificate,
only development will work.

## Docker ##

A simple Dockerfile is included for running the service containerised. It has been
tested with the following hosting solutions:

* [Zeit Now](https://zeit.co/now) - There is also a configuration file (`now.json`)
  for using this service to host it. It requites adding the p12 file as a base 64
  encoded secret: `now secrets add p12-base64 "$(cat toot-relay.p12 | base64)`
* [Heroku](https://heroku.com/) - Add a configuration var named `P12_BASE64`
  containing the base 64 encoded p12 file.

## Status ##

This is a fairly minimal implementation of only the parts of RFC 8030 that are
required to relay push notifications from Mastodon. It only supports the simple
POST requests, and not async requests with receipts.

It does support the various headers, such as `TTL:`, `Urgency:`, and `Topic:`,
which are converted into expiration time, priority (`very-low` and `low` are 5,
`high` and `very-high` are 10), and collapse ID.

The returned `Location:` header is nonsensical, but contains the APNs ID. I did
not read the spec closely enough to see if this address is actually used for
anything, but I do not think it is needed by Mastodon.

Currently only `Content-Encoding: aesgcm` is supported. `aes128gcm` is trivial
to support in this service, as it just needs to ignore the extra headers
(`Encryption:` and `Crypto-Key:`) used by `aesgcm`, but my client-side code does
not support it and this it is rejected. If your client-side code can handle it,
uncomment the line referring to it.

## Configuration ##

### Runtime Flags

- `-max-queue-size`: sets the size of the channel that buffers incoming notifications. Raising this value will consume more RAM (Go will pre-allocate the entire buffer to hold `N * size of message`). Defaults to `4096`.
- `-max-workers`: sets the number of parallel goroutines that will send notifications to Apple. Raising it will drain the queue more quickly at a small cost to increased resource usage. Each goroutine uses gets approx. 2kb of memory. This can safely scale into the thousands, if needed. See [this post](https://tpaschalis.github.io/goroutines-size/) for scaling considerations. Defaults to `8`.

### Environment Variables

The service will read a few environment variables that let you make some adjustments.

* `P12_FILENAME`: The name of the p12 file to use for the push notification certificate.
  Defaults to `toot-relay.p12`.
* `P12_BASE64`: Alternative, you can include the base64-encoded data for the entire p12
  file in this variable. This is useful for hosting services that let you set
  environment variables for secret values.
* `P12_PASSWORD`: The password for the p12 file or base64 encoded data. Defaults to no
  password.
* `PORT`: The port to listen on. Defaults to `42069`.
* `CRT_FILENAME`: The crt file to use for TLS connections. Defaults to `toot-relay.crt`.
* `KEY_FILENAME`: The key file to use for TLS connections. Defaults to `toot-relay.key`.
* `CA_FILENAME`: A file containing PEM encoded certificates that will override the system
  root CAs when connecting to the Apple Notification Service API if set. Default: unset.

## Receiving ##

The client needs to implement a user notification service extension that can
decrypt the payloads once they arrive. The original payload is transmitted in the
`p` property of the notification. The server's public key is transmitted in `k`,
the cryptographic salt in `s`, and any extra value supplied in the push endpoint
URL (the `extra` part as shown in the Usage section above) is passed in `x`.

### Example ###

An [excerpt of the Toot! code base](iOS/) for receiving and decrypting messages
is available. You can use this as a basis for your own implementation, or
read on for more technical details of how to do it yourself.

### Encoding ###

The fields `p`, `s` and `k` are transmitted using an extended variant of z85
encoding. This encoding is the same as [ZeroMQ's z85 encoding][z85], but extended
to support messages of any length, not just multiples of four bytes. It follows the
spec suggested in [this message][z85ext], storing the last 1-3 bytes as 2-4 characters,
representing an 8, 16 or 24-bit integer similarly to how normal z85 encoding represents
32-bit integers.

[z85]: https://rfc.zeromq.org/spec:32/Z85/
[z85ext]: http://grokbase.com/t/zeromq/zeromq-dev/144nd380c4/rfc-32-z85-requiring-frames-to-be-multiples-of-4-or-5-bytes

## Regarding HTTPS ##

Mastodon, and possibly others, force SSL when connecting to the push endpoint.
The service does have rudimentary support; put files named `toot-relay.crt` and
`toot-relay.key` in the same directory, and those will be loaded and used to
serve HTTPS instead of HTTP. (Also see the "Configuration" section.)

In practice, it may be easier to use ngnix or another service to handle HTTPS
traffic for you, and forward it to the service as plain HTTP.

## License ##

This code is released into the public domain with no warranties. If that is not
suitable, it is also available under the
[CC0 license](http://creativecommons.org/publicdomain/zero/1.0/).

