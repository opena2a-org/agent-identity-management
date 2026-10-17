// Package smtptest is a loopback SMTP server for tests, written from RFC 5321
// (https://www.rfc-editor.org/rfc/rfc5321), RFC 3207 (STARTTLS,
// https://www.rfc-editor.org/rfc/rfc3207), RFC 4954 (AUTH,
// https://www.rfc-editor.org/rfc/rfc4954) and RFC 4616 (the PLAIN mechanism,
// https://www.rfc-editor.org/rfc/rfc4616). It listens on 127.0.0.1 only,
// accepts every sender and recipient, and records each command it receives and
// each mail transaction's envelope and message data. It never relays a message.
//
// The replies it gives:
//
//   - 220 when a session opens (§3.1);
//   - 250 to EHLO and HELO, 250 to MAIL FROM:<...> and RCPT TO:<...> (§4.1.1);
//   - 354 to DATA, and 250 once the data ends with <CRLF>.<CRLF> (§4.1.1.4);
//   - 250 to RSET and NOOP, 221 to QUIT (§4.1.1.5, §4.1.1.9, §4.1.1.10);
//   - 500 to a command it does not recognise, 501 to a MAIL or RCPT whose
//     argument does not parse, and 503 to MAIL, RCPT or DATA out of sequence
//     (§4.2.2, §4.3.2).
//
// A line of message data that begins with a period has that period removed
// (§4.5.2). The session's commands are dispatched in one place, session.run,
// and the EHLO reply is built in one place, session.ehloReply.
//
// By default the server offers no extension. RequireSTARTTLS and
// RequireAuthPlain add STARTTLS and AUTH PLAIN with the replies those RFCs
// give; NoSTARTTLS states the default explicitly.
package smtptest

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"
)

// Transaction is one mail transaction the server accepted.
type Transaction struct {
	// From is the envelope sender, the address inside MAIL FROM:<...>; it is
	// empty for the null reverse-path <>.
	From string
	// To are the envelope recipients, the addresses inside each RCPT TO:<...>,
	// in the order they were given.
	To []string
	// Data is the message data as received: each line with its <CRLF>, a
	// leading period removed, and without the terminating ".<CRLF>".
	Data string
	// TLS is true when the transaction ran after the session's STARTTLS
	// handshake.
	TLS bool
	// Username is the authentication identity of the session's successful
	// AUTH, or empty when the session did not authenticate.
	Username string
}

// Command is one command line the server received.
type Command struct {
	// Session numbers the connection the command arrived on, from 1, in the
	// order the server accepted the connections.
	Session int
	// Verb is the command's first word, upper-cased: EHLO, STARTTLS, AUTH,
	// MAIL. Its arguments are not kept, so the credentials on an AUTH line
	// are never recorded.
	Verb string
	// TLS is true when the command arrived after the session's STARTTLS
	// handshake, and false when it arrived in clear text.
	TLS bool
}

// Option configures a server that NewServer starts.
type Option func(*options)

type options struct {
	starttls bool
	auth     bool
	username string
	password string
}

// RequireSTARTTLS makes the server offer STARTTLS (RFC 3207) with a
// certificate NewServer generates for the IP address 127.0.0.1, and answer
// every command other than NOOP, EHLO, STARTTLS and QUIT with
// "530 5.7.0 Must issue a STARTTLS command first" until the TLS handshake is
// done (§4). STARTTLS is answered 220 before the handshake; after it the
// server forgets what it knew from the client, the EHLO argument included
// (§4.2), and no longer lists STARTTLS. CertPool returns a pool that trusts
// the certificate.
func RequireSTARTTLS() Option {
	return func(o *options) { o.starttls = true }
}

// NoSTARTTLS makes the server offer no STARTTLS at all: the EHLO reply does
// not list it and the STARTTLS command is answered 500, as any command the
// server does not recognise. This is the default; it undoes an earlier
// RequireSTARTTLS in the same option list.
func NoSTARTTLS() Option {
	return func(o *options) { o.starttls = false }
}

// RequireAuthPlain makes the server require AUTH with the PLAIN mechanism
// (RFC 4954, RFC 4616) for username and password before a mail transaction:
// MAIL before a successful AUTH is answered "530 5.7.0 Authentication
// required". The EHLO reply lists "AUTH PLAIN" only after the STARTTLS
// handshake; an AUTH in clear text is answered "538 5.7.11 Encryption
// required for requested authentication mechanism". After TLS, valid
// credentials are answered 235 and invalid ones "535 5.7.8 Authentication
// credentials invalid".
func RequireAuthPlain(username, password string) Option {
	return func(o *options) {
		o.auth = true
		o.username = username
		o.password = password
	}
}

