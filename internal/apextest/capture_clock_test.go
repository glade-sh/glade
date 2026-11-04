package apextest

import (
	"fmt"
	"time"
)

// normalizeClockCapture retains the observed ordering and row/future identity
// while avoiding an assertion about incidental elapsed milliseconds.
func normalizeClockCapture(value map[string]any) (map[string]any, error) {
	out := normalizeCaptureIdentities(value).(map[string]any)
	read := func(name string) (time.Time, error) {
		s, ok := out[name].(string)
		if !ok {
			return time.Time{}, fmt.Errorf("missing %s", name)
		}
		return time.Parse(time.RFC3339Nano, s)
	}
	before, err := read("before")
	if err != nil {
		return nil, err
	}
	after, err := read("after")
	if err != nil {
		return nil, err
	}
	future, err := read("future")
	if err != nil {
		return nil, err
	}
	if before.After(after) || !after.Before(future) {
		return nil, fmt.Errorf("invalid clock ordering")
	}
	rows, ok := out["rows"].([]any)
	if !ok || len(rows) != 1 {
		return nil, fmt.Errorf("expected one future row")
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid future row")
	}
	start, ok := row["StartDateTime"].(string)
	if !ok {
		return nil, fmt.Errorf("missing row start")
	}
	rowTime, err := time.Parse(time.RFC3339Nano, start)
	if err != nil || !rowTime.Equal(future) {
		return nil, fmt.Errorf("row and future differ")
	}
	out["before"], out["after"] = "<ordered-time>", "<ordered-time>"
	return out, nil
}
