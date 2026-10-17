// Package hectest is a loopback HTTP Event Collector for tests. Only tests
// import it.
//
// The receiver enforces the event format and the error answers Splunk
// publishes for the HTTP Event Collector, read on 2026-10-16 from:
//
//	https://help.splunk.com/en/splunk-enterprise/get-started/get-data-in/10.6/get-data-with-http-event-collector/format-events-for-http-event-collector
//	https://help.splunk.com/en/splunk-enterprise/get-started/get-data-in/10.6/get-data-with-http-event-collector/troubleshoot-http-event-collector
//
// It is written from those pages, not from the adapter it tests, so an
// event the adapter gets wrong is refused here as Splunk would refuse it.
package hectest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// EventPath is the collector's JSON event endpoint.
const EventPath = "/services/collector/event"

// maxBody bounds the request body the receiver reads.
const maxBody = 8 << 20

// Answer is a collector answer: {"text":...,"code":...}.
type Answer struct {
	Status int    `json:"-"`
	Text   string `json:"text"`
	Code   int    `json:"code"`
}

// The answers the pages name. Each is the HTTP status and the body the
// collector sends.
var (
	Success           = Answer{http.StatusOK, "Success", 0}
	TokenRequired     = Answer{http.StatusUnauthorized, "Token is required", 2}
	InvalidAuth       = Answer{http.StatusUnauthorized, "Invalid authorization", 3}
	InvalidToken      = Answer{http.StatusForbidden, "Invalid token", 4}
	NoData            = Answer{http.StatusBadRequest, "No data", 5}
	InvalidDataFormat = Answer{http.StatusBadRequest, "Invalid data format", 6}
	EventRequired     = Answer{http.StatusBadRequest, "Event field is required", 12}
	EventBlank        = Answer{http.StatusBadRequest, "Event field cannot be blank", 13}
	IndexedFields     = Answer{http.StatusBadRequest, "Error in handling indexed fields", 15}
)

// Event is one event the receiver accepted.
type Event struct {
	Time       json.Number // empty when the event carried no time
	Host       string
	Source     string
	Sourcetype string
	Index      string
	Event      json.RawMessage
	Fields     map[string]json.RawMessage
}

// Decode unmarshals the event's event field into v.
func (e Event) Decode(v interface{}) error {
	return json.Unmarshal(e.Event, v)
}

// Refusal is one request the receiver did not accept. Status 0 is a
// connection the receiver closed without an answer.
type Refusal struct {
	Status int
	Code   int
	Text   string
}

// Receiver is a loopback collector that accepts the events of requests that
// carry its token and follow the published format. A request is accepted or
// refused whole: when one event in a batch is refused, none of the batch is
// recorded as accepted.
type Receiver struct {
	server *httptest.Server
	token  string

	mu               sync.Mutex
	accepted         []Event
	refusals         []Refusal
	refuseToken      bool
	closeConnections bool
}

// NewReceiver starts a receiver on loopback that accepts token. Close stops it.
func NewReceiver(token string) *Receiver {
	r := &Receiver{token: token}
	r.server = httptest.NewServer(http.HandlerFunc(r.serve))
	return r
}

// URL is the receiver's event endpoint.
func (r *Receiver) URL() string { return r.server.URL + EventPath }

// Close stops the receiver.
func (r *Receiver) Close() { r.server.Close() }

// Accepted returns the events accepted so far, in arrival order.
func (r *Receiver) Accepted() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.accepted...)
}

// Refusals returns the requests refused so far, in arrival order.
func (r *Receiver) Refusals() []Refusal {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Refusal(nil), r.refusals...)
}

// RefuseToken makes the receiver answer every request 403 Invalid token
// (code 4), as a collector does once the token is disabled or deleted.
func (r *Receiver) RefuseToken(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refuseToken = on
}

// CloseConnections makes the receiver close every connection without an
// answer.
func (r *Receiver) CloseConnections(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeConnections = on
}

func (r *Receiver) serve(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	refuseToken, closeConnections := r.refuseToken, r.closeConnections
	r.mu.Unlock()

	if closeConnections {
		r.refuse(Refusal{Text: "connection closed"})
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
				return
			}
		}
		panic(http.ErrAbortHandler)
	}
	// The receiver serves only the JSON event endpoint, and only POST to it.
	// These two answers are the receiver's own, not quoted from the pages.
	if req.URL.Path != EventPath {
		r.answer(w, Answer{http.StatusNotFound, "The requested URL was not found on this server.", 404})
		return
	}
	if req.Method != http.MethodPost {
		r.answer(w, Answer{http.StatusMethodNotAllowed, "Method not allowed", 405})
		return
	}
	if refuseToken {
		r.answer(w, InvalidToken)
		return
	}

	// "Authorization: Splunk <hec_token>".
	authorization := req.Header.Get("Authorization")
	switch {
	case authorization == "":
		r.answer(w, TokenRequired)
		return
	case !strings.HasPrefix(authorization, "Splunk "):
		r.answer(w, InvalidAuth)
		return
	case strings.TrimPrefix(authorization, "Splunk ") != r.token:
		r.answer(w, InvalidToken)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, maxBody))
	if err != nil {
		r.answer(w, InvalidDataFormat)
		return
	}
	events, refusal := parseBatch(body)
	if refusal != nil {
		r.answer(w, *refusal)
		return
	}
	r.mu.Lock()
	r.accepted = append(r.accepted, events...)
	r.mu.Unlock()
	writeAnswer(w, Success)
}

