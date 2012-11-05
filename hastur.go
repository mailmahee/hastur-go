/*
Package hastur is the Go Hastur API client which allows services/apps to easily publish correct Hastur
commands to their local machine's UDP sockets.

This library consists of message methods, such as Mark or Log, and helper utility methods such as Every.

Message methods each publish a single Hastur message to a local port via UDP. Most of these methods come in
two variants, the method and its "full" version (e.g., Mark and MarkFull). The Full version allows you to
specify all the available fields for the message, while the normal method only takes the most commonly used
parameters and uses the typical defaults for the remaining arguments (for example, the current time for
"timestamp" and an empty map for "labels").

The utility methods take care of some of the boilerplate for common Hastur client usages. For instance, Time,
TimeFull, and TimeCurrent each help time functions or code blocks and send results as a gauge. Every allows
for running some state reporting code on a regular interval. Functions are also provided to obtain and modify
the target UDP address/port and the default labels applied to all Hastur messages.

The app name and process ID are attached as labels to every Hastur message (as "app" and "pid", respectively).
The app name is chosen from either (a) a name set by SetAppName, (b) the environment variable HASTUR_APP_NAME,
or (c) the process name (preferred in that order).

You may call Start to automatically register your application and send heartbeat messages. This currently
sends messages each minute. If you set SendProcessHeartbeat to false before calling Start, heartbeat messages
will not be sent.
*/
package hastur

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

var (
	// Version is the current Go Hastur client library version.
	Version              = "0.0.1"
	udpAddress           = "127.0.0.1"
	udpPort              = 8125
	appName              string
	conn                 net.Conn
	defaultLabels        = make(map[string]interface{})
	recurringSend        = false
	// SendProcessHearbeat controls whether Start begins a periodic application heartbeat.
	SendProcessHeartbeat = true
)

func establishConn() {
	var err error
	conn, err = net.Dial("udp", fmt.Sprintf("%s:%d", udpAddress, udpPort))
	if err != nil {
		panic(err)
	}
}

// Interval specifies one of the time intervals that may be used in a call to Every.
type Interval int

const (
	FiveSecs Interval = iota
	Minute
	Hour
	Day
)

var intervalToDuration = map[Interval]time.Duration{
	FiveSecs: 5 * time.Second,
	Minute:   time.Minute,
	Hour:     time.Hour,
	Day:      24 * time.Hour,
}

// TimeFull is the same as Time but allows for explicit setting of the timestamp and labels.
func TimeFull(callback func(), name string, timestamp time.Time, labels map[string]interface{}) {
	start := time.Now()
	callback()
	end := time.Now()
	GaugeFull(name, end.Sub(start).Seconds(), timestamp, labels)
}

// Time runs a function and reports its runtime to Hastur as a gauge. callback is the function to run; name
// will be the name of the gauge message.
func Time(callback func(), name string) {
	TimeFull(callback, name, time.Now(), make(map[string]interface{}))
}

// TimeCurrent provides a convenient way to measure the time until the current function returns and report it
// to Hastur as a gauge. name is the name of the gauge and start is the starting time for measurement
// (generally time.Now()). This should be called using defer.
//
// Example:
//
//     func myFunc() {
//         defer TimeCurrent("myFunc", time.Now())
//         ...
//     }
func TimeCurrent(name string, start time.Time) {
	end := time.Now()
	Gauge(name, end.Sub(start).Seconds())
}

// Every runs callback code repeatedly at a fixed time interval. You can use this to collect and report
// periodic statistics. This is used by the default heartbeat message when you call Start.
func Every(interval Interval, callback func()) {
	duration, ok := intervalToDuration[interval]
	if !ok {
		panic(fmt.Sprintf("Every called with bad interval."))
	}
	go func() {
		ticker := time.NewTicker(duration)
		for {
			select {
			case <-ticker.C:
				callback()
			}
		}
	}()
}

// Start sends a periodic process heartbeat message once per minute.
func Start() {
	if SendProcessHeartbeat {
		Every(Minute, func() {
			Heartbeat("process_heartbeat")
		})
	}
	RegisterProcess(AppName(), make(map[string]interface{}), time.Now(), make(map[string]interface{}))
}

func init() {
	establishConn()
}

// Send an arbitrary message to the udp destination.
func send(message interface{}) {
	bytes, err := json.Marshal(message)
	if err != nil {
		if recurringSend {
			return
		}
		recurringSend = true
		defer func() { recurringSend = false }()
		Log(fmt.Sprintf("Error marshalling json message: %s", err.Error()), "")
		return
	}
	conn.Write(bytes)
}

// Convert time.Time to Hastur's time format (microseconds since epoch)
func convertTime(t time.Time) int64 { return t.UnixNano() / 1000 }

// UdpAddress returns the current target UDP address (defaulting to 127.0.0.1).
func UdpAddress() string { return udpAddress }

