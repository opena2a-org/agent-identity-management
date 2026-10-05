package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemConfigRepository_IsMarkerSet(t *testing.T) {
	query := regexp.QuoteMeta(`SELECT value FROM system_config WHERE key = $1`)

	cases := []struct {
		name string
		rows *sqlmock.Rows
		want bool
	}{
		{"missing key", sqlmock.NewRows([]string{"value"}), false},
		{"set to true", sqlmock.NewRows([]string{"value"}).AddRow("true"), true},
		{"set to something else", sqlmock.NewRows([]string{"value"}).AddRow("false"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery(query).WithArgs("some_marker").WillReturnRows(tc.rows)

			got, err := NewSystemConfigRepository(db).IsMarkerSet(context.Background(), "some_marker")
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSystemConfigRepository_SetMarkerUpserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta("ON CONFLICT (key) DO UPDATE")).
		WithArgs("some_marker", "what it records").
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, NewSystemConfigRepository(db).SetMarker(context.Background(), "some_marker", "what it records"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMCPServerRepository_ListAllIDsReadsEveryOrganizationInOneQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	first, second := uuid.New(), uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM mcp_servers ORDER BY created_at, id`)).
		WithoutArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(first.String()).AddRow(second.String()))

	ids, err := NewMCPServerRepository(db).ListAllIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}
