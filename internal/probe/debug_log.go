package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/trace"
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

func SummarizeLocalTrace(probeID, mode string, events []trace.Event) DebugLogSummary {
	counts := make(map[string]int)
	stream := make([]string, 0, len(events))
	for _, event := range events {
		key := normalizeTraceEvent(event)
		if key == "" {
			continue
		}
		counts[key]++
		stream = append(stream, key)
	}
	return DebugLogSummary{
		Phase:      "local",
		ProbeID:    probeID,
		Mode:       mode,
		TotalLines: len(events),
		Events:     counts,
		Signature:  eventStreamSignature(stream),
	}
}

func normalizeTraceEvent(event trace.Event) string {
	name := strings.ToUpper(strings.TrimSpace(event.Name))
	cat := strings.ToUpper(strings.TrimSpace(event.Category))
	switch {
	case name == "" && cat == "":
		return ""
	case cat == "":
		return name
	case name == "":
		return cat
	default:
		return cat + ":" + name
	}
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

func CompareTraceSummaries(probeIDs []string, org, local []DebugLogSummary) []TraceDiff {
	orgByProbe := indexSummariesByProbeID(org)
	localByProbe := indexSummariesByProbeID(local)
	out := make([]TraceDiff, 0)
	for _, probeID := range probeIDs {
		o, okOrg := orgByProbe[probeID]
		l, okLocal := localByProbe[probeID]
		if !okOrg && !okLocal {
			continue
		}
		if !okOrg || !okLocal {
			d := TraceDiff{ProbeID: probeID, Classification: "trace_missing"}
			if okOrg {
				d.OrgSignature = o.Signature
				d.OrgEventCount = sumEvents(o.Events)
				d.Details = "local trace summary missing"
			}
			if okLocal {
				d.LocalSignature = l.Signature
				d.LocalEventCount = sumEvents(l.Events)
				d.Details = "org trace summary missing"
			}
			out = append(out, d)
			continue
		}
		if !equalEventCounts(o.Events, l.Events) {
			out = append(out, TraceDiff{
				ProbeID:         probeID,
				Classification:  "trace_event_delta",
				OrgSignature:    o.Signature,
				LocalSignature:  l.Signature,
				OrgEventCount:   sumEvents(o.Events),
				LocalEventCount: sumEvents(l.Events),
				Details:         "event count maps differ",
			})
			continue
		}
		if o.Signature != l.Signature {
			out = append(out, TraceDiff{
				ProbeID:         probeID,
				Classification:  "trace_signature_mismatch",
				OrgSignature:    o.Signature,
				LocalSignature:  l.Signature,
				OrgEventCount:   sumEvents(o.Events),
				LocalEventCount: sumEvents(l.Events),
				Details:         "event stream signatures differ",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Classification == out[j].Classification {
			return out[i].ProbeID < out[j].ProbeID
		}
		return out[i].Classification < out[j].Classification
	})
	return out
}

func indexSummariesByProbeID(summaries []DebugLogSummary) map[string]DebugLogSummary {
	out := make(map[string]DebugLogSummary, len(summaries))
	for _, summary := range summaries {
		if summary.ProbeID != "" {
			out[summary.ProbeID] = summary
			continue
		}
		for _, id := range summary.ProbeIDs {
			out[id] = summary
		}
	}
	return out
}

func equalEventCounts(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func sumEvents(events map[string]int) int {
	total := 0
	for _, count := range events {
		total += count
	}
	return total
}

func formatTraceDiffSummary(diffs []TraceDiff) string {
	if len(diffs) == 0 {
		return "none"
	}
	counts := map[string]int{}
	for _, diff := range diffs {
		counts[diff.Classification]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
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