// SetUdpAddress sets the current target UDP address.
func SetUdpAddress(address string) {
	udpAddress = address
	establishConn()
}

// UdpPort returns the current target UDP port (defaulting to 8125).
func UdpPort() int { return udpPort }

// SetUdpPort sets the current target UDP port.
func SetUdpPort(port int) {
	udpPort = port
	establishConn()
}

// AddDefaultLabels adds label key/value pairs to the set of default labels to attach to every message.
func AddDefaultLabels(labels map[string]interface{}) {
	for label, value := range labels {
		defaultLabels[label] = value
	}
}

// RemoveDefaultLabels removes default labels from the default label set that were previously added using
// AddDefaultLabels. Provide labels to remove by key. This does not do anything if the labels given are not
// present in the default label list. The builtin default labels ("app" and "pid") cannot be removed.
func RemoveDefaultLabels(labels ...string) {
	for _, label := range labels {
		delete(defaultLabels, label)
	}
}

// DefaultLabels returns the current default labels which are attached to every Hastur message. This includes
// the defaults ("app" and "pid") and any additional labels added with AddDefaultLabels.
func DefaultLabels() map[string]interface{} {
	labels := map[string]interface{}{
		"pid": os.Getpid(),
		"app": AppName(),
	}
	for label, value := range defaultLabels {
		labels[label] = value
	}
	return labels
}

// Merge some extra labels with the default labels and return a new label map.
func mergeDefaultLabels(labels map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for label, value := range labels {
		result[label] = value
	}
	for label, value := range DefaultLabels() {
		result[label] = value
	}
	return result
}

// AppName returns the current app name as a string. This is chosen, in priority order, from: (a) an app name
// explicitly set with SetAppName, (b) the environment variable HASTUR_APP_NAME, or (c) the currently running
// executable.
func AppName() string {
	if appName != "" {
		return appName
	}
	if name := os.Getenv("HASTUR_APP_NAME"); name != "" {
		return name
	}
	return os.Args[0]
}

// SetAppName sets the current app name that will be attached to each message under the "app" label. This
// overrides all other sources of choosing an app name.
func SetAppName(name string) {
	appName = name
}

// MarkFull is the same as Mark but allows for explicit setting of the timestamp and labels.
func MarkFull(name, value string, timestamp time.Time, labels map[string]interface{}) {
	message := map[string]interface{}{
		"type":      "mark",
		"name":      name,
		"value":     value,
		"timestamp": convertTime(timestamp),
		"labels":    mergeDefaultLabels(labels),
	}
	send(message)
}

// Mark sends a 'mark' stat to Hastur. A mark gives the time that an interesting event occurred even with no
// value attached. You can also use a mark to send back string-valued stats that might otherwise be gauges --
// "Green", "Yellow", "Red" or similar.
//
// A mark is different from a Hastur event because it happens at stat priority -- it can be batched or
// slightly delayed, and doesn't have an end-to-end acknowledgement included.
func Mark(name, value string) {
	MarkFull(name, value, time.Now(), make(map[string]interface{}))
}

// CounterFull is the same as Counter but allows for explicit setting of the timestamp and labels.
func CounterFull(name string, value int, timestamp time.Time, labels map[string]interface{}) {
	message := map[string]interface{}{
		"type":      "counter",
		"name":      name,
		"value":     value,
		"timestamp": convertTime(timestamp),
		"labels":    mergeDefaultLabels(labels),
	}
	send(message)
}

// Counter sends a 'counter' stat to Hastur. Counters are linear, and are sent as deltas (differences).
// Sending a value of 1 adds 1 to the counter.
func Counter(name string, value int) {
	CounterFull(name, value, time.Now(), make(map[string]interface{}))
}

// GaugeFull is the same as Gauge but allows for explicit setting of the timestamp and labels.
func GaugeFull(name string, value float64, timestamp time.Time, labels map[string]interface{}) {
	message := map[string]interface{}{
		"type":      "gauge",
		"name":      name,
		"value":     value,
		"timestamp": convertTime(timestamp),
		"labels":    mergeDefaultLabels(labels),
	}
	send(message)
}

// Gauge sends a 'gauge' stat to Hastur. A gauge's value may or may not be on a linear scale. It is sent as an
// exact value, not a difference.
func Gauge(name string, value float64) {
	GaugeFull(name, value, time.Now(), make(map[string]interface{}))
}

// EventFull is the same as Event but allows for explicit setting of the timestamp and labels.
func EventFull(name, subject, body string, attn []string, timestamp time.Time, labels map[string]interface{}) {
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
		"labels":    mergeDefaultLabels(labels),
	}
	send(message)
}

