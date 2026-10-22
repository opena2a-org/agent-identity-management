package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/stretchr/testify/require"
)

const (
	shapeOrganization = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	shapeEventID      = "a1a1a1a1-0000-4000-8000-000000000001"
	shapeCorrected    = "a1a1a1a1-0000-4000-8000-000000000002"
)

// generatedKey is the public half of an Ed25519 key generated when the test
// runs.
func generatedKey(t *testing.T) record.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return record.PublicKey{Alg: record.AlgEd25519, Key: public}
}

// A write that is both a key rotation and a correction, a key rotation that
// names the writer's own key as the next key, and a reduction that is either
// one and could commit with a debt are refused before the writer takes a
// connection, and none is a failed record write.
func TestRecordWriteRefusesARotationOrCorrectionRequestItCannotAppend(t *testing.T) {
	draft := record.Draft{EventID: shapeEventID}
	for _, tc := range []struct {
		name string
		req  func(t *testing.T, w *Writer) Write
	}{
		{"rotation_and_correction", func(t *testing.T, _ *Writer) Write {
			return Write{Class: ClassObservation, OrganizationID: shapeOrganization, Draft: draft,
				Rotation: &Rotation{NextKey: generatedKey(t), Reason: record.RotationScheduled},
				Corrects: shapeCorrected}
		}},
		{"rotation_to_the_writers_own_key", func(_ *testing.T, w *Writer) Write {
			return Write{Class: ClassObservation, OrganizationID: shapeOrganization, Draft: draft,
				Rotation: &Rotation{NextKey: w.key, Reason: record.RotationScheduled}}
		}},
		{"reduction_rotation_that_could_owe_a_debt", func(t *testing.T, _ *Writer) Write {
			return Write{Class: ClassReduction, OrganizationID: shapeOrganization, Draft: draft,
				Rotation: &Rotation{NextKey: generatedKey(t), Reason: record.RotationScheduled}}
		}},
		{"reduction_correction_that_could_owe_a_debt", func(_ *testing.T, _ *Writer) Write {
			return Write{Class: ClassReduction, OrganizationID: shapeOrganization, Draft: draft,
				Corrects: shapeCorrected}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, reg, logs := offlineWriter(t)
			_, err := w.Write(context.Background(), tc.req(t, w))
			require.ErrorIs(t, err, ErrInvalidWrite)
			var we *WriteError
			require.False(t, errors.As(err, &we), "a refused request is not a failed record write: %v", err)
			require.Equal(t, 0.0, failureTotal(t, reg))
			require.Empty(t, logs.String())
		})
	}
}

// A correction holds its reference in its tenant part, so the writer gives
// that part a salt even when the draft has no tenant member. A correction
// draft with no member at all is then refused, before the writer takes a
// connection, for having no trace members, as an ordinary draft with none is.
func TestRecordCorrectionWithNoTraceMembersFailsWithReasonTrace(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  Write
	}{
		{"ordinary_record", Write{Class: ClassObservation, OrganizationID: shapeOrganization,
			Draft: record.Draft{EventID: shapeEventID, Type: "opena2a.administrative"}}},
		{"correction", Write{Class: ClassObservation, OrganizationID: shapeOrganization,
			Draft: record.Draft{EventID: shapeEventID}, Corrects: shapeCorrected}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, reg, logs := offlineWriter(t)
			_, err := w.Write(context.Background(), tc.req)
			var we *WriteError
			require.True(t, errors.As(err, &we), "want a *WriteError, got %v", err)
			require.Equal(t, ReasonTrace, we.Reason, "%v", err)
			require.Equal(t, 1.0, failureTotal(t, reg))
			require.Equal(t, "SECURITY record_write_failed class=observation reason=trace debt=none\n", logs.String())
		})
	}
}

// A key rotation or a correction whose record cannot be built fails with
// reason canonical before the writer takes a connection, as any draft that
// cannot be serialized does. So does a draft of either type that is not
// written as one.
func TestRecordRotationOrCorrectionThatCannotBeBuiltFailsBeforeTheLock(t *testing.T) {
	began := time.Date(2026, time.October, 20, 9, 30, 0, 0, time.UTC)
	rotation := func(t *testing.T, recordType string, r Rotation) Write {
		if r.NextKey.Key == nil {
			r.NextKey = generatedKey(t)
		}
		return Write{Class: ClassObservation, OrganizationID: shapeOrganization,
			Draft: record.Draft{EventID: shapeEventID, Type: recordType}, Rotation: &r}
	}
	correction := func(recordType, corrects string) Write {
		return Write{Class: ClassObservation, OrganizationID: shapeOrganization,
			Draft: record.Draft{EventID: shapeEventID, Type: recordType}, Corrects: corrects}
	}
	for _, tc := range []struct {
		name string
		req  func(t *testing.T) Write
	}{
		{"exposure_that_does_not_state_when_it_began", func(t *testing.T) Write {
			return rotation(t, "", Rotation{Reason: record.RotationExposure})
		}},
		{"scheduled_rotation_that_states_an_exposure_time", func(t *testing.T) Write {
			return rotation(t, "", Rotation{Reason: record.RotationScheduled, ExposureBegan: began})
		}},
		{"rotation_reason_outside_the_closed_set", func(t *testing.T) Write {
			return rotation(t, "", Rotation{Reason: "routine"})
		}},
		{"next_key_that_is_not_ed25519", func(t *testing.T) Write {
			return rotation(t, "", Rotation{Reason: record.RotationScheduled,
				NextKey: record.PublicKey{Alg: "RSA", Key: generatedKey(t).Key}})
		}},
		{"rotation_typed_as_a_correction", func(t *testing.T) Write {
			return rotation(t, record.TypeCorrection, Rotation{Reason: record.RotationScheduled})
		}},
		{"correction_of_itself", func(*testing.T) Write {
			return correction("", shapeEventID)
		}},
		{"corrected_event_id_in_upper_case", func(*testing.T) Write {
			return correction("", "A1A1A1A1-0000-4000-8000-000000000002")
		}},
		{"correction_typed_as_another_record", func(*testing.T) Write {
			return correction("opena2a.administrative", shapeCorrected)
		}},
		{"key_rotation_type_with_no_rotation", func(*testing.T) Write {
			return correction(record.TypeKeyRotation, "")
		}},
		{"correction_type_with_no_corrected_record", func(*testing.T) Write {
			return correction(record.TypeCorrection, "")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, reg, logs := offlineWriter(t)
			_, err := w.Write(context.Background(), tc.req(t))
			var we *WriteError
			require.True(t, errors.As(err, &we), "want a *WriteError, got %v", err)
			require.Equal(t, ReasonCanonical, we.Reason, "%v", err)
			require.Equal(t, 1.0, failureTotal(t, reg))
			require.Equal(t, "SECURITY record_write_failed class=observation reason=canonical debt=none\n", logs.String())
		})
	}
}
