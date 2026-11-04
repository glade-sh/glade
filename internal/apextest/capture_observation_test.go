package apextest

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// parseCaptureObservation requires the exact marker from the selected native
// method and a single complete JSON object. Ordinary assertions cannot pass as
// observations and a truncated or concatenated payload is never accepted.
func parseCaptureObservation(message, captureID string) (map[string]any, error) {
	if captureID == "" {
		return nil, fmt.Errorf("empty capture ID")
	}
	marker := "CAPTURE|" + captureID + "|"
	if strings.Count(message, marker) != 1 || strings.Count(message, "CAPTURE|") != 1 {
		return nil, fmt.Errorf("missing or repeated capture marker")
	}
	_, payload, _ := strings.Cut(message, marker)
	decoder := json.NewDecoder(strings.NewReader(payload))
	var result map[string]any
	if err := decoder.Decode(&result); err != nil || result == nil {
		return nil, fmt.Errorf("invalid capture object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing capture data")
	}
	return result, nil
}
