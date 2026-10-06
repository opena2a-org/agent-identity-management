package store

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type unusedKeys struct{ public ed25519.PublicKey }

func (unusedKeys) SignPayload(context.Context, record.PayloadClass, record.Payload) (string, []byte, error) {
	return "", nil, errors.New("not used")
}

func (k unusedKeys) PublicKey(context.Context) (record.PublicKey, error) {
	return record.PublicKey{Alg: record.AlgEd25519, Key: k.public}, nil
}

// offlineWriter's database is never reachable, so a test that passes with it
// proves the writer decided before it took a connection.
func offlineWriter(t *testing.T) (*Writer, *prometheus.Registry, *bytes.Buffer) {
	t.Helper()
	db, err := sql.Open("postgres", "host=invalid.invalid connect_timeout=1 sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	reg := prometheus.NewRegistry()
	m, err := NewMetrics(reg)
	require.NoError(t, err)
	logs := &bytes.Buffer{}
	w, err := NewWriter(Config{
		DB: db, Keys: unusedKeys{public}, Key: record.PublicKey{Alg: record.AlgEd25519, Key: public},
		Metrics: m, Logger: log.New(logs, "", 0),
	})
	require.NoError(t, err)
	return w, reg, logs
}

func TestRecordWriteRefusesAnUnknownClassOrOrganization(t *testing.T) {
	w, reg, logs := offlineWriter(t)
	ctx := context.Background()

	_, err := w.Write(ctx, Write{Class: "growth", OrganizationID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"})
	require.ErrorIs(t, err, ErrInvalidWrite)
	_, err = w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: "3F2504E0-4F89-41D3-9A0C-0305E82C3301"})
	require.ErrorIs(t, err, ErrInvalidWrite)

	require.Equal(t, 0.0, failureTotal(t, reg), "a refused request is not a failed record write")
	require.Empty(t, logs.String())
}

// Every failed write's line names its debt. A write refused whole committed
// nothing and owes no record, so its line says debt=none, whatever its class.
func TestRecordRefusedWriteLineSaysNoDebt(t *testing.T) {
	for _, class := range classes {
		t.Run(string(class), func(t *testing.T) {
			w, _, logs := offlineWriter(t)
			_, err := w.Write(context.Background(), Write{
				Class:          class,
				OrganizationID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
				Draft: record.Draft{
					EventID:  "a1a1a1a1-0000-4000-8000-000000000001",
					Type:     "opena2a.administrative",
					Retained: map[string]any{"score": 0.5},
				},
			})
			var we *WriteError
			require.True(t, errors.As(err, &we), "want a *WriteError, got %v", err)
			require.Equal(t, "SECURITY record_write_failed class="+string(class)+" reason=canonical debt=none\n", logs.String())
		})
	}
}

// A draft that cannot be canonicalized fails before the write takes a
// connection or waits for the lock.
func TestRecordCanonicalFailureComesBeforeTheLock(t *testing.T) {
	w, reg, logs := offlineWriter(t)
	_, err := w.Write(context.Background(), Write{
		Class:          ClassReduction,
		OrganizationID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
		Draft: record.Draft{
			EventID:  "a1a1a1a1-0000-4000-8000-000000000001",
			Type:     "opena2a.administrative",
			Retained: map[string]any{"score": 0.5},
		},
	})
	var we *WriteError
	require.True(t, errors.As(err, &we), "want a *WriteError, got %v", err)
	require.Equal(t, ReasonCanonical, we.Reason)
	require.Equal(t, 1.0, failureTotal(t, reg))
	require.Equal(t, "SECURITY record_write_failed class=reduction reason=canonical debt=none\n", logs.String())
}

func TestRecordFailureSeriesExistAtZeroWithNoOrganizationLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	_, err := NewMetrics(reg)
	require.NoError(t, err)
	families, err := reg.Gather()
	require.NoError(t, err)
	series := 0
	for _, f := range families {
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				require.Contains(t, []string{"class", "reason"}, l.GetName(), "%s has label %s", f.GetName(), l.GetName())
			}
			if f.GetName() == "aim_record_write_failures_total" {
				series++
				require.Equal(t, 0.0, m.GetCounter().GetValue())
			}
		}
	}
	require.Equal(t, len(classes)*len(reasons), series)
}

