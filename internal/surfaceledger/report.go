package surfaceledger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func WriteLedgerJSON(w io.Writer, ledger SurfaceLedger) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(ledger)
}

func WriteRowsJSON(w io.Writer, rows []SurfaceLedgerRow) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func ReadRowsJSON(path string) ([]SurfaceLedgerRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []SurfaceLedgerRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func ReadLedgerJSON(path string) (SurfaceLedger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SurfaceLedger{}, err
	}
	var ledger SurfaceLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return SurfaceLedger{}, err
	}
	for i := range ledger.Rows {
		Classify(&ledger.Rows[i])
	}
	ledger.Summary = Summarize(ledger.Rows)
	if ledger.SchemaVersion == 0 {
		ledger.SchemaVersion = SchemaVersion
	}
	return ledger, nil
}

func marshalPretty(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func DashboardMarkdown(ledger SurfaceLedger) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Dashboard")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| bucket | count |")
	fmt.Fprintln(&b, "| --- | ---: |")
	fmt.Fprintf(&b, "| implemented | %d |\n", ledger.Summary.Implemented)
	fmt.Fprintf(&b, "| partial | %d |\n", ledger.Summary.Partial)
	fmt.Fprintf(&b, "| passive | %d |\n", ledger.Summary.Passive)
	fmt.Fprintf(&b, "| explicitUnsupported | %d |\n", ledger.Summary.ExplicitUnsupported)
	fmt.Fprintf(&b, "| gap | %d |\n", sumMap(ledger.Summary.Gaps))
	fmt.Fprintf(&b, "| failure | %d |\n", sumMap(ledger.Summary.Failures))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Owners")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| owner | rows |")
	fmt.Fprintln(&b, "| --- | ---: |")
	for _, count := range ownerCounts(ledger.Rows) {
		fmt.Fprintf(&b, "| %s | %d |\n", count.Name, count.Count)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Top Gaps")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| priority | gap | surface | owner |")
	fmt.Fprintln(&b, "| ---: | --- | --- | --- |")
	gaps := topRows(ledger.Rows, func(row SurfaceLedgerRow) bool { return row.Bucket == BucketGap }, 25)
	for _, row := range gaps {
		fmt.Fprintf(&b, "| %d | %s | `%s` | %s |\n", row.Priority, row.GapClass, row.SurfaceID, row.Owner)
	}
	return b.String()
}

func GapsMarkdown(ledger SurfaceLedger) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Gaps")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| priority | gap | surface | source | next |")
	fmt.Fprintln(&b, "| ---: | --- | --- | --- | --- |")
	for _, row := range topRows(ledger.Rows, func(row SurfaceLedgerRow) bool { return row.Bucket == BucketGap }, 0) {
		fmt.Fprintf(&b, "| %d | %s | `%s` | %s | `glade compat surface explain --ledger SURFACE_LEDGER.json --id %s` |\n", row.Priority, row.GapClass, row.SurfaceID, row.DocsSource, row.SurfaceID)
	}
	return b.String()
}

func FailuresMarkdown(ledger SurfaceLedger) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Failures")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| failure | surface | notes |")
	fmt.Fprintln(&b, "| --- | --- | --- |")
	for _, row := range topRows(ledger.Rows, func(row SurfaceLedgerRow) bool { return row.Bucket == BucketFailure }, 0) {
		fmt.Fprintf(&b, "| %s | `%s` | %s |\n", row.GapClass, row.SurfaceID, row.Notes)
	}
	return b.String()
}

func ReleaseDiffMarkdown(oldLedger, newLedger SurfaceLedger) string {
	oldRows := map[string]SurfaceLedgerRow{}
	for _, row := range oldLedger.Rows {
		oldRows[row.SurfaceID] = row
	}
	var added, changed []SurfaceLedgerRow
	for _, row := range newLedger.Rows {
		old, ok := oldRows[row.SurfaceID]
		if !ok {
			added = append(added, row)
			continue
		}
		if old.Signature != row.Signature || old.Bucket != row.Bucket || old.GapClass != row.GapClass {
			changed = append(changed, row)
		}
		delete(oldRows, row.SurfaceID)
	}
	var removed []SurfaceLedgerRow
	for _, row := range oldRows {
		removed = append(removed, row)
	}
	sortRows(added)
	sortRows(changed)
	sortRows(removed)

	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Release Diff")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Added: %d\n", len(added))
	fmt.Fprintf(&b, "- Changed: %d\n", len(changed))
	fmt.Fprintf(&b, "- Removed: %d\n", len(removed))
	return b.String()
}

func ExplainMarkdown(ledger SurfaceLedger, id string) string {
	for _, row := range ledger.Rows {
		if row.SurfaceID != id {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", row.SurfaceID)
		fmt.Fprintf(&b, "- product: %s\n", row.Product)
		fmt.Fprintf(&b, "- docs: %s\n", row.Docs)
		fmt.Fprintf(&b, "- org: %s\n", row.Org)
		fmt.Fprintf(&b, "- gladeShape: %s\n", row.GladeShape)
		fmt.Fprintf(&b, "- gladeBehavior: %s\n", row.GladeBehavior)
		fmt.Fprintf(&b, "- evidence: %s\n", row.Evidence)
		fmt.Fprintf(&b, "- gapClass: %s\n", row.GapClass)
		fmt.Fprintf(&b, "- next: glade compat surface gaps --ledger SURFACE_LEDGER.json\n")
		return b.String()
	}
	return ""
}

type CheckOptions struct {
	MaxMissingShape    int
	MaxMissingBehavior int
	MaxParserFailures  int
}

func CheckLedger(ledger SurfaceLedger, options CheckOptions) error {
	if got := ledger.Summary.Gaps[GapMissingShape]; got > options.MaxMissingShape {
		return fmt.Errorf("missing-shape=%d exceeds max %d", got, options.MaxMissingShape)
	}
	if got := ledger.Summary.Gaps[GapMissingBehavior]; got > options.MaxMissingBehavior {
		return fmt.Errorf("missing-behavior=%d exceeds max %d", got, options.MaxMissingBehavior)
	}
	if got := ledger.Summary.Failures["parser"]; got > options.MaxParserFailures {
		return fmt.Errorf("parser=%d exceeds max %d", got, options.MaxParserFailures)
	}
	return nil
}

type namedCount struct {
	Name  string
	Count int
}

func ownerCounts(rows []SurfaceLedgerRow) []namedCount {
	counts := map[string]int{}
	for _, row := range rows {
		owner := row.Owner
		if owner == "" {
			owner = "unassigned"
		}
		counts[owner]++
	}
	return sortedCounts(counts)
}

func sortedCounts(counts map[string]int) []namedCount {
	out := make([]namedCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, namedCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func topRows(rows []SurfaceLedgerRow, keep func(SurfaceLedgerRow) bool, limit int) []SurfaceLedgerRow {
	var out []SurfaceLedgerRow
	for _, row := range rows {
		if keep(row) {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].SurfaceID < out[j].SurfaceID
		}
		return out[i].Priority < out[j].Priority
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func sumMap(values map[string]int) int {
	var total int
	for _, value := range values {
		total += value
	}
	return total
}
