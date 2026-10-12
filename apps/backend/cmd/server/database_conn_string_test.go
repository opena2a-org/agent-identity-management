package main

// The database connection string by what PostgreSQL receives: a server that
// reads the startup packet and the password message records what the driver
// sent after parsing the string initDatabase built.

import (
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// loginRefusal is the message of the error the recording server answers every
// login with, so a test can tell a refused login from a string that never
// parsed.
const loginRefusal = "login refused by the recording server"

type recordedLogin struct {
	user     string
	database string
	password string
}

// recordingPostgres listens on a loopback port and speaks the start of the
// PostgreSQL protocol: it reads the startup packet, asks for the password in
// clear text, records what arrives and refuses the login.
func recordingPostgres(t *testing.T) (port int, logins func() []recordedLogin) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var (
		mu       sync.Mutex
		recorded []recordedLogin
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			answerPostgresLogin(conn, func(login recordedLogin) {
				mu.Lock()
				recorded = append(recorded, login)
				mu.Unlock()
			})
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
	})

	return ln.Addr().(*net.TCPAddr).Port, func() []recordedLogin {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedLogin(nil), recorded...)
	}
}

// answerPostgresLogin reads one login from conn, hands it to record and only
// then writes the refusal. initDatabase returns as soon as the refusal
// arrives, so a login recorded after the refusal was written could still be
// missing when the test reads the recorded logins.
func answerPostgresLogin(conn net.Conn, record func(recordedLogin)) {
	login, ok := readPostgresLogin(conn)
	if !ok {
		return
	}
	record(login)

	// ErrorResponse, so the driver returns instead of waiting for a session.
	fields := "SFATAL\x00C28P01\x00M" + loginRefusal + "\x00\x00"
	refusal := make([]byte, 5, 5+len(fields))
	refusal[0] = 'E'
	binary.BigEndian.PutUint32(refusal[1:], uint32(4+len(fields)))
	_, _ = conn.Write(append(refusal, fields...))
}

func readPostgresLogin(conn net.Conn) (recordedLogin, bool) {
	var login recordedLogin
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Startup packet: length, protocol version, then name and value pairs.
	startup, ok := readPostgresFrame(conn)
	if !ok || len(startup) < 4 {
		return login, false
	}
	pairs := strings.Split(string(startup[4:]), "\x00")
	for i := 0; i+1 < len(pairs); i += 2 {
		switch pairs[i] {
		case "user":
			login.user = pairs[i+1]
		case "database":
			login.database = pairs[i+1]
		}
	}

	// AuthenticationCleartextPassword.
	if _, err := conn.Write([]byte{'R', 0, 0, 0, 8, 0, 0, 0, 3}); err != nil {
		return login, false
	}

	// PasswordMessage: a type byte, then a frame holding the password and a
	// terminating zero byte.
	kind := make([]byte, 1)
	if _, err := io.ReadFull(conn, kind); err != nil || kind[0] != 'p' {
		return login, false
	}
	password, ok := readPostgresFrame(conn)
	if !ok {
		return login, false
	}
	login.password = strings.TrimSuffix(string(password), "\x00")
	return login, true
}

// readPostgresFrame reads a four-byte length that counts itself, then the
// bytes that follow it.
func readPostgresFrame(conn net.Conn) ([]byte, bool) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return nil, false
	}
	size := binary.BigEndian.Uint32(head)
	if size < 4 || size > 1<<16 {
		return nil, false
	}
	body := make([]byte, size-4)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, false
	}
	return body, true
}

func databaseConfigForLoginTest(port int) config.DatabaseConfig {
	return config.DatabaseConfig{
		Host:            "127.0.0.1",
		Port:            port,
		User:            "aim",
		Password:        "plain",
		Database:        "identity",
		SSLMode:         "disable",
		MaxConnections:  2,
		ConnMaxLifetime: time.Minute,
	}
}

// loginSentBy runs initDatabase against a recording server and returns the one
// login it received.
func loginSentBy(t *testing.T, database func(port int) config.DatabaseConfig) recordedLogin {
	t.Helper()
	port, logins := recordingPostgres(t)

	db, err := initDatabase(&config.Config{Database: database(port)})
	if db != nil {
		_ = db.Close()
	}
	require.ErrorContains(t, err, loginRefusal,
		"the connection string must parse and the login must reach the server, which refuses it")

	got := logins()
	require.Len(t, got, 1, "one login reaches the server")
	return got[0]
}