func TestRecordLockWaitLineCountsCumulativeBuckets(t *testing.T) {
	var p lockWaitPeriod
	start := time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
	p.since = start
	for _, s := range []float64{0.0005, 0.001, 0.003, 0.3, 5} {
		p.observe(s)
	}
	line := p.line(start.Add(60 * time.Second))
	require.Equal(t, "record_append_lock_waits period_seconds=60.000 le_0.001=2 le_0.005=3 le_0.01=3 le_0.05=3 "+
		"le_0.1=3 le_0.25=3 le_0.5=4 le_1=4 le_2=4 le_inf=5", line)

	next := p.line(start.Add(120 * time.Second))
	require.True(t, strings.HasSuffix(next, " le_inf=0"), "a new period starts empty: %s", next)
}

func TestRecordLockTimeoutsAreOrdered(t *testing.T) {
	require.Less(t, StatementTimeout, LockTimeout)
	require.Less(t, IdleInTransactionTimeout, LockTimeout)
	require.Equal(t, "2000", millis(LockTimeout))
	require.Equal(t, "1", millis(0))
	require.Equal(t, "2", millis(1500*time.Microsecond))
}

func failureTotal(t *testing.T, reg *prometheus.Registry) float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	total := 0.0
	for _, f := range families {
		if f.GetName() == "aim_record_write_failures_total" {
			for _, m := range f.GetMetric() {
				total += m.GetCounter().GetValue()
			}
		}
	}
	return total
}

// signingKeys signs with one generated key and reports its public half.
type signingKeys struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

func (k signingKeys) PublicKey(context.Context) (record.PublicKey, error) {
	return record.PublicKey{Alg: record.AlgEd25519, Key: k.public}, nil
}

func (k signingKeys) SignPayload(_ context.Context, class record.PayloadClass, payload record.Payload) (string, []byte, error) {
	message, err := record.SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	return record.PublicKey{Alg: record.AlgEd25519, Key: k.public}.KeyID(), ed25519.Sign(k.private, message), nil
}

// The writer opens a signed record under the chain's key, so a record signed
// by any other key, or with its signature changed, does not open.
func TestRecordWriterOpensARecordOnlyUnderTheChainKey(t *testing.T) {
	ctx := context.Background()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keys := signingKeys{public: public, private: private}
	chainKey := record.PublicKey{Alg: record.AlgEd25519, Key: public}
	body, err := record.NewGenesis(record.Genesis{
		ChainID:  "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
		FirstKey: chainKey,
		Draft:    record.Draft{EventID: "3f2504e0-4f89-41d3-9a0c-0305e82c3302", Timestamp: time.Unix(1700000000, 0).UTC()},
	})
	require.NoError(t, err)
	rec, err := record.Sign(ctx, body, keys)
	require.NoError(t, err)

	payload, err := record.Open(rec.Envelope, record.ClassRecordV1, keyVerifier{chainKey})
	require.NoError(t, err)
	require.Equal(t, body.Canonical(), payload)

	other, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, err = record.Open(rec.Envelope, record.ClassRecordV1, keyVerifier{record.PublicKey{Alg: record.AlgEd25519, Key: other}})
	require.Error(t, err, "a record signed by another key does not open under the chain's key")

	tampered := rec.Envelope
	tampered.Signatures = append([]record.Signature(nil), rec.Envelope.Signatures...)
	sig := []byte(tampered.Signatures[0].Sig)
	sig[0] ^= 'A' ^ 'B'
	tampered.Signatures[0].Sig = string(sig)
	_, err = record.Open(tampered, record.ClassRecordV1, keyVerifier{chainKey})
	require.Error(t, err, "a changed signature does not open")

	_, err = record.Open(rec.Envelope, record.ClassRecordV1, keyVerifier{record.PublicKey{Alg: "RSA", Key: public}})
	require.Error(t, err, "a chain key that is not Ed25519 opens nothing")
}
