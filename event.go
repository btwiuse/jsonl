package jsonl

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Event represents an asciicast v3 event encoded as [time, type, data].
type Event struct {
	Time float64
	Type string
	Data string
}

// UnmarshalJSON decodes an event from [time, type, data] format.
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

// MarshalJSON encodes an event as [time, type, data] format.
func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal([3]any{e.Time, e.Type, e.Data})
}

// ResizeDimensions parses the Data field of a resize ("r") event.
// The expected format is "COLSxROWS".
func (e *Event) ResizeDimensions() (cols, rows int, err error) {
	parts := strings.SplitN(e.Data, "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("jsonl: invalid resize data: %q", e.Data)
	}
	cols, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("jsonl: invalid cols in resize data: %w", err)
	}
	rows, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("jsonl: invalid rows in resize data: %w", err)
	}
	return
}