// Server is a running loopback SMTP server.
type Server struct {
	listener net.Listener
	hostname string
	opts     options

	// tlsConfig and certificate are set when the server offers STARTTLS.
	tlsConfig   *tls.Config
	certificate *x509.Certificate

	mu           sync.Mutex
	transactions []Transaction
	commands     []Command
	sessions     int
	conns        map[net.Conn]struct{}
	closed       bool

	wg sync.WaitGroup
}

// sessionTimeout bounds how long a session waits for a client's next line
// and for the TLS handshake.
const sessionTimeout = 30 * time.Second

// maxLineLength bounds one line a client sends, its <CRLF> included. RFC 5321
// sets 512 for a command line and 1000 for a line of text (§4.5.3.1.4,
// §4.5.3.1.6); a longer line ends the session.
const maxLineLength = 1000

// NewServer starts a server on 127.0.0.1 at a port the system chooses. Close
// stops it.
func NewServer(opts ...Option) (*Server, error) {
	s := &Server{hostname: "127.0.0.1", conns: map[net.Conn]struct{}{}}
	for _, opt := range opts {
		opt(&s.opts)
	}
	if s.opts.starttls {
		certificate, err := newCertificate()
		if err != nil {
			return nil, fmt.Errorf("smtptest: generate certificate: %w", err)
		}
		s.certificate = certificate.Leaf
		s.tlsConfig = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.listener = listener
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// newCertificate returns a self-signed ECDSA P-256 certificate for the IP
// address 127.0.0.1, valid from an hour ago for a day.
func newCertificate() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "smtptest"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, nil
}

// Addr is the server's host:port.
func (s *Server) Addr() string { return s.listener.Addr().String() }

// Host is the address the server listens on, 127.0.0.1.
func (s *Server) Host() string { return s.listener.Addr().(*net.TCPAddr).IP.String() }

// Port is the port the server listens on.
func (s *Server) Port() int { return s.listener.Addr().(*net.TCPAddr).Port }

// CertPool returns a pool holding the certificate the server presents after
// STARTTLS, for a client's tls.Config RootCAs. It is nil when the server
// offers no STARTTLS.
func (s *Server) CertPool() *x509.CertPool {
	if s.certificate == nil {
		return nil
	}
	pool := x509.NewCertPool()
	pool.AddCert(s.certificate)
	return pool
}

// Transactions returns a copy of the transactions accepted so far, in the
// order their data ended.
func (s *Server) Transactions() []Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Transaction, len(s.transactions))
	for i, tx := range s.transactions {
		tx.To = append([]string(nil), tx.To...)
		out[i] = tx
	}
	return out
}

// Commands returns a copy of the commands received so far, on every session,
// in the order the server read them. Lines inside DATA and the responses of
// an AUTH exchange are not commands and are not included.
func (s *Server) Commands() []Command {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Command(nil), s.commands...)
}

// Close stops listening, ends every open session, and returns when each has
// ended. It may be called more than once.
func (s *Server) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	_ = s.listener.Close()
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns[conn] = struct{}{}
		s.sessions++
		id := s.sessions
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.conns, conn)
				s.mu.Unlock()
				_ = conn.Close()
			}()
			(&session{server: s, id: id, conn: conn, reader: bufio.NewReaderSize(conn, maxLineLength)}).run()
		}()
	}
}

func (s *Server) record(tx Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions = append(s.transactions, tx)
}

func (s *Server) recordCommand(cmd Command) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, cmd)
}

// session is one client connection.
type session struct {
	server *Server
	id     int
	conn   net.Conn
	reader *bufio.Reader

	// tls is set once the STARTTLS handshake is done.
	tls bool
	// greeted is set by EHLO or HELO.
	greeted bool
	// authenticated is set by a successful AUTH, and username to its
	// authentication identity.
	authenticated bool
	username      string
	// tx is the transaction in progress: from is set by MAIL, to by RCPT.
	tx      *Transaction
	hasFrom bool
}

var errLineTooLong = errors.New("line too long")

// readLine returns the next line without its line ending. RFC 5321 lines end
// in <CRLF>; a bare <LF> is taken as one too.
func (c *session) readLine() (string, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(sessionTimeout))
	line, err := c.reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return "", errLineTooLong
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r"), nil
}

