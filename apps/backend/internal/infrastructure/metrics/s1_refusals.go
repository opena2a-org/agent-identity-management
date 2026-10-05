package metrics

import (
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// S1 is the signed-request step of agent authentication: the middleware that checks
// an agent's request signature (Ed25519, ML-DSA or hybrid) before any handler runs.
//
// A request it refuses gets a 401 (400 for an unsupported algorithm). That response
// goes to the caller and nowhere else, so a server whose clock has drifted, or an SDK
// release that signs the wrong bytes, refuses honest agents exactly the way it refuses
// a forged request, and an operator could neither tell the two apart nor see either one
// happening. Every refusal is therefore recorded in two places:
//
//   - aim_s1_refusals_total{reason, sdk} on /metrics
//   - one "s1_refusals" log line per period in which any refusal occurred
//
// Both carry values from two closed sets and nothing else. A refusal happens before
// the caller is authenticated, so nothing the caller sent (agent id, signature, key,
// signed bytes, the raw User-Agent) may become a label or reach the line.

// S1RefusalReason is why the signed-request middleware refused a request. The set is
// closed: one constant per kind of refusal branch in the middleware, and a branch added
// there takes an existing constant or adds one here in the same change.
type S1RefusalReason string

const (
	// The X-Algorithm header names an algorithm the server cannot verify.
	S1ReasonUnsupportedAlgorithm S1RefusalReason = "unsupported_algorithm"
	// The X-Agent-ID header is not a UUID.
	S1ReasonInvalidAgentID S1RefusalReason = "invalid_agent_id"
	// The X-Timestamp header is not an integer.
	S1ReasonInvalidTimestamp S1RefusalReason = "invalid_timestamp"
	// The timestamp is older than the allowed clock skew: a caller clock running
	// slow, a server clock running fast, or a replayed request.
	S1ReasonSkewPast S1RefusalReason = "skew_past"
	// The timestamp is further ahead than the allowed clock skew: a caller clock
	// running fast or a server clock running slow.
	S1ReasonSkewFuture S1RefusalReason = "skew_future"
	// Loading the agent failed: it does not exist, or the lookup itself failed.
	S1ReasonAgentLookupFailed S1RefusalReason = "agent_lookup_failed"
	// The agent's status (revoked, suspended, unrecognised) does not permit auth.
	S1ReasonAgentStatusDenied S1RefusalReason = "agent_status_denied"
	// A signature or public-key header the chosen algorithm requires is absent.
	S1ReasonMissingSignatureHeaders S1RefusalReason = "missing_signature_headers"
	// The agent has no registered key for the chosen algorithm.
	S1ReasonNoRegisteredKey S1RefusalReason = "no_registered_key"
	// The key the request presents is not the key registered for the agent.
	S1ReasonPublicKeyMismatch S1RefusalReason = "public_key_mismatch"
	// The key registered for the agent cannot be decoded or has the wrong size.
	S1ReasonRegisteredKeyMalformed S1RefusalReason = "registered_key_malformed"
	// A signature header is not valid base64.
	S1ReasonSignatureMalformed S1RefusalReason = "signature_malformed"
	// The Ed25519 signature does not verify over the bytes the server reconstructs.
	S1ReasonSignatureInvalidEd25519 S1RefusalReason = "signature_invalid_ed25519"
	// The ML-DSA signature does not verify over the bytes the server reconstructs.
	S1ReasonSignatureInvalidMLDSA S1RefusalReason = "signature_invalid_mldsa"
	// Never used by the middleware. It is what RecordS1Refusal records for a value
	// outside this set, so a caller mistake cannot turn request data into a label.
	S1ReasonUnclassified S1RefusalReason = "unclassified"
)

var s1RefusalReasons = map[S1RefusalReason]bool{
	S1ReasonUnsupportedAlgorithm:    true,
	S1ReasonInvalidAgentID:          true,
	S1ReasonInvalidTimestamp:        true,
	S1ReasonSkewPast:                true,
	S1ReasonSkewFuture:              true,
	S1ReasonAgentLookupFailed:       true,
	S1ReasonAgentStatusDenied:       true,
	S1ReasonMissingSignatureHeaders: true,
	S1ReasonNoRegisteredKey:         true,
	S1ReasonPublicKeyMismatch:       true,
	S1ReasonRegisteredKeyMalformed:  true,
	S1ReasonSignatureMalformed:      true,
	S1ReasonSignatureInvalidEd25519: true,
	S1ReasonSignatureInvalidMLDSA:   true,
	S1ReasonUnclassified:            true,
}

// s1SDKOther is the sdk label for a caller that is not one of the three SDKs.
const s1SDKOther = "other"

// s1SDKUserAgentPrefixes maps the User-Agent each SDK sends to its sdk label. The
// match is a prefix match on the product token, so the version and any trailing
// comment ("AIM-Python-SDK/2.0.2 (A2A)") never reach the label.
var s1SDKUserAgentPrefixes = []struct {
	prefix string
	sdk    string
}{
	{"AIM-Python-SDK/", "python"},
	{"AIM-SDK-TypeScript/", "typescript"},
	{"AIM-Java-SDK/", "java"},
}

func s1SDKLabel(userAgent string) string {
	for _, known := range s1SDKUserAgentPrefixes {
		if strings.HasPrefix(userAgent, known.prefix) {
			return known.sdk
		}
	}
	return s1SDKOther
}

var s1RefusalsTotal = factory.NewCounterVec(
	prometheus.CounterOpts{
		Name: "aim_s1_refusals_total",
		Help: "Signed agent requests refused by the request-signature middleware, by reason and calling SDK",
	},
	[]string{"reason", "sdk"},
)

// S1RefusalPeriod is the length of one "s1_refusals" log period.
const S1RefusalPeriod = 64 * time.Second

type s1RefusalKey struct {
	reason S1RefusalReason
	sdk    string
}

// s1RefusalLog turns refusals into one log line per period. A period opens at the
// first refusal after the previous one closed and lasts S1RefusalPeriod; when it
// closes, the refusals counted in it are written as one line. Nothing is written,
// and no timer runs, while no request is being refused.
type s1RefusalLog struct {
	mu     sync.Mutex
	counts map[s1RefusalKey]uint64
	open   bool
	// gen numbers the open period, so the timer of a period that was already closed
	// by hand finds a different number and closes nothing.
	gen    uint64
	period time.Duration
	after  func(time.Duration, func())
	write  func(string)
}

func newS1RefusalLog(period time.Duration, after func(time.Duration, func()), write func(string)) *s1RefusalLog {
	return &s1RefusalLog{
		counts: make(map[s1RefusalKey]uint64),
		period: period,
		after:  after,
		write:  write,
	}
}

func (l *s1RefusalLog) add(key s1RefusalKey) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.counts[key]++
	if l.open {
		return
	}
	l.open = true
	l.gen++
	gen := l.gen
	l.after(l.period, func() { l.closePeriod(gen) })
}