// Event sends an event to Hastur. An event is high-priority and never buffered, and will be sent
// preferentially to stats or heartbeats. It includes an end-to-end acknowledgement mechanism to ensure
// arrival, but is expensive to store, send and query.
//
// 'attn' is a mechanism to describe the system or component in which the event occurs and who would care
// about it. Obvious values to include in the array include user logins, email addresses, team names, and
// server, library or component names. This allows making searches like "what events should I worry about?".
//
// The name is the name of the event (e.g., "bad.log.line"). The subject is a subject or message for this
// specific event. The body can contain additional details -- this could be a stack trace or an email body.
// "attn" are relevant components or teams. Web hooks or email addresses would go here.
func Event(name, subject, body string, attn []string) {
	EventFull(name, subject, body, attn, time.Now(), make(map[string]interface{}))
}

// LogFull is the same as Log but allows for explicit setting of the timestamp and labels.
func LogFull(subject string, data interface{}, timestamp time.Time, labels map[string]interface{}) {
	truncatedSubject := subject
	if len(subject) > 7168 {
		truncatedSubject = subject[:7168]
	}
	message := map[string]interface{}{
		"type":      "log",
		"subject":   truncatedSubject,
		"data":      data,
		"timestamp": convertTime(timestamp),
		"labels":    mergeDefaultLabels(labels),
	}
	send(message)
}

// Log sends a log line to Hastur. A log line is of relatively low priority, comparable to stats, and is
// allowed to be buffered or batched while higher-priority data is sent first.
//
// The data values must be convertable to json. Severity can be included in the data field with the tag
// "severity", if desired.
func Log(subject string, data interface{}) {
	LogFull(subject, data, time.Now(), make(map[string]interface{}))
}

// RegisterProcess sends a process registration to Hastur. This indicates that the process is currently
// running, and that heartbeats should be sent for some time afterward.
//
// The name parameter indicates the name of the app or process, while data is any additional information to
// include with the registration. The values of data must be convertable to json.
func RegisterProcess(name string, data map[string]interface{}, timestamp time.Time, labels map[string]interface{}) {
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
		"labels":    mergeDefaultLabels(labels),
	}
	send(message)
}

// InfoProcessFull is the same as InfoProcess but allows for explicit setting of the timestamp and labels.
func InfoProcessFull(tag string, data map[string]interface{}, timestamp time.Time, labels map[string]interface{}) {
	message := map[string]interface{}{
		"type": "info_process",
		"tag": tag,
		"data": data,
		"timestamp": convertTime(timestamp),
		"labels": mergeDefaultLabels(labels),
	}
	send(message)
}

// InfoProcess sends freeform process information to Hastur. This can be supplemental information about
// resources like memory, files open and whatnot. It can be additional configuration or deployment information
// like environment (dev/staging/prod), software or component version, etc. It can be information about the
// application as deployed, as run, or as it is currently running.
//
// The default labels contain application name and process ID to match this information with the process
// registration and similar details.
//
// Any number of these can be sent as information changes or is superceded. However, if information changes
// constantly or needs to be graphed or alerted on, send that separately as a metric or event. These messages
// are freeform and not readily separable or graphable.
func InfoProcess(tag string, data map[string]interface{}) {
	InfoProcessFull(tag, data, time.Now(), make(map[string]interface{}))
}

// InfoAgentFull is the same as InfoAgent but allows for explicit setting of the timestamp and labels.
func InfoAgentFull(tag string, data map[string]interface{}, timestamp time.Time, labels map[string]interface{}) {
	message := map[string]interface{}{
		"type": "info_agent",
		"tag": tag,
		"data": data,
		"timestamp": convertTime(timestamp),
		"labels": mergeDefaultLabels(labels),
	}
	send(message)
}


// InfoAgent sends back freeform data about the agent or host that Hastur is running on. Sample uses include
// what libraries or packages are installed and available, or the total installed memory.
//
// Any number of these can be sent as information changes or is superceded. However, if information changes
// constantly or needs to be graphed or alerted on, send that separately as a metric or event. These messages
// are freeform and not readily separable or graphable.
func InfoAgent(tag string, data map[string]interface{}) {
	InfoAgentFull(tag, data, time.Now(), make(map[string]interface{}))
}

// HeartbeatFull is the same as Heartbeat but allows for explicit setting of the timestamp and labels.
func HeartbeatFull(name string, value, timeout float64, timestamp time.Time, labels map[string]interface{}) {
	message := map[string]interface{}{
		"type":      "hb_process",
		"name":      name,
		"value":     value,
		"timeout":   timeout,
		"timestamp": convertTime(timestamp),
		"labels":    mergeDefaultLabels(labels),
	}
	send(message)
}

// Heartbeat sends a heartbeat to Hastur. A heartbeat is a periodic message which indicates that a host,
// application or service is currently running. It is higher priority than a statistic and should not be
// batched, but is lower priority than an event and does not include an end-to-end acknowledgement.
func Heartbeat(name string) {
	HeartbeatFull(name, 0, 0, time.Now(), make(map[string]interface{}))
}
