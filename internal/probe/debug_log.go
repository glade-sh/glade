package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DebugLogSummary captures a stable, comparison-friendly shape for an org debug log.
type DebugLogSummary struct {
	Phase      string         `json:"phase"`
	ProbeID    string         `json:"probeId,omitempty"`
	ProbeIDs   []string       `json:"probeIds,omitempty"`
	Mode       string         `json:"mode"`
	TotalLines int            `json:"totalLines"`
	Events     map[string]int `json:"events"`
	Signature  string         `json:"signature"`
}

func SummarizeDebugLogs(logs []ProbeDebugLog) []DebugLogSummary {
	out := make([]DebugLogSummary, 0, len(logs))
	for _, entry := range logs {
		out = append(out, summarizeDebugLog(entry))
	}
	return out
}

func summarizeDebugLog(entry ProbeDebugLog) DebugLogSummary {
	lines := strings.Split(entry.Log, "\n")
	events := make(map[string]int)
	stream := make([]string, 0, len(lines))
	total := 0
	for _, line := range lines {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}
		total++
		event := extractDebugEvent(raw)
		if event == "" {
			continue
		}
		events[event]++
		stream = append(stream, event)
	}

	return DebugLogSummary{
		Phase:      entry.Phase,
		ProbeID:    entry.ProbeID,
		ProbeIDs:   append([]string(nil), entry.ProbeIDs...),
		Mode:       entry.Mode,
		TotalLines: total,
		Events:     events,
		Signature:  eventStreamSignature(stream),
	}
}

func extractDebugEvent(line string) string {
	first := strings.Index(line, "|")
	if first < 0 || first+1 >= len(line) {
		return ""
	}
	rest := line[first+1:]
	second := strings.Index(rest, "|")
	if second < 0 {
		return normalizeEvent(rest)
	}
	return normalizeEvent(rest[:second])
}

func normalizeEvent(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func eventStreamSignature(events []string) string {
	if len(events) == 0 {
		return ""
	}
	h := sha256.New()
	for _, event := range events {
		_, _ = h.Write([]byte(event))
		_, _ = h.Write([]byte{'\n'})
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if len(sum) <= 16 {
		return sum
	}
	return sum[:16]
}
