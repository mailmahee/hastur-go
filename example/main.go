package main

import (
	"git.corp.ooyala.com/hastur-go"
	"time"
)

func main() {
	i := 0
	hastur.Every(hastur.FiveSecs, func() {
		i++
		hastur.Counter("foo", i)
	})
	time.Sleep(20 * time.Second)
}
