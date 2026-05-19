package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type SummaryCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type GapSummary struct {
	Total                     int            `json:"total"`
	ByGapType                 map[string]int `json:"byGapType"`
	ByFamily                  map[string]int `json:"byFamily"`
	ByDiffShape               map[string]int `json:"byDiffShape"`
	StubSuperfamilyCounts     map[string]int `json:"stubSuperfamilyCounts"`
	DiffShapeTop              []SummaryCount `json:"diffShapeTop"`
	TraceClassificationCounts map[string]int `json:"traceClassificationCounts,omitempty"`
	UnsupportedIDs            []string       `json:"unsupportedIds"`
}

func SummarizeGapReport(path string) (GapSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GapSummary{}, fmt.Errorf("open gap report: %w", err)
	}
	var report GapReport
	if err := json.Unmarshal(data, &report); err != nil {
		return GapSummary{}, fmt.Errorf("decode gap report: %w", err)
	}
	summary := SummarizeGaps(report.Entries)

	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err == nil {
		summary.TraceClassificationCounts = extractTraceClassificationCounts(envelope["traceDiffs"])
	}

	return summary, nil
}

func SummarizeGaps(entries []GapEntry) GapSummary {
	summary := GapSummary{
		Total:                 len(entries),
		ByGapType:             map[string]int{},
		ByFamily:              map[string]int{},
		ByDiffShape:           map[string]int{},
		StubSuperfamilyCounts: map[string]int{},
		DiffShapeTop:          make([]SummaryCount, 0),
		UnsupportedIDs:        make([]string, 0),
	}
	for _, entry := range entries {
		gapType := string(entry.GapType)
		if gapType == "" {
			gapType = "unknown"
		}
		summary.ByGapType[gapType]++

		family := probeFamily(entry.ProbeID)
		if family == "" {
			family = "unknown"
		}
		summary.ByFamily[family]++

		shape := diffShape(entry)
		summary.ByDiffShape[shape]++

		if stubFamily := stubSuperfamily(entry.ProbeID); stubFamily != "" {
			summary.StubSuperfamilyCounts[stubFamily]++
		}

		if entry.GapType == GapTypeUnsupported {
			summary.UnsupportedIDs = append(summary.UnsupportedIDs, entry.ProbeID)
		}
	}
	sort.Strings(summary.UnsupportedIDs)
	summary.DiffShapeTop = topCounts(summary.ByDiffShape, len(summary.ByDiffShape))
	return summary
}

func probeFamily(id string) string {
	id = strings.TrimPrefix(id, "stub.")
	if dot := strings.Index(id, "."); dot >= 0 {
		return id[:dot]
	}
	return id
}

func diffShape(entry GapEntry) string {
	switch {
	case strings.HasPrefix(entry.Diff, "org throws ") && strings.Contains(entry.Diff, "; local throws "):
		orgExc := strings.TrimPrefix(entry.Diff, "org throws ")
		parts := strings.SplitN(orgExc, "; local throws ", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("throw:%s->%s", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	case strings.HasPrefix(entry.Diff, "org throws ") && strings.Contains(entry.Diff, "; local returns "):
		orgExc := strings.TrimPrefix(entry.Diff, "org throws ")
		parts := strings.SplitN(orgExc, "; local returns ", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("throw->return:%s", strings.TrimSpace(parts[0]))
		}
	case strings.HasPrefix(entry.Diff, "org returns ") && strings.Contains(entry.Diff, "; local throws "):
		orgRet := strings.TrimPrefix(entry.Diff, "org returns ")
		parts := strings.SplitN(orgRet, "; local throws ", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("return->throw:%s", strings.TrimSpace(parts[1]))
		}
	case strings.HasPrefix(entry.Diff, "org returns ") && strings.Contains(entry.Diff, "; local returns "):
		return "return->return"
	}
	return "other"
}

func stubSuperfamily(id string) string {
	if !strings.HasPrefix(id, "stub.") {
		return ""
	}
	stem := strings.TrimPrefix(id, "stub.")
	cut := len(stem)
	if dot := strings.Index(stem, "."); dot >= 0 && dot < cut {
		cut = dot
	}
	if dash := strings.Index(stem, "-"); dash >= 0 && dash < cut {
		cut = dash
	}
	if cut <= 0 {
		return "unknown"
	}
	return stem[:cut]
}

func topCounts(counts map[string]int, limit int) []SummaryCount {
	if limit <= 0 {
		return []SummaryCount{}
	}
	out := make([]SummaryCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, SummaryCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if limit > len(out) {
		limit = len(out)
	}
	return out[:limit]
}

func extractTraceClassificationCounts(raw any) map[string]int {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return map[string]int{}
	}
	counts := map[string]int{}
	for _, item := range items {
		classification := "unknown"
		if row, ok := item.(map[string]any); ok {
			if value, ok := row["classification"]; ok {
				if name, ok := value.(string); ok && strings.TrimSpace(name) != "" {
					classification = strings.TrimSpace(name)
				}
			}
		}
		counts[classification]++
	}
	return counts
}
