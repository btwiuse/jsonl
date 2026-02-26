package jsonl

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// ServerSession is a server-side session over a JSONL transport.
//
// It implements io.ReadWriteCloser:
//   - Read extracts data from incoming input ("i") events.
//   - Write wraps data as output ("o") events and sends them.
//   - Close closes the underlying connection.
//
// Resize events ("r") are delivered via NextResize.
type ServerSession interface {
	io.Reader
	io.Writer
	io.Closer

	// NextResize blocks until the next resize event from the client.
	// Up to 8 resize events are buffered; if the buffer is full, new
	// resize events are dropped until the caller consumes one.
	// Returns io.EOF when the session is closed.
	NextResize() (*Event, error)
}

// ClientSession is a client-side session over a JSONL transport.
//
// It implements io.ReadWriteCloser:
//   - Read extracts data from incoming output ("o") events.
//   - Write wraps data as input ("i") events and sends them.
//   - Close closes the underlying connection.
//
// Resize sends a resize event to the server.
type ClientSession interface {
	io.Reader
	io.Writer
	io.Closer

	// Resize sends a terminal resize event ("r") to the server.
	Resize(cols, rows int) error
}

// NewServerSession creates a server-side session wrapping conn.
// A background goroutine reads events from conn. Input data
// is available via Read; resize events via NextResize.
func NewServerSession(conn io.ReadWriteCloser) ServerSession {
	s := &serverSession{
		conn:     conn,
		resizeCh: make(chan *Event, 8),
	}
	s.pr, s.pw = io.Pipe()
	go s.readLoop()
	return s
}

type serverSession struct {
	conn     io.ReadWriteCloser
	pr       *io.PipeReader
	pw       *io.PipeWriter
	resizeCh chan *Event
	wmu      sync.Mutex
}

func (s *serverSession) readLoop() {
	defer s.pw.Close()
	defer close(s.resizeCh)

	scanner := NewScanner(s.conn)
	scanner.SkipBlank = true
	scanner.SkipComments = []string{"#"}

	for scanner.Next() {
		line, err := scanner.Line()
		if err != nil {
			s.pw.CloseWithError(err)
			return
		}
		var ev Event
		if err := line.Scan(&ev); err != nil {
			s.pw.CloseWithError(err)
			return
		}
		switch ev.Type {
		case "i":
			if _, err := s.pw.Write([]byte(ev.Data)); err != nil {
				return
			}
		case "r":
			select {
			case s.resizeCh <- &ev:
			default:
			}
		}
	}
	if err := scanner.Err(); err != nil {
		s.pw.CloseWithError(err)
	}
}

func (s *serverSession) Read(p []byte) (int, error) {
	return s.pr.Read(p)
}

func (s *serverSession) Write(p []byte) (int, error) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	if err := writeEvent(s.conn, &Event{Type: "o", Data: string(p)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *serverSession) Close() error {
	s.pr.Close()
	return s.conn.Close()
}

func (s *serverSession) NextResize() (*Event, error) {
	ev, ok := <-s.resizeCh
	if !ok {
		return nil, io.EOF
	}
	return ev, nil
}

// NewClientSession creates a client-side session wrapping conn.
// A background goroutine reads events from conn. Output data
// is available via Read.
func NewClientSession(conn io.ReadWriteCloser) ClientSession {
	c := &clientSession{
		conn: conn,
	}
	c.pr, c.pw = io.Pipe()
	go c.readLoop()
	return c
}

type clientSession struct {
	conn io.ReadWriteCloser
	pr   *io.PipeReader
	pw   *io.PipeWriter
	wmu  sync.Mutex
}

func (c *clientSession) readLoop() {
	defer c.pw.Close()

	scanner := NewScanner(c.conn)
	scanner.SkipBlank = true
	scanner.SkipComments = []string{"#"}

	for scanner.Next() {
		line, err := scanner.Line()
		if err != nil {
			c.pw.CloseWithError(err)
			return
		}
		var ev Event
		if err := line.Scan(&ev); err != nil {
			c.pw.CloseWithError(err)
			return
		}
		switch ev.Type {
		case "o":
			if _, err := c.pw.Write([]byte(ev.Data)); err != nil {
				return
			}
		}
	}
	if err := scanner.Err(); err != nil {
		c.pw.CloseWithError(err)
	}
}

func (c *clientSession) Read(p []byte) (int, error) {
	return c.pr.Read(p)
}

func (c *clientSession) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if err := writeEvent(c.conn, &Event{Type: "i", Data: string(p)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *clientSession) Close() error {
	c.pr.Close()
	return c.conn.Close()
}

func (c *clientSession) Resize(cols, rows int) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return writeEvent(c.conn, &Event{
		Type: "r",
		Data: fmt.Sprintf("%dx%d", cols, rows),
	})
}

// writeEvent encodes an event as JSON followed by a newline and writes it to w.
func writeEvent(w io.Writer, ev *Event) error {
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	_, err = w.Write(raw)
	return err
}
