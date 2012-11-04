package main

import (
	"git.corp.ooyala.com/hastur-go"
	"time"
)

func main() {
	hastur.SendProcessHeartbeat = false
	hastur.Start()
	time.Sleep(20 * time.Second)
}