// closePeriod writes the line for period gen if that period is still the open one.
func (l *s1RefusalLog) closePeriod(gen uint64) {
	l.mu.Lock()
	if !l.open || gen != l.gen {
		l.mu.Unlock()
		return
	}
	counts := l.counts
	l.counts = make(map[s1RefusalKey]uint64)
	l.open = false
	l.mu.Unlock()

	l.write(formatS1RefusalLine(l.period, counts))
}

// flush closes the open period now, if there is one.
func (l *s1RefusalLog) flush() {
	l.mu.Lock()
	gen := l.gen
	l.mu.Unlock()

	l.closePeriod(gen)
}

// formatS1RefusalLine renders one period as
//
//	s1_refusals period_s=64 total=3 signature_invalid_ed25519.typescript=1 skew_past.python=2
//
// with one reason.sdk=count field per series that was refused, in a stable order.
func formatS1RefusalLine(period time.Duration, counts map[s1RefusalKey]uint64) string {
	keys := make([]s1RefusalKey, 0, len(counts))
	var total uint64
	for key, n := range counts {
		keys = append(keys, key)
		total += n
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].reason != keys[j].reason {
			return keys[i].reason < keys[j].reason
		}
		return keys[i].sdk < keys[j].sdk
	})

	var b strings.Builder
	b.WriteString("s1_refusals period_s=")
	b.WriteString(strconv.FormatInt(int64(period/time.Second), 10))
	b.WriteString(" total=")
	b.WriteString(strconv.FormatUint(total, 10))
	for _, key := range keys {
		b.WriteByte(' ')
		b.WriteString(string(key.reason))
		b.WriteByte('.')
		b.WriteString(key.sdk)
		b.WriteByte('=')
		b.WriteString(strconv.FormatUint(counts[key], 10))
	}
	return b.String()
}

var s1Refusals = newS1RefusalLog(
	S1RefusalPeriod,
	func(d time.Duration, f func()) { time.AfterFunc(d, f) },
	func(line string) { log.Print(line) },
)

// RecordS1Refusal counts one request refused by the signed-request middleware.
//
// reason is the branch that refused it. userAgent is the request's User-Agent header;
// only the sdk label derived from it is kept. A reason outside the closed set is
// recorded as S1ReasonUnclassified.
func RecordS1Refusal(reason S1RefusalReason, userAgent string) {
	if !s1RefusalReasons[reason] {
		reason = S1ReasonUnclassified
	}
	sdk := s1SDKLabel(userAgent)

	s1RefusalsTotal.WithLabelValues(string(reason), sdk).Inc()
	s1Refusals.add(s1RefusalKey{reason: reason, sdk: sdk})
}

// FlushS1RefusalLine writes the "s1_refusals" line for the open period now instead of
// when the period ends. It writes nothing if no refusal has been counted since the
// last line. The server calls it on shutdown so that refusals in the final, shorter
// period still reach a line.
func FlushS1RefusalLine() {
	s1Refusals.flush()
}
