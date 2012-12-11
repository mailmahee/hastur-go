A Go client for Hastur. This is an API for publishing (json) Hastur messages via local UDP.

## Get the code

    $ go get github.com/ooyala/hastur-go

(You may also wish to check in the code under `vendor/` or something instead). Then in your project:

``` go
import "github.com/ooyala/hastur-go"
...
hastur.Counter("my.app.loglines", 1)
```

See the [top- and method-level documentation](http://go.pkgdoc.org/github.com/ooyala/hastur-go) for usage
instructions.

## Development

Run the tests with:

    $ go get launchpad.net/gocheck
    $ go test
