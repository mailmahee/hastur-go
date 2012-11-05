A Go client for Hastur. This is an API for publishing (json) Hastur messages via local UDP.

## Get the code

Right now code on git.corp.ooyala.com isn't easily `go get`-able, so you'll need to clone
`ssh://git@git.corp.ooyala.com/hastur-go` to the correct path (`$GOPATH/src/git.corp.ooyala.com/`).

Then in your project:

``` go
import "git.corp.ooyala.com/hastur-go"
...
hastur.Counter("my.app.loglines", 1)
```

See the top- and method-level documentation for usage instructions.

## Development

Run the tests with:

    $ go get launchpad.net/gocheck
    $ go test
