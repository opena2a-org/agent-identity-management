package metrics

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// s1SeriesValue reads aim_s1_refusals_total{reason, sdk} from the registry the
// /metrics endpoint serves. A series that was never incremented reads as 0.
func s1SeriesValue(t *testing.T, reason, sdk string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather(): %v", err)
	}
	for _, family := range families {
		if family.GetName() != "aim_s1_refusals_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, pair := range metric.GetLabel() {
				labels[pair.GetName()] = pair.GetValue()
			}
			if labels["reason"] == reason && labels["sdk"] == sdk {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// fakeS1Clock stands in for time.AfterFunc: it records each period timer the log
// arms and lets the test end a period by firing one.
type fakeS1Clock struct {
	periods []time.Duration
	timers  []func()
	lines   []string
}

func (f *fakeS1Clock) log() *s1RefusalLog {
	return newS1RefusalLog(
		S1RefusalPeriod,
		func(d time.Duration, fire func()) {
			f.periods = append(f.periods, d)
			f.timers = append(f.timers, fire)
		},
		func(line string) { f.lines = append(f.lines, line) },
	)
}

func TestS1SDKLabel(t *testing.T) {
	tests := []struct {
		userAgent string
		want      string
	}{
		{"AIM-Python-SDK/2.0.2", "python"},
		{"AIM-Python-SDK/2.0.2 (A2A)", "python"},
		{"AIM-SDK-TypeScript/1.4.0", "typescript"},
		{"AIM-Java-SDK/1.0.0", "java"},
		{"", "other"},
		{"python-requests/2.32.3", "other"},
		{"curl/8.7.1", "other"},
		// A prefix match, not a substring match: the product token leads.
		{"curl/8.7.1 AIM-Python-SDK/2.0.2", "other"},
		// The SDK's telemetry relay is a different client from the request signer.
		{"OpenA2A-AIM-SDK-Relay/2.0.2", "other"},
		{"AIM-Python-SDK", "other"},
	}
	for _, tt := range tests {
		if got := s1SDKLabel(tt.userAgent); got != tt.want {
			t.Errorf("s1SDKLabel(%q) = %q, want %q", tt.userAgent, got, tt.want)
		}
	}
}

func TestS1RefusalPeriodIs64Seconds(t *testing.T) {
	if S1RefusalPeriod != 64*time.Second {
		t.Fatalf("S1RefusalPeriod = %v, want 64s", S1RefusalPeriod)
	}
	if s1Refusals.period != S1RefusalPeriod {
		t.Fatalf("the process-wide log uses a %v period, want %v", s1Refusals.period, S1RefusalPeriod)
	}
}

func TestS1RefusalLogWritesOneLinePerPeriodWithRefusals(t *testing.T) {
	clock := &fakeS1Clock{}
	l := clock.log()

	l.add(s1RefusalKey{S1ReasonSkewPast, "python"})
	l.add(s1RefusalKey{S1ReasonSkewPast, "python"})
	l.add(s1RefusalKey{S1ReasonSignatureInvalidEd25519, "typescript"})

	if !reflect.DeepEqual(clock.periods, []time.Duration{64 * time.Second}) {
		t.Fatalf("three refusals in one period armed timers %v, want one 64s timer", clock.periods)
	}
	if len(clock.lines) != 0 {
		t.Fatalf("a line was written before the period ended: %q", clock.lines)
	}

	clock.timers[0]()

	want := "s1_refusals period_s=64 total=3 signature_invalid_ed25519.typescript=1 skew_past.python=2"
	if !reflect.DeepEqual(clock.lines, []string{want}) {
		t.Fatalf("period line = %q, want exactly [%q]", clock.lines, want)
	}

	// The period has closed: its timer firing again, or a flush, writes nothing more.
	clock.timers[0]()
	l.flush()
	if len(clock.lines) != 1 {
		t.Fatalf("a closed period wrote again: %q", clock.lines)
	}
	if len(clock.timers) != 1 {
		t.Fatalf("a timer was armed with no refusal to count: %d timers", len(clock.timers))
	}

	// The next refusal opens the next period, which gets its own line.
	l.add(s1RefusalKey{S1ReasonSkewFuture, "java"})
	if len(clock.timers) != 2 {
		t.Fatalf("the refusal after a closed period armed %d timers in total, want 2", len(clock.timers))
	}
	clock.timers[1]()
	if got, want := clock.lines[len(clock.lines)-1], "s1_refusals period_s=64 total=1 skew_future.java=1"; got != want || len(clock.lines) != 2 {
		t.Fatalf("second period lines = %q, want a second line %q", clock.lines, want)
	}
}

func TestS1RefusalLogWritesNothingForAPeriodWithoutRefusals(t *testing.T) {
	clock := &fakeS1Clock{}
	l := clock.log()

	l.flush()

	if len(clock.lines) != 0 || len(clock.timers) != 0 {
		t.Fatalf("an idle log wrote %q and armed %d timers, want neither", clock.lines, len(clock.timers))
	}
}

// A flush (shutdown) ends a period early. The timer that period armed is still
// pending; when it fires it must not cut the following period short, or a refusal
// would be reported on a line covering less than a period while its own timer later
// finds nothing to write.
func TestS1RefusalLogFlushedPeriodTimerDoesNotCloseTheNextPeriod(t *testing.T) {
	clock := &fakeS1Clock{}
	l := clock.log()

	l.add(s1RefusalKey{S1ReasonSkewPast, "python"})
	l.flush()
	if !reflect.DeepEqual(clock.lines, []string{"s1_refusals period_s=64 total=1 skew_past.python=1"}) {
		t.Fatalf("flush wrote %q", clock.lines)
	}

	l.add(s1RefusalKey{S1ReasonInvalidTimestamp, "other"})
	clock.timers[0]() // the flushed period's timer
	if len(clock.lines) != 1 {
		t.Fatalf("the flushed period's timer closed the next period early: %q", clock.lines)
	}

	clock.timers[1]()
	if !reflect.DeepEqual(clock.lines[1:], []string{"s1_refusals period_s=64 total=1 invalid_timestamp.other=1"}) {
		t.Fatalf("the next period wrote %q", clock.lines[1:])
	}
}

// Refusals arrive on many request goroutines while periods close underneath them.
// Each one must land on exactly one line: the totals of every line written add up to
// the number of refusals, with none lost between two periods and none repeated.
func TestS1RefusalLogCountsEachConcurrentRefusalOnOneLine(t *testing.T) {
	const goroutines, each = 8, 500

	var mu sync.Mutex
	total := 0
	l := newS1RefusalLog(
		S1RefusalPeriod,
		func(time.Duration, func()) {},
		func(line string) {
			var n int
			if _, err := fmt.Sscanf(line, "s1_refusals period_s=64 total=%d", &n); err != nil {
				t.Errorf("unreadable line %q: %v", line, err)
			}
			mu.Lock()
			total += n
			mu.Unlock()
		},
	)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				l.add(s1RefusalKey{S1ReasonSkewPast, "python"})
				if i%50 == 0 {
					l.flush()
				}
			}
		}()
	}
	wg.Wait()
	l.flush()

	if total != goroutines*each {
		t.Fatalf("the lines account for %d refusals, want %d", total, goroutines*each)
	}
}