// The recording server records a login before it refuses it, so loginSentBy,
// which reads the recorded logins once initDatabase has seen the refusal,
// finds the login on every run. net.Pipe has no buffer: the server's write of
// the refusal does not return until the client has read all of it, so while
// the client has read only the first byte, the server can have done nothing
// that follows that write.
func TestRecordingPostgres_RecordsTheLoginBeforeRefusingIt(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))

	var (
		mu       sync.Mutex
		recorded []recordedLogin
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer server.Close()
		answerPostgresLogin(server, func(login recordedLogin) {
			mu.Lock()
			recorded = append(recorded, login)
			mu.Unlock()
		})
	}()

	// StartupMessage: length, protocol version 3.0, then name and value pairs.
	params := "user\x00aim\x00database\x00identity\x00\x00"
	startup := make([]byte, 8, 8+len(params))
	binary.BigEndian.PutUint32(startup, uint32(8+len(params)))
	binary.BigEndian.PutUint32(startup[4:], 3<<16)
	_, err := client.Write(append(startup, params...))
	require.NoError(t, err)

	ask := make([]byte, 9)
	_, err = io.ReadFull(client, ask)
	require.NoError(t, err)
	require.Equal(t, []byte{'R', 0, 0, 0, 8, 0, 0, 0, 3}, ask, "the server asks for the password in clear text")

	password := "plain\x00"
	message := make([]byte, 5, 5+len(password))
	message[0] = 'p'
	binary.BigEndian.PutUint32(message[1:], uint32(4+len(password)))
	_, err = client.Write(append(message, password...))
	require.NoError(t, err)

	kind := make([]byte, 1)
	_, err = io.ReadFull(client, kind)
	require.NoError(t, err)
	require.Equal(t, byte('E'), kind[0], "the server refuses the login")

	mu.Lock()
	got := append([]recordedLogin(nil), recorded...)
	mu.Unlock()
	assert.Equal(t, []recordedLogin{{user: "aim", database: "identity", password: "plain"}}, got,
		"the login is recorded before the refusal is written")

	rest, err := io.ReadAll(client)
	require.NoError(t, err)
	assert.Contains(t, string(rest), loginRefusal)
	<-done
}

func TestInitDatabase_PasswordReachesPostgresUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
	}{
		{name: "letters and digits", password: "plain42"},
		{name: "single quote inside", password: "it's"},
		{name: "single quote first and last", password: "'quoted'"},
		{name: "single quote before an equals sign", password: "a'=b"},
		{name: "two single quotes in a row", password: "a''b"},
		{name: "backslash inside", password: `back\slash`},
		{name: "backslash last", password: `ends-with\`},
		{name: "backslash before a single quote", password: `a\'b`},
		{name: "two backslashes in a row", password: `a\\b`},
		{name: "space inside", password: "two words"},
		{name: "empty", password: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			login := loginSentBy(t, func(port int) config.DatabaseConfig {
				database := databaseConfigForLoginTest(port)
				database.Password = tc.password
				return database
			})

			assert.Equal(t, tc.password, login.password)
			assert.Equal(t, "aim", login.user)
			assert.Equal(t, "identity", login.database)
		})
	}
}

// The string itself: every string setting is quoted, and inside the quotes a
// backslash and a single quote each carry a backslash in front.
func TestDatabaseConnString_QuotesAndEscapesEveryStringSetting(t *testing.T) {
	database := databaseConfigForLoginTest(5432)
	assert.Equal(t,
		`host='127.0.0.1' port=5432 user='aim' password='plain' dbname='identity' sslmode='disable'`,
		databaseConnString(database))

	database.User = "aim owner"
	database.Password = `it's a\b`
	database.Database = "o'brien"
	assert.Equal(t,
		`host='127.0.0.1' port=5432 user='aim owner' password='it\'s a\\b' dbname='o\'brien' sslmode='disable'`,
		databaseConnString(database))
}

// The user and the database name are written by the same rule as the password.
func TestInitDatabase_UserAndDatabaseNameReachPostgresUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name     string
		user     string
		database string
	}{
		{name: "letters only", user: "aim", database: "identity"},
		{name: "space inside", user: "aim owner", database: "identity records"},
		{name: "single quote inside", user: "o'brien", database: "o'brien"},
		{name: "backslash inside", user: `corp\aim`, database: `corp\identity`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			login := loginSentBy(t, func(port int) config.DatabaseConfig {
				database := databaseConfigForLoginTest(port)
				database.User = tc.user
				database.Database = tc.database
				return database
			})

			assert.Equal(t, tc.user, login.user)
			assert.Equal(t, tc.database, login.database)
			assert.Equal(t, "plain", login.password)
		})
	}
}