// reply writes one reply: code and text, or a multiline reply with one line per
// text, every line but the last with a hyphen after the code (§4.2.1).
func (c *session) reply(code int, texts ...string) error {
	if len(texts) == 0 {
		texts = []string{""}
	}
	var b strings.Builder
	for i, text := range texts {
		sep := "-"
		if i == len(texts)-1 {
			sep = " "
		}
		fmt.Fprintf(&b, "%d%s%s\r\n", code, sep, text)
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(sessionTimeout))
	_, err := io.WriteString(c.conn, b.String())
	return err
}

// reset ends the transaction in progress, as RSET does (§4.1.1.5).
func (c *session) reset() {
	c.tx = nil
	c.hasFrom = false
}

// ehloReply is the server's answer to EHLO: its name, then one line per
// extension it offers (§4.1.1.1). STARTTLS is listed before the handshake and
// AUTH PLAIN after it.
func (c *session) ehloReply() []string {
	lines := []string{c.server.hostname}
	if c.server.opts.starttls && !c.tls {
		lines = append(lines, "STARTTLS")
	}
	if c.server.opts.auth && c.tls {
		lines = append(lines, "AUTH PLAIN")
	}
	return lines
}

// needsTLS reports whether verb must wait for STARTTLS: a server that
// requires TLS takes only NOOP, EHLO, STARTTLS and QUIT before it (RFC 3207
// §4).
func (c *session) needsTLS(verb string) bool {
	if !c.server.opts.starttls || c.tls {
		return false
	}
	switch verb {
	case "NOOP", "EHLO", "STARTTLS", "QUIT":
		return false
	}
	return true
}

// recognised reports whether the server knows verb. STARTTLS and AUTH are
// known only when the server offers them.
func (c *session) recognised(verb string) bool {
	switch verb {
	case "EHLO", "HELO", "MAIL", "RCPT", "DATA", "RSET", "NOOP", "QUIT":
		return true
	case "STARTTLS":
		return c.server.opts.starttls
	case "AUTH":
		return c.server.opts.auth
	}
	return false
}

