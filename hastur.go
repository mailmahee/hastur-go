package hastur

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

var (
	Version    = "0.0.1"
	UdpAddress = "127.0.0.1"
	UdpPort    = 8125
	appName    string
	conn       net.Conn
)

func init() {
	establishConn()
}

func establishConn() {
	var err error
	conn, err = net.Dial("udp", fmt.Sprintf("%s:%d", UdpAddress, UdpPort))
	if err != nil {
		panic("handle this")
	}
}

// Send an arbitrary message to the udp destination.
func send(message interface{}) {
	bytes, err := json.Marshal(message)
	if err != nil {
		panic("Handle this.")
	}
	conn.Write(bytes)
}

// Convert time.Time to Hastur's time format (microseconds since epoch)
func convertTime(t time.Time) int64 {
	return t.UnixNano() / 1000
}

func SetUdpAddress(address string) {
	UdpAddress = address
	establishConn()
}

func SetUdpPort(port int) {
	UdpPort = port
	establishConn()
}

func AppName() string {
	if appName != "" {
		return appName
	}
	if name := os.Getenv("HASTUR_APP_NAME"); name != "" {
		return name
	}
	return os.Args[0]
}

func SetAppName(name string) {
	appName = name
}

func MarkFull(name, value string, timestamp time.Time, labels map[string]string) {
	message := map[string]interface{}{
		"type":      "mark",
		"name":      name,
		"value":     value,
		"timestamp": convertTime(timestamp),
		"labels":    labels,
	}
	send(message)
}

func Mark(name, value string) {
	MarkFull(name, value, time.Now(), make(map[string]string))
}

func CounterFull(name string, value int, timestamp time.Time, labels map[string]string) {
	message := map[string]interface{}{
		"type":      "counter",
		"name":      name,
		"value":     value,
		"timestamp": convertTime(timestamp),
		"labels":    labels,
	}
	send(message)
}

func Counter(name string, value int) {
	CounterFull(name, value, time.Now(), make(map[string]string))
}

func GaugeFull(name string, value float64, timestamp time.Time, labels map[string]string) {
	message := map[string]interface{}{
		"type":      "gauge",
		"name":      name,
		"value":     value,
		"timestamp": convertTime(timestamp),
		"labels":    labels,
	}
	send(message)
}

func Gauge(name string, value float64) {
	GaugeFull(name, value, time.Now(), make(map[string]string))
}

func EventFull(name, subject, body string, attn []string, timestamp time.Time, labels map[string]string) {
	truncatedSubject := subject
	if len(subject) > 3072 {
		truncatedSubject = subject[:3072]
	}
	truncatedBody := body
	if len(body) > 3072 {
		truncatedBody = body[:3072]
	}
	message := map[string]interface{}{
		"type":      "event",
		"name":      name,
		"subject":   truncatedSubject,
		"body":      truncatedBody,
		"attn":      attn,
		"timestamp": convertTime(timestamp),
		"labels":    labels,
	}
	send(message)
}

func Event(name, subject, body string, attn []string) {
	EventFull(name, subject, body, attn, time.Now(), make(map[string]string))
}

func LogFull(subject string, data interface{}, timestamp time.Time, labels map[string]string) {
	truncatedSubject := subject
	if len(subject) > 7168 {
		truncatedSubject = subject[:7168]
	}
	message := map[string]interface{}{
		"type":      "log",
		"subject":   truncatedSubject,
		"data":      data,
		"timestamp": convertTime(timestamp),
		"labels":    labels,
	}
	send(message)
}

func Log(subject string, data interface{}) {
	LogFull(subject, data, time.Now(), make(map[string]string))
}

func RegisterProcess(name string, data map[string]interface{}, timestamp time.Time, labels map[string]string) {
	allData := map[string]interface{}{
		"name":     name,
		"language": "go",
		"version":  Version,
	}
	for key, value := range data {
		allData[key] = value
	}
	message := map[string]interface{}{
		"type":      "reg_process",
		"data":      allData,
		"timestamp": convertTime(timestamp),
		"labels":    labels,
	}
	send(message)
}

func HeartbeatFull(name string, value, timeout float64, timestamp time.Time, labels map[string]string) {
	message := map[string]interface{}{
		"type":      "hb_process",
		"name":      name,
		"value":     value,
		"timeout":   timeout,
		"timestamp": convertTime(timestamp),
		"labels":    labels,
	}
	send(message)
}

func Heartbeat() {
	HeartbeatFull("application.heartbeat", 0, 0, time.Now(), make(map[string]string))
}

func TimeFull(callback func(), name string, timestamp time.Time, labels map[string]string) {
	start := time.Now()
	callback()
	end := time.Now()
	GaugeFull(name, end.Sub(start).Seconds(), timestamp, labels)
}

func Time(callback func(), name string) {
	TimeFull(callback, name, time.Now(), make(map[string]string))
}

// Time until the current function returns. Call with defer.
func TimeCurrent(name string, start time.Time) {
	end := time.Now()
	Gauge(name, end.Sub(start).Seconds())
}
