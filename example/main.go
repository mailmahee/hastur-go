package main

import (
	"git.corp.ooyala.com/hastur-go"
	"time"
)

func main() {
	badLabel := make(map[string]interface{})
	badLabel["foo"] = make(chan bool)
	hastur.CounterFull("foo", 1, time.Now(), badLabel)
}
