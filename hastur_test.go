package hastur_test

import (
	"git.corp.ooyala.com/hastur-go"

	"encoding/json"
	"fmt"
	. "launchpad.net/gocheck"
	"log"
	"net"
	"os"
	"reflect"
	"testing"
	"time"
)

// Hook gocheck into gotest
func Test(t *testing.T) { TestingT(t) }

type HasturSuite struct{}

var _ = Suite(&HasturSuite{})

var messages [][]byte
var captureQuit = make(chan bool)

func StartCapture() {
	messages = make([][]byte, 0)
	addr, err := net.ResolveUDPAddr("udp", ":8126")
	if err != nil {
		log.Fatalln(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		for {
			bytes := make([]byte, 1024)
			n, _, err := conn.ReadFrom(bytes)
			if err != nil {
				log.Fatalln(err)
			}
			message := bytes[:n]
			if string(message) == "stop" {
				captureQuit <- true
				conn.Close()
				return
			}
			messages = append(messages, message)
		}
	}()
}

type Message map[string]interface{}

func FinishCapture() []Message {
	conn, err := net.Dial("udp", "localhost:8126")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Fprint(conn, "stop")
	<-captureQuit

	results := make([]Message, 0)
	for _, rawMessage := range messages {
		message := make(Message)
		err := json.Unmarshal(rawMessage, &message)
		if err != nil {
			log.Fatalln(err)
		}
		results = append(results, message)
	}
	return results
}

func (s *HasturSuite) SetUpTest(c *C) {
	// Use a port for testing that doesn't conflict with the agent
	hastur.SetUdpPort(8126)
	hastur.SetAppName("test.app")
	StartCapture()
}

// Helper for the common case that there's a single message.
func getOneMessage(c *C) Message {
	messages := FinishCapture()
	c.Assert(messages, HasLen, 1)
	return messages[0]
}

func getLabels(c *C, m Message) map[string]interface{} {
	rawLabels, ok := m["labels"]
	c.Assert(ok, Equals, true)
	c.Assert(reflect.TypeOf(rawLabels).Kind(), Equals, reflect.Map)
	return rawLabels.(map[string]interface{})
}

func verifyCommonAttributes(c *C, m Message) {
	labels := getLabels(c, m)
	c.Check(labels["app"], Equals, "test.app")
	c.Check(labels["pid"], Equals, float64(os.Getpid())) // json recovers all numbers as float64
}

// Check that the timestamp is present and is set to a recent value (as it should be if the default timestamp
// value of time.Now() was selected).
func verifyCurrentTimestamp(c *C, m Message) {
	rawTimestamp, ok := m["timestamp"]
	c.Assert(ok, Equals, true)
	c.Assert(reflect.TypeOf(rawTimestamp).Kind(), Equals, reflect.Float64)
	unixNanos := int64(rawTimestamp.(float64)) * 1000 // µs -> s
	unixSecs := unixNanos / 1000000000
	timestamp := time.Unix(unixSecs, unixNanos - (unixSecs * 1000000000))
	delta := timestamp.Sub(time.Now()).Seconds()
	inRange := delta < 1 && delta > -1
	c.Check(inRange, Equals, true)
}

func (s *HasturSuite) TestMark(c *C) {
	hastur.MarkFull("test.mark", "baz", time.Now(), map[string]interface{}{"label1": "value1"})
	m := getOneMessage(c)

	verifyCommonAttributes(c, m)
	verifyCurrentTimestamp(c, m)

	c.Check(m["type"], Equals, "mark")
	c.Check(m["name"], Equals, "test.mark")
	c.Check(m["value"], Equals, "baz")
	labels := getLabels(c, m)
	c.Check(labels["label1"], Equals, "value1")
}

func (s *HasturSuite) TestCounter(c *C) {
	hastur.Counter("test.counter", 10)
	m := getOneMessage(c)

	verifyCommonAttributes(c, m)
	verifyCurrentTimestamp(c, m)

	c.Check(m["type"], Equals, "counter")
	c.Check(m["name"], Equals, "test.counter")
	c.Check(m["value"], Equals, 10.0)
}