// run serves the session's commands until QUIT, an error, or the server's
// Close. Every command is recorded and dispatched here.
func (c *session) run() {
	if c.reply(220, c.server.hostname+" ESMTP smtptest") != nil {
		return
	}
	for {
		line, err := c.readLine()
		if err != nil {
			if errors.Is(err, errLineTooLong) {
				_ = c.reply(500, "5.5.6 Line too long")
			}
			return
		}
		word, arg, _ := strings.Cut(line, " ")
		verb := strings.ToUpper(word)
		c.server.recordCommand(Command{Session: c.id, Verb: verb, TLS: c.tls})
		switch {
		case !c.recognised(verb):
			err = c.reply(500, "5.5.2 Command not recognized")
		case c.needsTLS(verb):
			err = c.reply(530, "5.7.0 Must issue a STARTTLS command first")
		default:
			switch verb {
			case "EHLO":
				c.reset()
				c.greeted = true
				err = c.reply(250, c.ehloReply()...)
			case "HELO":
				c.reset()
				c.greeted = true
				err = c.reply(250, c.server.hostname)
			case "STARTTLS":
				err = c.startTLS(arg)
			case "AUTH":
				err = c.auth(arg)
			case "MAIL":
				err = c.mail(arg)
			case "RCPT":
				err = c.rcpt(arg)
			case "DATA":
				err = c.data()
			case "RSET":
				c.reset()
				err = c.reply(250, "2.0.0 OK")
			case "NOOP":
				err = c.reply(250, "2.0.0 OK")
			case "QUIT":
				_ = c.reply(221, "2.0.0 "+c.server.hostname+" closing connection")
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// startTLS answers STARTTLS with 220 and runs the TLS handshake (RFC 3207
// §4). The command takes no parameters, and is out of sequence once TLS is in
// place. After the handshake the session forgets what it knew from the
// client, the EHLO and any transaction included (§4.2), and reads with a new
// buffer, so clear text a client sent after the command is discarded, never
// read as if it had arrived over TLS. A failed handshake ends the session.
func (c *session) startTLS(arg string) error {
	if arg != "" {
		return c.reply(501, "5.5.4 Syntax error (no parameters allowed)")
	}
	if c.tls {
		return c.reply(503, "5.5.1 TLS already active")
	}
	if err := c.reply(220, "2.0.0 Ready to start TLS"); err != nil {
		return err
	}
	tlsConn := tls.Server(c.conn, c.server.tlsConfig)
	_ = tlsConn.SetDeadline(time.Now().Add(sessionTimeout))
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	c.conn = tlsConn
	c.reader = bufio.NewReaderSize(tlsConn, maxLineLength)
	c.tls = true
	c.greeted = false
	c.authenticated = false
	c.username = ""
	c.reset()
	return nil
}

// auth runs an AUTH exchange (RFC 4954 §4) for the PLAIN mechanism (RFC
// 4616): the response is the base64 of "authzid NUL authcid NUL passwd",
// given on the AUTH line or after a "334 " challenge, where "=" is an empty
// initial response and "*" cancels the exchange.
func (c *session) auth(arg string) error {
	if !c.tls {
		return c.reply(538, "5.7.11 Encryption required for requested authentication mechanism")
	}
	if !c.greeted || c.authenticated || c.hasFrom {
		return c.reply(503, "5.5.1 Bad sequence of commands")
	}
	mechanism, response, hasResponse := strings.Cut(arg, " ")
	if !strings.EqualFold(mechanism, "PLAIN") {
		return c.reply(504, "5.5.4 Unrecognized authentication type")
	}
	if !hasResponse {
		if err := c.reply(334, ""); err != nil {
			return err
		}
		line, err := c.readLine()
		if err != nil {
			return err
		}
		response = line
	}
	if response == "*" {
		return c.reply(501, "5.0.0 Authentication cancelled")
	}
	if response == "=" {
		response = ""
	}
	message, err := base64.StdEncoding.DecodeString(response)
	if err != nil {
		return c.reply(501, "5.5.2 Cannot decode response")
	}
	parts := strings.Split(string(message), "\x00")
	if len(parts) != 3 || !c.validCredentials(parts[0], parts[1], parts[2]) {
		return c.reply(535, "5.7.8 Authentication credentials invalid")
	}
	c.authenticated = true
	c.username = parts[1]
	return c.reply(235, "2.7.0 Authentication successful")
}

// validCredentials checks a PLAIN message's parts against the server's
// username and password. An authorization identity, when given, must be the
// authentication identity itself.
func (c *session) validCredentials(authzid, authcid, passwd string) bool {
	if authzid != "" && authzid != authcid {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(authcid), []byte(c.server.opts.username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(passwd), []byte(c.server.opts.password)) == 1
	return userOK && passOK
}

// path returns the address inside the angle brackets that start arg after
// prefix ("FROM:" or "TO:", any case), ignoring any parameters after the
// closing bracket (§4.1.2).
func path(arg, prefix string) (string, bool) {
	if len(arg) < len(prefix) || !strings.EqualFold(arg[:len(prefix)], prefix) {
		return "", false
	}
	rest := strings.TrimLeft(arg[len(prefix):], " ")
	if !strings.HasPrefix(rest, "<") {
		return "", false
	}
	end := strings.IndexByte(rest, '>')
	if end < 0 {
		return "", false
	}
	return rest[1:end], true
}

func (c *session) mail(arg string) error {
	if !c.greeted || c.hasFrom {
		return c.reply(503, "5.5.1 Bad sequence of commands")
	}
	if c.server.opts.auth && !c.authenticated {
		return c.reply(530, "5.7.0 Authentication required")
	}
	from, ok := path(arg, "FROM:")
	if !ok {
		return c.reply(501, "5.5.4 Syntax: MAIL FROM:<address>")
	}
	c.tx = &Transaction{From: from}
	c.hasFrom = true
	return c.reply(250, "2.1.0 OK")
}

func (c *session) rcpt(arg string) error {
	if !c.hasFrom {
		return c.reply(503, "5.5.1 Bad sequence of commands")
	}
	to, ok := path(arg, "TO:")
	if !ok || to == "" {
		return c.reply(501, "5.5.4 Syntax: RCPT TO:<address>")
	}
	c.tx.To = append(c.tx.To, to)
	return c.reply(250, "2.1.5 OK")
}

// data reads the message data after a 354 until a line holding a period
// alone, removes the period that starts any other line (§4.5.2), records the
// transaction and answers 250.
func (c *session) data() error {
	if !c.hasFrom || len(c.tx.To) == 0 {
		return c.reply(503, "5.5.1 Bad sequence of commands")
	}
	if err := c.reply(354, "End data with <CR><LF>.<CR><LF>"); err != nil {
		return err
	}
	var b strings.Builder
	for {
		line, err := c.readLine()
		if err != nil {
			return err
		}
		if line == "." {
			break
		}
		b.WriteString(strings.TrimPrefix(line, "."))
		b.WriteString("\r\n")
	}
	tx := *c.tx
	tx.Data = b.String()
	tx.TLS = c.tls
	tx.Username = c.username
	c.server.record(tx)
	c.reset()
	return c.reply(250, "2.0.0 OK")
}