func TestRecordS1RefusalCountsEachReasonInItsOwnSeries(t *testing.T) {
	for reason := range s1RefusalReasons {
		before := s1SeriesValue(t, string(reason), "java")
		RecordS1Refusal(reason, "AIM-Java-SDK/1.0.0")
		if got := s1SeriesValue(t, string(reason), "java") - before; got != 1 {
			t.Errorf("RecordS1Refusal(%q) moved its series by %v, want 1", reason, got)
		}
	}
}

// A reason outside the closed set is a caller mistake. It must not become a label
// value, because the likeliest mistake is passing something the request carried.
func TestRecordS1RefusalKeepsRequestDataOutOfLabels(t *testing.T) {
	const agentID = "4f0c2c1e-8a55-4d0e-9d0b-6a3d1c7e2b90"
	const userAgent = "evil-agent/9.9 4f0c2c1e-signature-bytes"
	before := s1SeriesValue(t, string(S1ReasonUnclassified), "other")

	RecordS1Refusal(S1RefusalReason(agentID), userAgent)

	if got := s1SeriesValue(t, string(S1ReasonUnclassified), "other") - before; got != 1 {
		t.Fatalf("an unknown reason moved unclassified/other by %v, want 1", got)
	}

	sdks := map[string]bool{"python": true, "typescript": true, "java": true, "other": true}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather(): %v", err)
	}
	for _, family := range families {
		if family.GetName() != "aim_s1_refusals_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, pair := range metric.GetLabel() {
				switch pair.GetName() {
				case "reason":
					if !s1RefusalReasons[S1RefusalReason(pair.GetValue())] {
						t.Errorf("reason label %q is outside the closed set", pair.GetValue())
					}
				case "sdk":
					if !sdks[pair.GetValue()] {
						t.Errorf("sdk label %q is outside the closed set", pair.GetValue())
					}
				default:
					t.Errorf("unexpected label %q on aim_s1_refusals_total", pair.GetName())
				}
				if strings.Contains(pair.GetValue(), "4f0c2c1e") {
					t.Errorf("label %s=%q carries request data", pair.GetName(), pair.GetValue())
				}
			}
		}
	}
}
