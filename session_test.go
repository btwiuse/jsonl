package jsonl_test

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/btwiuse/jsonl"
)

func TestEventMarshalRoundTrip(t *testing.T) {
	ev := &jsonl.Event{Time: 1.5, Type: "o", Data: "hello"}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	// Should be [1.5,"o","hello"]
	want := `[1.5,"o","hello"]`
	if string(raw) != want {
		t.Fatalf("got %s, want %s", raw, want)
	}

	var ev2 jsonl.Event
	if err := json.Unmarshal(raw, &ev2); err != nil {
		t.Fatal(err)
	}
	if ev2 != *ev {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", ev2, *ev)
	}
}

func TestResizeDimensions(t *testing.T) {
	ev := &jsonl.Event{Type: "r", Data: "90x30"}
	cols, rows, err := ev.ResizeDimensions()
	if err != nil {
		t.Fatal(err)
	}
	if cols != 90 || rows != 30 {
		t.Fatalf("got %dx%d, want 90x30", cols, rows)
	}
}

func TestResizeDimensionsInvalid(t *testing.T) {
	for _, data := range []string{"", "90", "axb", "90x"} {
		ev := &jsonl.Event{Type: "r", Data: data}
		_, _, err := ev.ResizeDimensions()
		if err == nil {
			t.Fatalf("expected error for data %q", data)
		}
	}
}

func TestSessionInputOutput(t *testing.T) {
	sconn, cconn := net.Pipe()

	ss := jsonl.NewServerSession(sconn)
	cs := jsonl.NewClientSession(cconn)

	defer ss.Close()
	defer cs.Close()

	// Client writes input, server reads it.
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := cs.Write([]byte("hello")); err != nil {
			t.Errorf("client write: %v", err)
		}
	}()

	buf := make([]byte, 1024)
	n, err := ss.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("server read: got %q, want %q", string(buf[:n]), "hello")
	}
	<-done

	// Server writes output, client reads it.
	done = make(chan struct{})
	go func() {
		defer close(done)
		if _, err := ss.Write([]byte("world")); err != nil {
			t.Errorf("server write: %v", err)
		}
	}()

	n, err = cs.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "world" {
		t.Fatalf("client read: got %q, want %q", string(buf[:n]), "world")
	}
	<-done
}

func TestSessionResize(t *testing.T) {
	sconn, cconn := net.Pipe()

	ss := jsonl.NewServerSession(sconn)
	cs := jsonl.NewClientSession(cconn)

	defer ss.Close()
	defer cs.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := cs.Resize(90, 30); err != nil {
			t.Errorf("client resize: %v", err)
		}
	}()

	ev, err := ss.NextResize()
	if err != nil {
		t.Fatal(err)
	}
	cols, rows, err := ev.ResizeDimensions()
	if err != nil {
		t.Fatal(err)
	}
	if cols != 90 || rows != 30 {
		t.Fatalf("resize: got %dx%d, want 90x30", cols, rows)
	}
	<-done
}

func TestSessionClose(t *testing.T) {
	sconn, cconn := net.Pipe()

	ss := jsonl.NewServerSession(sconn)
	cs := jsonl.NewClientSession(cconn)

	// Close client side; server Read should get EOF.
	cs.Close()

	buf := make([]byte, 1024)
	_, err := ss.Read(buf)
	if err == nil {
		t.Fatal("expected error after close")
	}

	ss.Close()
}

func TestSessionWriteReturnsByteCount(t *testing.T) {
	sconn, cconn := net.Pipe()

	ss := jsonl.NewServerSession(sconn)
	cs := jsonl.NewClientSession(cconn)

	defer ss.Close()
	defer cs.Close()

	// Server Write should return len(p), not the wire byte count.
	go func() {
		buf := make([]byte, 1024)
		cs.Read(buf) // drain the output
	}()

	data := []byte("test data 123")
	n, err := ss.Write(data)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Fatalf("Write returned %d, want %d", n, len(data))
	}
}
