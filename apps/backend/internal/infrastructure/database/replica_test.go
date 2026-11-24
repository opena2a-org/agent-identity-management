package database

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func fenceFor(t *testing.T) (*ReplicaFence, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()
	primary, primaryMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	replica, replicaMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = primary.Close(); _ = replica.Close() })
	fence, err := NewReplicaFence(primary, replica)
	if err != nil {
		t.Fatal(err)
	}
	return fence, primaryMock, replicaMock
}

const (
	positionQuery = `SELECT pg_current_wal_lsn\(\)::text`
	replayQuery   = `SELECT pg_last_wal_replay_lsn\(\) >= \$1::pg_lsn`
)

// The replica answers only when it has replayed the primary's position at the
// time of the read; a replica that is behind, cannot be asked, is not a
// standby, or a primary that cannot be asked, sends the read to the primary.
func TestReplicaFence_ReplicaAnswersOnlyWhenItHasCaughtUp(t *testing.T) {
	cases := []struct {
		name  string
		setup func(primary, replica sqlmock.Sqlmock)
		want  bool
	}{
		{"caught up", func(p, r sqlmock.Sqlmock) {
			p.ExpectQuery(positionQuery).WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow("0/3000060"))
			r.ExpectQuery(replayQuery).WithArgs("0/3000060").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(true))
		}, true},
		{"behind", func(p, r sqlmock.Sqlmock) {
			p.ExpectQuery(positionQuery).WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow("0/3000060"))
			r.ExpectQuery(replayQuery).WithArgs("0/3000060").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(false))
		}, false},
		{"not a standby", func(p, r sqlmock.Sqlmock) {
			p.ExpectQuery(positionQuery).WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow("0/3000060"))
			r.ExpectQuery(replayQuery).WithArgs("0/3000060").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(nil))
		}, false},
		{"replica unreachable", func(p, r sqlmock.Sqlmock) {
			p.ExpectQuery(positionQuery).WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow("0/3000060"))
			r.ExpectQuery(replayQuery).WithArgs("0/3000060").WillReturnError(errors.New("connection refused"))
		}, false},
		{"primary unreadable", func(p, r sqlmock.Sqlmock) {
			p.ExpectQuery(positionQuery).WillReturnError(errors.New("connection refused"))
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fence, primary, replica := fenceFor(t)
			c.setup(primary, replica)
			if got := fence.Replica(context.Background()); got != c.want {
				t.Errorf("Replica = %t, want %t", got, c.want)
			}
			if err := primary.ExpectationsWereMet(); err != nil {
				t.Error(err)
			}
			if err := replica.ExpectationsWereMet(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestNewReplicaFence_NeedsBothHandles(t *testing.T) {
	if _, err := NewReplicaFence(nil, nil); err == nil {
		t.Error("NewReplicaFence accepted missing handles")
	}
}
