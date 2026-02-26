// Command asciicast parses an asciicast v3 JSONL session
// into a header and a list of events using the jsonl package.
//
// Usage:
//
//	go run .
//	go run . < session.jsonl
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/btwiuse/jsonl"
)

// Header represents an asciicast v3 header.
type Header struct {
	Version   int               `json:"version"`
	Term      Term              `json:"term"`
	Timestamp int64             `json:"timestamp"`
	Title     string            `json:"title"`
	Env       map[string]string `json:"env"`
}

// Term describes the terminal configuration.
type Term struct {
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	Type string `json:"type"`
}

// Event represents an asciicast v3 event: [time, type, data].
type Event struct {
	Time float64
	Type string
	Data string
}

func (e *Event) UnmarshalJSON(b []byte) error {
	var raw [3]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[0], &e.Time); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &e.Type); err != nil {
		return err
	}
	return json.Unmarshal(raw[2], &e.Data)
}

// sample contains an embedded asciicast v3 session for demonstration.
var sample = `{"version": 3, "term": {"cols": 80, "rows": 24, "type": "xterm-256color"}, "timestamp": 1504467315, "title": "Demo", "env": {"SHELL": "/bin/zsh"}}
# event stream follows the header
[0.248, "o", "\u001b[1;31mHello \u001b[32mWorld!\u001b[0m\n"]
[1.001, "o", "That was ok\rThis is better."]
[3.500, "m", ""]
[0.143, "o", "Now... "]
# terminal window resized to 90 cols and 30 rows
[2.050, "r", "90x30"]
[1.541, "o", "Bye!"]
[0.887, "x", "0"]
`

func main() {
	var src io.Reader

	// Use stdin if data is being piped in, otherwise use sample data.
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		src = os.Stdin
	} else {
		fmt.Println("(no stdin detected, using embedded sample data)")
		fmt.Println()
		src = bytes.NewBufferString(sample)
	}

	header, events, err := parse(src)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Header ===")
	fmt.Printf("  Version:   %d\n", header.Version)
	fmt.Printf("  Title:     %s\n", header.Title)
	fmt.Printf("  Timestamp: %d\n", header.Timestamp)
	fmt.Printf("  Terminal:  %dx%d (%s)\n", header.Term.Cols, header.Term.Rows, header.Term.Type)
	fmt.Printf("  Env:       %v\n", header.Env)

	fmt.Printf("\n=== Events (%d) ===\n", len(events))
	for i, e := range events {
		fmt.Printf("  [%d] t=%.3f type=%q data=%q\n", i, e.Time, e.Type, e.Data)
	}
}

// parse reads an asciicast v3 JSONL stream and returns the header and events.
func parse(src io.Reader) (*Header, []Event, error) {
	s := jsonl.NewScanner(src)
	s.SkipBlank = true
	s.SkipComments = []string{"#"}

	// The first line is the header.
	if !s.Next() {
		return nil, nil, fmt.Errorf("expected header line")
	}
	line, err := s.Line()
	if err != nil {
		return nil, nil, err
	}

	var header Header
	if err := line.Scan(&header); err != nil {
		return nil, nil, fmt.Errorf("parsing header: %w", err)
	}

	// Remaining lines are events.
	var events []Event
	for s.Next() {
		line, err := s.Line()
		if err != nil {
			return nil, nil, err
		}
		var event Event
		if err := line.Scan(&event); err != nil {
			return nil, nil, fmt.Errorf("parsing event: %w", err)
		}
		events = append(events, event)
	}
	if err := s.Err(); err != nil {
		return nil, nil, err
	}

	return &header, events, nil
}
