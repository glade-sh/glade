package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type GapSummary struct {
	Total          int            `json:"total"`
	ByGapType      map[string]int `json:"byGapType"`
	ByFamily       map[string]int `json:"byFamily"`
	ByDiffShape    map[string]int `json:"byDiffShape"`
	UnsupportedIDs []string       `json:"unsupportedIds"`
}

func SummarizeGapReport(path string) (GapSummary, error) {
	var report GapReport
	file, err := os.Open(path)
	if err != nil {
		return GapSummary{}, fmt.Errorf("open gap report: %w", err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return GapSummary{}, fmt.Errorf("decode gap report: %w", err)
	}
	return SummarizeGaps(report.Entries), nil
}

func SummarizeGaps(entries []GapEntry) GapSummary {
	summary := GapSummary{
		Total:          len(entries),
		ByGapType:      map[string]int{},
		ByFamily:       map[string]int{},
		ByDiffShape:    map[string]int{},
		UnsupportedIDs: make([]string, 0),
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

		if entry.GapType == GapTypeUnsupported {
			summary.UnsupportedIDs = append(summary.UnsupportedIDs, entry.ProbeID)
		}
	}
	sort.Strings(summary.UnsupportedIDs)
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
