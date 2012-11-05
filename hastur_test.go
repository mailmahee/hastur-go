package hastur_test

import (
	"git.corp.ooyala.com/hastur-go"

	"encoding/json"
	"fmt"
	. "launchpad.net/gocheck"
	"log"
	"net"
	"os"
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

func GetLabels(c *C, m Message) map[string]interface{} {
	rawLabels, ok := m["labels"]
	c.Assert(ok, Equals, true)
	labels, ok := rawLabels.(map[string]interface{})
	c.Assert(ok, Equals, true)
	return labels
}

func VerifyCommonAttributes(c *C, m Message) {
	labels := GetLabels(c, m)
	c.Check(labels["app"], Equals, "test.app")
	c.Check(labels["pid"], Equals, float64(os.Getpid())) // json recovers all numbers as float64
}

// Check that the timestamp is present and is set to a recent value (as it should be if the default timestamp
// value of time.Now() was selected).
func VerifyCurrentTimestamp(c *C, m Message) {
	rawTimestamp, ok := m["timestamp"]
	c.Assert(ok, Equals, true)
	floatTimestamp, ok := rawTimestamp.(float64)
	c.Assert(ok, Equals, true)
	unixNanos := int64(floatTimestamp) * 1000 // µs -> s
	unixSecs := unixNanos / 1000000000
	timestamp := time.Unix(unixSecs, unixNanos-(unixSecs*1000000000))
	delta := timestamp.Sub(time.Now()).Seconds()
	inRange := delta < 1 && delta > -1
	c.Check(inRange, Equals, true)
}

// Test each supported Hastur message type

// Helper for the common case that there's a single message sent with a timestamp of time.Now()
func GetAndVerifySingleMessage(c *C) Message {
	messages := FinishCapture()
	c.Assert(messages, HasLen, 1)
	message := messages[0]
	VerifyCommonAttributes(c, message)
	VerifyCurrentTimestamp(c, message)
	return message
}

func (s *HasturSuite) TestMark(c *C) {
	hastur.MarkFull("test.mark", "baz", time.Now(), map[string]interface{}{"label1": "value1"})
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "mark")
	c.Check(m["name"], Equals, "test.mark")
	c.Check(m["value"], Equals, "baz")
	labels := GetLabels(c, m)
	c.Check(labels["label1"], Equals, "value1")
}

func (s *HasturSuite) TestCounter(c *C) {
	hastur.Counter("test.counter", 10)
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "counter")
	c.Check(m["name"], Equals, "test.counter")
	c.Check(m["value"], Equals, 10.0)
}

func (s *HasturSuite) TestGauge(c *C) {
	hastur.Gauge("test.gauge", 1.234)
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "gauge")
	c.Check(m["name"], Equals, "test.gauge")
	c.Check(m["value"], Equals, 1.234)
}

func (s *HasturSuite) TestEvent(c *C) {
	hastur.Event("test.event", "hey", "there", []string{"foo@bar.com"})
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "event")
	c.Check(m["name"], Equals, "test.event")
	c.Check(m["subject"], Equals, "hey")
	c.Check(m["body"], Equals, "there")
	c.Check(m["attn"], DeepEquals, []interface{}{"foo@bar.com"})
}

func (s *HasturSuite) TestLog(c *C) {
	hastur.Log("hey", "there")
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "log")
	c.Check(m["subject"], Equals, "hey")
	c.Check(m["data"], Equals, "there")
}

func (s *HasturSuite) TestRegisterProcess(c *C) {
	hastur.RegisterProcess(
		"test.process",
		map[string]interface{}{"haz": "data"},
		time.Now(),
		make(map[string]interface{}),
	)
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "reg_process")

	rawData, ok := m["data"]
	c.Assert(ok, Equals, true)
	data, ok := rawData.(map[string]interface{})
	c.Assert(ok, Equals, true)

	c.Check(data["name"], Equals, "test.process")
	c.Check(data["language"], Equals, "go")
	c.Check(data["haz"], Equals, "data")
}

func (s *HasturSuite) TestInfoProcess(c *C) {
	hastur.InfoProcess("test.tag", map[string]interface{}{"moar": "data"})
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "info_process")
	c.Check(m["tag"], Equals, "test.tag")
	c.Check(m["data"], DeepEquals, map[string]interface{}{"moar": "data"})
}

func (s *HasturSuite) TestInfoAgent(c *C) {
	hastur.InfoAgent("test.tag", map[string]interface{}{"agent": "data"})
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "info_agent")
	c.Check(m["tag"], Equals, "test.tag")
	c.Check(m["data"], DeepEquals, map[string]interface{}{"agent": "data"})
}

func (s *HasturSuite) TestHeartbeat(c *C) {
	hastur.Heartbeat()
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "hb_process")
	c.Check(m["name"], Equals, "application.heartbeat")
	c.Check(m["value"], Equals, 0.0)
	c.Check(m["timeout"], Equals, 0.0)
}

// Test some various behaviors not tied to particular message type

func (s *HasturSuite) TestLogOnError(c *C) {
	// Try to send a mark with a label value that can't be serialized to json.
	hastur.MarkFull("test.mark", "foo", time.Now(), map[string]interface{}{"foo": make(chan bool)})
	m := GetAndVerifySingleMessage(c)

	c.Check(m["type"], Equals, "log")
	c.Check(m["subject"], Matches, ".*unsupported type.*")
}

func (s *HasturSuite) TestDefaultLabels(c *C) {
	hastur.MarkFull("test.mark", "foo", time.Now(), map[string]interface{}{"label1": "value1"})
	hastur.AddDefaultLabels(map[string]interface{}{"label2": "value2"})
	hastur.MarkFull("test.mark", "foo", time.Now(), map[string]interface{}{"label1": "value1"})
	hastur.RemoveDefaultLabels("label2")
	hastur.MarkFull("test.mark", "foo", time.Now(), map[string]interface{}{"label1": "value1"})

	messages := FinishCapture()
	c.Assert(messages, HasLen, 3)

	allLabels := make([]map[string]interface{}, len(messages))
	for i, message := range messages {
		labels := GetLabels(c, message)
		c.Check(labels["label1"], Equals, "value1")
		allLabels[i] = labels
	}
	_, ok := allLabels[0]["label2"]
	c.Check(ok, Equals, false)
	value, ok := allLabels[1]["label2"]
	c.Check(ok, Equals, true)
	c.Check(value, Equals, "value2")
	_, ok = allLabels[2]["label2"]
	c.Check(ok, Equals, false)
}

func (s *HasturSuite) TestAppName(c *C) {
	hastur.SetAppName("")
	os.Setenv("HASTUR_APP_NAME", "env.name")
	hastur.Mark("foo", "bar")
	hastur.SetAppName("real.name")
	hastur.Mark("foo", "bar")

	messages := FinishCapture()
	c.Assert(messages, HasLen, 2)

	names := make([]interface{}, len(messages))
	for i, message := range messages {
		labels := GetLabels(c, message)
		names[i] = labels["app"]
	}

	c.Check(names[0], Equals, "env.name")
	c.Check(names[1], Equals, "real.name")
}
