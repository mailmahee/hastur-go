A Go client for Hastur. This is an API for publishing (json) Hastur messages via local UDP.

## Get the code

    $ go get git.corp.ooyala.com/hastur-go

In your project:

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
