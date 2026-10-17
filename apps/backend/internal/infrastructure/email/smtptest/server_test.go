package smtptest

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// client speaks to the server one line at a time and reads each reply whole.
type client struct {
	t      *testing.T
	conn   net.Conn
	reader *bufio.Reader
}

func dial(t *testing.T, s *Server) *client {
	t.Helper()
	conn, err := net.Dial("tcp", s.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	return &client{t: t, conn: conn, reader: bufio.NewReader(conn)}
}

// read returns the next reply's code and its lines, a multiline reply's
// lines included (RFC 5321 §4.2.1).
func (c *client) read() (string, []string) {
	c.t.Helper()
	var lines []string
	for {
		line, err := c.reader.ReadString('\n')
		require.NoError(c.t, err)
		require.True(c.t, strings.HasSuffix(line, "\r\n"), "a reply line ends in CRLF: %q", line)
		line = strings.TrimSuffix(line, "\r\n")
		require.GreaterOrEqual(c.t, len(line), 4, "reply line %q", line)
		lines = append(lines, line[4:])
		if line[3] == ' ' {
			return line[:3], lines
		}
		require.Equal(c.t, byte('-'), line[3], "reply line %q", line)
	}
}

// send writes line with CRLF and returns the reply's code.
func (c *client) send(line string) string {
	c.t.Helper()
	code, _ := c.sendLines(line)
	return code
}

// sendLines writes line with CRLF and returns the reply's code and lines.
func (c *client) sendLines(line string) (string, []string) {
	c.t.Helper()
	_, err := c.conn.Write([]byte(line + "\r\n"))
	require.NoError(c.t, err)
	return c.read()
}

// sendReply writes line with CRLF and returns the reply as "<code> <text>",
// the last line's text for a multiline reply.
func (c *client) sendReply(line string) string {
	c.t.Helper()
	code, lines := c.sendLines(line)
	return code + " " + lines[len(lines)-1]
}

// startTLS sends STARTTLS, requires 220, and runs the client side of the
// handshake against roots; later commands go over TLS.
func (c *client) startTLS(roots *x509.CertPool) {
	c.t.Helper()
	require.Equal(c.t, "220", c.send("STARTTLS"))
	tlsConn := tls.Client(c.conn, &tls.Config{ServerName: "127.0.0.1", RootCAs: roots, MinVersion: tls.VersionTLS12})
	require.NoError(c.t, tlsConn.Handshake())
	c.conn = tlsConn
	c.reader = bufio.NewReader(tlsConn)
}

func newServer(t *testing.T, opts ...Option) *Server {
	t.Helper()
	s, err := NewServer(opts...)
	require.NoError(t, err)
	t.Cleanup(s.Close)
	return s
}

// plainResponse is the base64 PLAIN message "authzid NUL authcid NUL passwd"
// (RFC 4616 §2).
func plainResponse(authzid, authcid, passwd string) string {
	return base64.StdEncoding.EncodeToString([]byte(authzid + "\x00" + authcid + "\x00" + passwd))
}

// testCredentials builds a username and password at run time.
func testCredentials(t *testing.T) (string, string) {
	t.Helper()
	b := make([]byte, 12)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return "relay-user-" + hex.EncodeToString(b[:4]), hex.EncodeToString(b[4:])
}

func TestServerListensOnLoopbackOnly(t *testing.T) {
	s := newServer(t)
	assert.Equal(t, "127.0.0.1", s.Host())
	assert.NotZero(t, s.Port())
	assert.True(t, strings.HasPrefix(s.Addr(), "127.0.0.1:"), s.Addr())
}

// Each command is answered with the reply RFC 5321 gives it.
func TestServerReplies(t *testing.T) {
	s := newServer(t)
	c := dial(t, s)

	code, _ := c.read()
	assert.Equal(t, "220", code, "a session opens with 220")

	assert.Equal(t, "503", c.send("MAIL FROM:<a@example.com>"), "MAIL before EHLO or HELO is out of sequence")
	assert.Equal(t, "250", c.send("EHLO client.example.com"))
	assert.Equal(t, "250", c.send("HELO client.example.com"))
	assert.Equal(t, "250", c.send("NOOP"))
	assert.Equal(t, "500", c.send("VRFY a@example.com"), "an unrecognised command is answered 500")
	assert.Equal(t, "500", c.send("FROB"))
	assert.Equal(t, "503", c.send("RCPT TO:<b@example.com>"), "RCPT before MAIL is out of sequence")
	assert.Equal(t, "503", c.send("DATA"), "DATA before MAIL is out of sequence")
	assert.Equal(t, "501", c.send("MAIL FROM:a@example.com"), "a reverse-path without angle brackets does not parse")
	assert.Equal(t, "250", c.send("mail from:<a@example.com>"), "commands are case-insensitive")
	assert.Equal(t, "503", c.send("MAIL FROM:<again@example.com>"), "a second MAIL in one transaction is out of sequence")
	assert.Equal(t, "503", c.send("DATA"), "DATA before any RCPT is out of sequence")
	assert.Equal(t, "501", c.send("RCPT TO:<>"), "a recipient is required")
	assert.Equal(t, "250", c.send("RCPT TO:<b@example.com>"))
	assert.Equal(t, "250", c.send("RCPT TO:<c@example.com> NOTIFY=NEVER"), "parameters after the path are ignored")
	assert.Equal(t, "354", c.send("DATA"))
	_, err := c.conn.Write([]byte("Subject: hi\r\n\r\nbody\r\n.\r\n"))
	require.NoError(t, err)
	code, _ = c.read()
	assert.Equal(t, "250", code, "the data ends with <CRLF>.<CRLF> and is answered 250")
	assert.Equal(t, "250", c.send("RSET"))
	assert.Equal(t, "221", c.send("QUIT"))

	txs := s.Transactions()
	require.Len(t, txs, 1)
	assert.Equal(t, "a@example.com", txs[0].From)
	assert.Equal(t, []string{"b@example.com", "c@example.com"}, txs[0].To)
	assert.Equal(t, "Subject: hi\r\n\r\nbody\r\n", txs[0].Data)
}

// The EHLO reply names the server; it offers no extension.
func TestServerEHLOReply(t *testing.T) {
	c := dial(t, newServer(t))
	c.read()
	_, err := c.conn.Write([]byte("EHLO client.example.com\r\n"))
	require.NoError(t, err)
	code, lines := c.read()
	assert.Equal(t, "250", code)
	assert.Equal(t, []string{"127.0.0.1"}, lines)
}

// A line of data that begins with a period has that period removed
// (§4.5.2); a line holding a period alone ends the data.
func TestServerRemovesTheLeadingPeriod(t *testing.T) {
	s := newServer(t)
	c := dial(t, s)
	c.read()
	require.Equal(t, "250", c.send("EHLO client.example.com"))
	require.Equal(t, "250", c.send("MAIL FROM:<>"))
	require.Equal(t, "250", c.send("RCPT TO:<b@example.com>"))
	require.Equal(t, "354", c.send("DATA"))
	_, err := c.conn.Write([]byte("first\r\n..\r\n...two\r\n.x\r\nlast.\r\n.\r\n"))
	require.NoError(t, err)
	code, _ := c.read()
	require.Equal(t, "250", code)

	txs := s.Transactions()
	require.Len(t, txs, 1)
	assert.Equal(t, "", txs[0].From, "the null reverse-path <> is an empty sender")
	assert.Equal(t, "first\r\n.\r\n..two\r\nx\r\nlast.\r\n", txs[0].Data)
}

// RSET ends the transaction in progress: its sender and recipients are
// dropped, and the next one starts with MAIL.
func TestServerRSETDropsTheTransaction(t *testing.T) {
	s := newServer(t)
	c := dial(t, s)
	c.read()
	require.Equal(t, "250", c.send("EHLO client.example.com"))
	require.Equal(t, "250", c.send("MAIL FROM:<dropped@example.com>"))
	require.Equal(t, "250", c.send("RCPT TO:<dropped-rcpt@example.com>"))
	assert.Equal(t, "250", c.send("RSET"))
	assert.Equal(t, "503", c.send("RCPT TO:<b@example.com>"), "after RSET a recipient needs a new MAIL")
	assert.Equal(t, "503", c.send("DATA"), "after RSET there is no transaction")
	require.Equal(t, "250", c.send("MAIL FROM:<kept@example.com>"))
	require.Equal(t, "250", c.send("RCPT TO:<kept-rcpt@example.com>"))
	require.Equal(t, "354", c.send("DATA"))
	_, err := c.conn.Write([]byte("kept\r\n.\r\n"))
	require.NoError(t, err)
	code, _ := c.read()
	require.Equal(t, "250", code)

	txs := s.Transactions()
	require.Len(t, txs, 1, "the transaction RSET ended is not recorded")
	assert.Equal(t, "kept@example.com", txs[0].From)
	assert.Equal(t, []string{"kept-rcpt@example.com"}, txs[0].To)
	assert.Equal(t, "kept\r\n", txs[0].Data)
}

// A second EHLO resets the transaction as RSET does (§4.1.4).
func TestServerEHLOResetsTheTransaction(t *testing.T) {
	s := newServer(t)
	c := dial(t, s)
	c.read()
	require.Equal(t, "250", c.send("EHLO client.example.com"))
	require.Equal(t, "250", c.send("MAIL FROM:<a@example.com>"))
	require.Equal(t, "250", c.send("EHLO client.example.com"))
	assert.Equal(t, "503", c.send("RCPT TO:<b@example.com>"))
}

// Go's own SMTP client delivers through the server: one transaction per
// message, with the data dot-stuffed on the wire and unstuffed on receipt.
func TestServerReceivesFromNetSMTP(t *testing.T) {
	s := newServer(t)
	msg := "Subject: test\r\n\r\nline one\r\n.line with a period\r\n"
	require.NoError(t, smtp.SendMail(s.Addr(), nil, "from@example.com", []string{"to@example.com"}, []byte(msg)))

	txs := s.Transactions()
	require.Len(t, txs, 1)
	assert.Equal(t, "from@example.com", txs[0].From)
	assert.Equal(t, []string{"to@example.com"}, txs[0].To)
	assert.Equal(t, msg, txs[0].Data)
}

// A server that requires STARTTLS and AUTH PLAIN answers as RFC 3207, RFC
// 4954 and RFC 4616 say, before and after the handshake.
func TestServerSTARTTLSAndAuthPlainReplies(t *testing.T) {
	username, password := testCredentials(t)
	s := newServer(t, RequireSTARTTLS(), RequireAuthPlain(username, password))
	require.NotNil(t, s.CertPool())
	c := dial(t, s)
	c.read()

	code, lines := c.sendLines("EHLO client.example.com")
	assert.Equal(t, "250", code)
	assert.Equal(t, []string{"127.0.0.1", "STARTTLS"}, lines, "STARTTLS is listed before TLS, AUTH is not")
	assert.Equal(t, "530 5.7.0 Must issue a STARTTLS command first", c.sendReply("MAIL FROM:<a@example.com>"))
	assert.Equal(t, "530 5.7.0 Must issue a STARTTLS command first", c.sendReply("AUTH PLAIN "+plainResponse("", username, password)))
	assert.Equal(t, "530 5.7.0 Must issue a STARTTLS command first", c.sendReply("RSET"))
	assert.Equal(t, "530 5.7.0 Must issue a STARTTLS command first", c.sendReply("HELO client.example.com"))
	assert.Equal(t, "500", c.send("FROB"), "an unrecognised command is still answered 500")
	assert.Equal(t, "250", c.send("NOOP"))
	assert.Equal(t, "501", c.send("STARTTLS now"), "STARTTLS takes no parameters")
	c.startTLS(s.CertPool())

	assert.Equal(t, "503", c.send("MAIL FROM:<a@example.com>"), "after the handshake the EHLO is forgotten")
	code, lines = c.sendLines("EHLO client.example.com")
	assert.Equal(t, "250", code)
	assert.Equal(t, []string{"127.0.0.1", "AUTH PLAIN"}, lines, "AUTH PLAIN is listed after TLS, STARTTLS is not")
	assert.Equal(t, "503", c.send("STARTTLS"), "STARTTLS once TLS is in place is out of sequence")
	assert.Equal(t, "530 5.7.0 Authentication required", c.sendReply("MAIL FROM:<a@example.com>"))
	assert.Equal(t, "504", c.send("AUTH LOGIN"), "only PLAIN is offered")
	assert.Equal(t, "501", c.send("AUTH PLAIN not-base64!"))
	assert.Equal(t, "535 5.7.8 Authentication credentials invalid", c.sendReply("AUTH PLAIN "+plainResponse("", username, password+"x")))
	assert.Equal(t, "535 5.7.8 Authentication credentials invalid", c.sendReply("AUTH PLAIN "+plainResponse("", username+"x", password)))
	assert.Equal(t, "535 5.7.8 Authentication credentials invalid", c.sendReply("AUTH PLAIN "+plainResponse("someone-else", username, password)),
		"an authorization identity other than the authentication identity is refused")
	assert.Equal(t, "334", c.send("AUTH PLAIN"), "without an initial response the server asks for one")
	assert.Equal(t, "501 5.0.0 Authentication cancelled", c.sendReply("*"))
	assert.Equal(t, "334", c.send("AUTH PLAIN"))
	assert.Equal(t, "235 2.7.0 Authentication successful", c.sendReply(plainResponse("", username, password)))
	assert.Equal(t, "503", c.send("AUTH PLAIN "+plainResponse("", username, password)), "a second AUTH is out of sequence")
	require.Equal(t, "250", c.send("MAIL FROM:<a@example.com>"))
	require.Equal(t, "250", c.send("RCPT TO:<b@example.com>"))
	require.Equal(t, "354", c.send("DATA"))
	_, err := c.conn.Write([]byte("Subject: hi\r\n\r\nbody\r\n.\r\n"))
	require.NoError(t, err)
	code, _ = c.read()
	require.Equal(t, "250", code)
	assert.Equal(t, "221", c.send("QUIT"))

	txs := s.Transactions()
	require.Len(t, txs, 1)
	assert.True(t, txs[0].TLS)
	assert.Equal(t, username, txs[0].Username)
	assert.Equal(t, []string{"b@example.com"}, txs[0].To)

	var clear, overTLS []string
	for _, cmd := range s.Commands() {
		assert.Equal(t, 1, cmd.Session)
		if cmd.TLS {
			overTLS = append(overTLS, cmd.Verb)
		} else {
			clear = append(clear, cmd.Verb)
		}
	}
	assert.Equal(t, []string{"EHLO", "MAIL", "AUTH", "RSET", "HELO", "FROB", "NOOP", "STARTTLS", "STARTTLS"}, clear)
	assert.Equal(t, []string{"MAIL", "EHLO", "STARTTLS", "MAIL", "AUTH", "AUTH", "AUTH", "AUTH", "AUTH", "AUTH", "AUTH",
		"AUTH", "MAIL", "RCPT", "DATA", "QUIT"}, overTLS, "the responses of an AUTH exchange are not commands")
}

// An AUTH line's credentials are never recorded.
func TestServerDoesNotRecordAuthArguments(t *testing.T) {
	username, password := testCredentials(t)
	s := newServer(t, RequireSTARTTLS(), RequireAuthPlain(username, password))
	c := dial(t, s)
	c.read()
	require.Equal(t, "250", c.send("EHLO client.example.com"))
	c.startTLS(s.CertPool())
	require.Equal(t, "250", c.send("EHLO client.example.com"))
	require.Equal(t, "235", c.send("AUTH PLAIN "+plainResponse("", username, password)))
	for _, cmd := range s.Commands() {
		assert.NotContains(t, cmd.Verb, " ")
	}
}

// A server that offers no STARTTLS lists no extension, answers STARTTLS 500,
// and with AUTH PLAIN required refuses AUTH in clear text and MAIL before it.
func TestServerNoSTARTTLSReplies(t *testing.T) {
	username, password := testCredentials(t)
	s := newServer(t, RequireSTARTTLS(), NoSTARTTLS(), RequireAuthPlain(username, password))
	assert.Nil(t, s.CertPool(), "NoSTARTTLS undoes an earlier RequireSTARTTLS")
	c := dial(t, s)
	c.read()

	code, lines := c.sendLines("EHLO client.example.com")
	assert.Equal(t, "250", code)
	assert.Equal(t, []string{"127.0.0.1"}, lines, "neither STARTTLS nor AUTH is listed")
	assert.Equal(t, "500", c.send("STARTTLS"))
	assert.Equal(t, "538 5.7.11 Encryption required for requested authentication mechanism",
		c.sendReply("AUTH PLAIN "+plainResponse("", username, password)))
	assert.Equal(t, "530 5.7.0 Authentication required", c.sendReply("MAIL FROM:<a@example.com>"))
	assert.Equal(t, "221", c.send("QUIT"))
	assert.Empty(t, s.Transactions())
}

// Go's own SMTP client runs STARTTLS and AUTH PLAIN against the server and
// delivers a message over TLS.
func TestServerReceivesFromNetSMTPOverSTARTTLS(t *testing.T) {
	username, password := testCredentials(t)
	s := newServer(t, RequireSTARTTLS(), RequireAuthPlain(username, password))
	cl, err := smtp.Dial(s.Addr())
	require.NoError(t, err)
	defer func() { _ = cl.Close() }()
	ok, _ := cl.Extension("STARTTLS")
	require.True(t, ok)
	require.NoError(t, cl.StartTLS(&tls.Config{ServerName: "127.0.0.1", RootCAs: s.CertPool(), MinVersion: tls.VersionTLS12}))
	ok, mechanisms := cl.Extension("AUTH")
	require.True(t, ok)
	assert.Equal(t, "PLAIN", mechanisms)
	require.NoError(t, cl.Auth(smtp.PlainAuth("", username, password, "127.0.0.1")))
	require.NoError(t, cl.Mail("from@example.com"))
	require.NoError(t, cl.Rcpt("to@example.com"))
	w, err := cl.Data()
	require.NoError(t, err)
	_, err = w.Write([]byte("Subject: test\r\n\r\nbody\r\n"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, cl.Quit())

	txs := s.Transactions()
	require.Len(t, txs, 1)
	assert.True(t, txs[0].TLS)
	assert.Equal(t, username, txs[0].Username)
	assert.Equal(t, "Subject: test\r\n\r\nbody\r\n", txs[0].Data)
}

// A client that does not trust the server's certificate fails the handshake.
func TestServerCertificateIsTrustedOnlyThroughCertPool(t *testing.T) {
	s := newServer(t, RequireSTARTTLS())
	cl, err := smtp.Dial(s.Addr())
	require.NoError(t, err)
	defer func() { _ = cl.Close() }()
	assert.Error(t, cl.StartTLS(&tls.Config{ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}))
}

func TestServerCloseEndsOpenSessions(t *testing.T) {
	s, err := NewServer()
	require.NoError(t, err)
	c := dial(t, s)
	c.read()

	done := make(chan struct{})
	go func() {
		s.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return while a session was open")
	}
	s.Close()
	_, err = net.DialTimeout("tcp", s.Addr(), time.Second)
	assert.Error(t, err, "a closed server accepts no connection")
}
