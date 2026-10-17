// Package smtptest is a loopback SMTP server for tests, written from RFC 5321
// (https://www.rfc-editor.org/rfc/rfc5321). It listens on 127.0.0.1 only,
// accepts every sender and recipient, and records each mail transaction's
// envelope and message data. It never relays a message.
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
// and the EHLO reply is built in one place, ehloReply, so an extension such as
// STARTTLS or AUTH is added there.
package smtptest

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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
}

// Server is a running loopback SMTP server.
type Server struct {
	listener net.Listener
	hostname string

	mu           sync.Mutex
	transactions []Transaction
	conns        map[net.Conn]struct{}
	closed       bool

	wg sync.WaitGroup
}

// sessionTimeout bounds how long a session waits for a client's next line.
const sessionTimeout = 30 * time.Second

// maxLineLength bounds one line a client sends, its <CRLF> included. RFC 5321
// sets 512 for a command line and 1000 for a line of text (§4.5.3.1.4,
// §4.5.3.1.6); a longer line ends the session.
const maxLineLength = 1000

// NewServer starts a server on 127.0.0.1 at a port the system chooses. Close
// stops it.
func NewServer() (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{listener: listener, hostname: "127.0.0.1", conns: map[net.Conn]struct{}{}}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// Addr is the server's host:port.
func (s *Server) Addr() string { return s.listener.Addr().String() }

// Host is the address the server listens on, 127.0.0.1.
func (s *Server) Host() string { return s.listener.Addr().(*net.TCPAddr).IP.String() }

// Port is the port the server listens on.
func (s *Server) Port() int { return s.listener.Addr().(*net.TCPAddr).Port }

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
			(&session{server: s, conn: conn, reader: bufio.NewReaderSize(conn, maxLineLength)}).run()
		}()
	}
}

func (s *Server) record(tx Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions = append(s.transactions, tx)
}

// ehloReply is the server's answer to EHLO: its name, then one line per
// extension it offers (§4.1.1.1). It offers none.
func (s *Server) ehloReply() []string {
	return []string{s.hostname}
}

// session is one client connection.
type session struct {
	server *Server
	conn   net.Conn
	reader *bufio.Reader

	// greeted is set by EHLO or HELO.
	greeted bool
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

// run serves the session's commands until QUIT, an error, or the server's
// Close. Every command is dispatched here.
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
		verb, arg, _ := strings.Cut(line, " ")
		switch strings.ToUpper(verb) {
		case "EHLO":
			c.reset()
			c.greeted = true
			err = c.reply(250, c.server.ehloReply()...)
		case "HELO":
			c.reset()
			c.greeted = true
			err = c.reply(250, c.server.hostname)
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
		default:
			err = c.reply(500, "5.5.2 Command not recognized")
		}
		if err != nil {
			return
		}
	}
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
	c.server.record(tx)
	c.reset()
	return c.reply(250, "2.0.0 OK")
}