func (r *Receiver) refuse(refusal Refusal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refusals = append(r.refusals, refusal)
}

// answer records a refusal and sends it.
func (r *Receiver) answer(w http.ResponseWriter, a Answer) {
	r.refuse(Refusal{Status: a.Status, Code: a.Code, Text: a.Text})
	writeAnswer(w, a)
}

func writeAnswer(w http.ResponseWriter, a Answer) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(a.Status)
	_ = json.NewEncoder(w).Encode(a)
}

// parseBatch reads a body of event objects: "Both concatenated JSON objects
// and JSON arrays like this are accepted for sending batched events".
func parseBatch(body []byte) ([]Event, *Answer) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, &NoData
	}

	var objects []json.RawMessage
	if trimmed[0] == '[' {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		if err := dec.Decode(&objects); err != nil {
			return nil, &InvalidDataFormat
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			return nil, &InvalidDataFormat
		}
	} else {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		for {
			var object json.RawMessage
			err := dec.Decode(&object)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, &InvalidDataFormat
			}
			objects = append(objects, object)
		}
	}
	if len(objects) == 0 {
		return nil, &NoData
	}

	events := make([]Event, 0, len(objects))
	for _, object := range objects {
		event, refusal := parseEvent(object)
		if refusal != nil {
			return nil, refusal
		}
		events = append(events, event)
	}
	return events, nil
}

func parseEvent(object json.RawMessage) (Event, *Answer) {
	var fields map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(object))
	dec.UseNumber()
	if err := dec.Decode(&fields); err != nil || fields == nil {
		return Event{}, &InvalidDataFormat
	}

	raw, ok := fields["event"]
	if !ok {
		return Event{}, &EventRequired
	}
	if blank(raw) {
		return Event{}, &EventBlank
	}
	event := Event{Event: raw}

	if t, ok := fields["time"]; ok {
		number, valid := epochTime(t)
		if !valid {
			return Event{}, &InvalidDataFormat
		}
		event.Time = number
	}
	// host, source, sourcetype and index are strings. The pages name no
	// code for a value of another type; the receiver answers code 6.
	for name, dst := range map[string]*string{
		"host": &event.Host, "source": &event.Source, "sourcetype": &event.Sourcetype, "index": &event.Index,
	} {
		v, ok := fields[name]
		if !ok {
			continue
		}
		if err := json.Unmarshal(v, dst); err != nil {
			return Event{}, &InvalidDataFormat
		}
	}
	if f, ok := fields["fields"]; ok {
		indexed, valid := flatFields(f)
		if !valid {
			return Event{}, &IndexedFields
		}
		event.Fields = indexed
	}
	return event, nil
}

// blank reports whether an event field is empty: null or an empty string.
func blank(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if string(trimmed) == "null" {
		return true
	}
	var s string
	if json.Unmarshal(trimmed, &s) == nil {
		return strings.TrimSpace(s) == ""
	}
	return false
}

// epochTime reads a time field: epoch seconds, optionally with a fraction,
// or the integer milliseconds, microseconds or nanoseconds the pages also
// accept. Any other value is invalid.
func epochTime(raw json.RawMessage) (json.Number, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == '"' || trimmed[0] == '-' {
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	number, ok := v.(json.Number)
	if !ok {
		return "", false
	}
	if _, err := number.Float64(); err != nil {
		return "", false
	}
	return number, true
}

// flatFields reads the fields object: "a flat (not nested) list of explicit
// custom fields". A value is a string, number, boolean or null, or an array
// of those for a field with several values; an object anywhere is nesting.
func flatFields(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, false
	}
	for _, v := range fields {
		trimmed := bytes.TrimSpace(v)
		switch {
		case len(trimmed) == 0, trimmed[0] == '{':
			return nil, false
		case trimmed[0] == '[':
			var values []json.RawMessage
			if err := json.Unmarshal(trimmed, &values); err != nil {
				return nil, false
			}
			for _, value := range values {
				value = bytes.TrimSpace(value)
				if len(value) == 0 || value[0] == '{' || value[0] == '[' {
					return nil, false
				}
			}
		}
	}
	return fields, true
}
