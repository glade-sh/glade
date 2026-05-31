package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/typesys"
)

const ReconciliationSchemaVersion = 1

// DerivedStatus is the runtime-derived verdict for a documented Salesforce
// surface. Unlike the hand-maintained catalog Status, it is computed by
// cross-referencing the docs catalog against what the glade runtime actually
// knows: the executable stdlib verdict layer (StdlibMatrix) and the type-known
// symbol table (typesys.StandardPlatformSymbols).
type DerivedStatus string

const (
	// DerivedSupported and DerivedPartial carry a hand-verified executable
	// verdict from StdlibMatrix.
	DerivedSupported DerivedStatus = "supported"
	DerivedPartial   DerivedStatus = "partial"
	// DerivedUnsupported is an intentional runtime rejection (stable
	// diagnostic), as opposed to a documentation surface.
	DerivedUnsupported DerivedStatus = "unsupported"
	// DerivedTyped means the owning type is type-known (Apex referencing it
	// compiles) but there is no executable verdict yet.
	DerivedTyped DerivedStatus = "typed"
	// DerivedUnknown means the owning type is not type-known: Apex that
	// references it will not even resolve. This is the highest-value gap.
	DerivedUnknown DerivedStatus = "unknown"
	// DerivedDoc is a language or guide surface, not a runtime target.
	DerivedDoc DerivedStatus = "doc"
)

// Reconciliation reports, for every documented surface in the catalog, a
// runtime-derived status and a prioritized worklist of gaps.
type Reconciliation struct {
	SchemaVersion   int                    `json:"schemaVersion"`
	SourceDocuments int                    `json:"sourceDocuments"`
	SourceMembers   int                    `json:"sourceMembers"`
	TotalEntries    int                    `json:"totalEntries"`
	RuntimeTargets  RuntimeTargetCoverage  `json:"runtimeTargets"`
	ByStatus        []ReconcileStatusCount `json:"byStatus"`
	ByArea          []ReconcileAreaCount   `json:"byArea"`
	Worklist        []ReconcileWorkItem    `json:"worklist"`
	WorklistTotals  []ReconcileStatusCount `json:"worklistTotals"`
}

// RuntimeTargetCoverage summarizes the executable-parity and local-model
// surfaces (the areas glade intends to run, not just type), which is the
// headline number for roadmap progress.
type RuntimeTargetCoverage struct {
	Total       int     `json:"total"`
	Supported   int     `json:"supported"`
	Partial     int     `json:"partial"`
	Unsupported int     `json:"unsupported"`
	Typed       int     `json:"typed"`
	Unknown     int     `json:"unknown"`
	CoveragePct float64 `json:"coveragePct"`
}

type ReconcileStatusCount struct {
	Status DerivedStatus `json:"status"`
	Count  int           `json:"count"`
}

type ReconcileAreaCount struct {
	Area      string                 `json:"area"`
	Total     int                    `json:"total"`
	Breakdown []ReconcileStatusCount `json:"breakdown"`
}

// ReconcileWorkItem is a single documented surface that needs runtime work,
// sorted so the most impactful gaps come first.
type ReconcileWorkItem struct {
	Symbol     string        `json:"symbol"`
	Area       string        `json:"area"`
	Target     SupportTarget `json:"target"`
	Kind       string        `json:"kind"`
	Status     DerivedStatus `json:"status"`
	OwnerType  string        `json:"ownerType,omitempty"`
	Owner      string        `json:"owner,omitempty"`
	DocsSource string        `json:"docsSource,omitempty"`
}

// The default cap on worklist entries written to the report so the artifact
// stays reviewable. The totals are always reported in full.
const defaultReconcileWorklistLimit = 250

// BuildReconciliation derives a runtime status for every catalog entry by
// layering the type-known symbol table on top of the catalog's executable
// verdicts.
func BuildReconciliation(cat Catalog, platform []typesys.TypeSymbol) Reconciliation {
	return BuildReconciliationLimited(cat, platform, defaultReconcileWorklistLimit)
}

// BuildReconciliationLimited is BuildReconciliation with an explicit worklist
// cap (0 means unlimited).
func BuildReconciliationLimited(cat Catalog, platform []typesys.TypeSymbol, worklistLimit int) Reconciliation {
	known := buildTypeKnownIndex(platform)

	rec := Reconciliation{
		SchemaVersion:   ReconciliationSchemaVersion,
		SourceDocuments: cat.SourceDocuments,
		SourceMembers:   cat.SourceMembers,
		TotalEntries:    len(cat.Entries),
	}

	statusCounts := map[DerivedStatus]int{}
	areaCounts := map[string]map[DerivedStatus]int{}
	worklistTotals := map[DerivedStatus]int{}
	var work []ReconcileWorkItem

	for _, entry := range cat.Entries {
		ownerType, typeKnown := resolveTypeKnown(entry, known)
		status := deriveStatus(entry, typeKnown)

		statusCounts[status]++
		if areaCounts[entry.Area] == nil {
			areaCounts[entry.Area] = map[DerivedStatus]int{}
		}
		areaCounts[entry.Area][status]++

		if isRuntimeTarget(entry.Target) {
			rec.RuntimeTargets.Total++
			switch status {
			case DerivedSupported:
				rec.RuntimeTargets.Supported++
			case DerivedPartial:
				rec.RuntimeTargets.Partial++
			case DerivedUnsupported:
				rec.RuntimeTargets.Unsupported++
			case DerivedTyped:
				rec.RuntimeTargets.Typed++
			case DerivedUnknown:
				rec.RuntimeTargets.Unknown++
			}
		}

		if isWorklistStatus(status) {
			worklistTotals[status]++
			work = append(work, ReconcileWorkItem{
				Symbol:     entry.Symbol,
				Area:       entry.Area,
				Target:     entry.Target,
				Kind:       entry.Kind,
				Status:     status,
				OwnerType:  ownerType,
				Owner:      entry.Owner,
				DocsSource: entry.DocsSource,
			})
		}
	}

	if rec.RuntimeTargets.Total > 0 {
		covered := rec.RuntimeTargets.Supported + rec.RuntimeTargets.Partial
		rec.RuntimeTargets.CoveragePct = round2(float64(covered) * 100 / float64(rec.RuntimeTargets.Total))
	}

	rec.ByStatus = sortedStatusCounts(statusCounts)
	rec.ByArea = sortedAreaCounts(areaCounts)
	rec.WorklistTotals = sortedStatusCounts(worklistTotals)

	sortWorklist(work)
	if worklistLimit > 0 && len(work) > worklistLimit {
		work = work[:worklistLimit]
	}
	rec.Worklist = work
	return rec
}

// typeKnownIndex holds the set of lowercased type identities that the runtime
// symbol table can resolve.
type typeKnownIndex struct {
	types map[string]bool
}

func buildTypeKnownIndex(platform []typesys.TypeSymbol) typeKnownIndex {
	idx := typeKnownIndex{types: map[string]bool{}}
	for _, sym := range platform {
		name := strings.ToLower(strings.TrimSpace(sym.Name))
		if name == "" {
			continue
		}
		idx.types[name] = true
		ns := strings.ToLower(strings.TrimSpace(sym.Namespace))
		if ns != "" {
			idx.types[ns+"."+name] = true
		}
	}
	return idx
}

func (idx typeKnownIndex) has(candidate string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if candidate == "" {
		return false
	}
	return idx.types[candidate]
}

// resolveTypeKnown reports whether the type that owns the catalog entry is
// type-known, returning the recovered owner type name for the worklist.
func resolveTypeKnown(entry CatalogEntry, known typeKnownIndex) (ownerType string, typeKnown bool) {
	for _, cand := range ownerTypeCandidates(entry) {
		if cand == "" {
			continue
		}
		if ownerType == "" {
			ownerType = cand
		}
		if known.has(cand) {
			return cand, true
		}
		ns := canonicalNamespace(entry.Namespace)
		if ns != "" && ns != "System" && known.has(ns+"."+cand) {
			return cand, true
		}
	}
	return ownerType, false
}

// ownerTypeCandidates recovers the owning Apex type for a documented surface.
// The scraped docs are one file per page, so per-method pages carry the method
// name as the doc TypeName; the owning type is recovered from the doc source
// path (e.g. apex_System_PageReference_getContentAsPDF.md -> PageReference).
func ownerTypeCandidates(entry CatalogEntry) []string {
	candidates := make([]string, 0, 3)
	if fromPath := typeFromDocsSource(entry.DocsSource); fromPath != "" {
		candidates = append(candidates, fromPath)
	}
	if entry.TypeName != "" {
		candidates = append(candidates, canonicalName(entry.TypeName))
	}
	return candidates
}

var docKindTokens = map[string]bool{
	"class":      true,
	"classes":    true,
	"interface":  true,
	"interfaces": true,
	"enum":       true,
	"enums":      true,
	"methods":    true,
	"method":     true,
	"properties": true,
	"property":   true,
	"namespace":  true,
	"exceptions": true,
	"exception":  true,
	"dml":        true,
}

func typeFromDocsSource(source string) string {
	if source == "" {
		return ""
	}
	base := path.Base(source)
	base = strings.TrimSuffix(base, path.Ext(base))
	tokens := strings.Split(base, "_")
	cleaned := make([]string, 0, len(tokens))
	for i, tok := range tokens {
		if i == 0 && strings.EqualFold(tok, "apex") {
			continue
		}
		if docKindTokens[strings.ToLower(tok)] {
			continue
		}
		if tok == "" {
			continue
		}
		cleaned = append(cleaned, tok)
	}
	if len(cleaned) == 0 {
		return ""
	}
	if len(cleaned) == 1 {
		// Namespace-only doc (e.g. apex_namespace_System); not a concrete type.
		return ""
	}
	// cleaned[0] is the namespace; cleaned[1] is the owning type.
	return cleaned[1]
}

func deriveStatus(entry CatalogEntry, typeKnown bool) DerivedStatus {
	if entry.Target == TargetUnsupported {
		return DerivedDoc
	}
	switch entry.Status {
	case StatusSupported:
		return DerivedSupported
	case StatusPartial:
		return DerivedPartial
	case StatusUnsupported:
		return DerivedUnsupported
	case StatusStub:
		return DerivedTyped
	}
	if typeKnown {
		return DerivedTyped
	}
	return DerivedUnknown
}

func isRuntimeTarget(target SupportTarget) bool {
	return target == TargetExecutableParity || target == TargetLocalModel
}

func isWorklistStatus(status DerivedStatus) bool {
	return status == DerivedUnknown || status == DerivedTyped
}

// worklistTargetRank orders surfaces so executable-parity gaps come before
// local-model, then typed-stub product namespaces.
func worklistTargetRank(target SupportTarget) int {
	switch target {
	case TargetExecutableParity:
		return 0
	case TargetLocalModel:
		return 1
	case TargetTypedStub:
		return 2
	default:
		return 3
	}
}

// worklistStatusRank orders unknown (won't compile) before typed (compiles, no
// verdict).
func worklistStatusRank(status DerivedStatus) int {
	if status == DerivedUnknown {
		return 0
	}
	return 1
}

func sortWorklist(work []ReconcileWorkItem) {
	sort.SliceStable(work, func(i, j int) bool {
		if r := worklistTargetRank(work[i].Target) - worklistTargetRank(work[j].Target); r != 0 {
			return r < 0
		}
		if r := worklistStatusRank(work[i].Status) - worklistStatusRank(work[j].Status); r != 0 {
			return r < 0
		}
		if work[i].Area != work[j].Area {
			return work[i].Area < work[j].Area
		}
		return work[i].Symbol < work[j].Symbol
	})
}

var derivedStatusOrder = []DerivedStatus{
	DerivedSupported, DerivedPartial, DerivedUnsupported, DerivedTyped, DerivedUnknown, DerivedDoc,
}

func sortedStatusCounts(counts map[DerivedStatus]int) []ReconcileStatusCount {
	out := make([]ReconcileStatusCount, 0, len(counts))
	for _, status := range derivedStatusOrder {
		if n, ok := counts[status]; ok {
			out = append(out, ReconcileStatusCount{Status: status, Count: n})
		}
	}
	return out
}

func sortedAreaCounts(areas map[string]map[DerivedStatus]int) []ReconcileAreaCount {
	out := make([]ReconcileAreaCount, 0, len(areas))
	for area, counts := range areas {
		total := 0
		for _, n := range counts {
			total += n
		}
		out = append(out, ReconcileAreaCount{
			Area:      area,
			Total:     total,
			Breakdown: sortedStatusCounts(counts),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Area < out[j].Area })
	return out
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// RuntimeTargetUnknownCount returns the number of runtime-target surfaces whose
// owning type is not type-known. This is the ratchet metric for the docs
// support gate: it must not grow.
func (rec Reconciliation) RuntimeTargetUnknownCount() int {
	return rec.RuntimeTargets.Unknown
}

func WriteReconciliationJSON(w io.Writer, rec Reconciliation) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rec)
}

func WriteReconciliationMarkdown(w io.Writer, rec Reconciliation) error {
	fmt.Fprintln(w, "# Runtime Reconciliation")
	fmt.Fprintln(w, "\nGenerated by `glade compat reconcile`. Cross-references the documented")
	fmt.Fprintln(w, "Salesforce surface (catalog) against the glade runtime: the executable")
	fmt.Fprintln(w, "stdlib verdict layer and the type-known symbol table.")
	fmt.Fprintln(w, "\nDerived status values:")
	fmt.Fprintln(w, "\n- `supported`/`partial`: hand-verified executable behavior.")
	fmt.Fprintln(w, "- `unsupported`: intentional runtime rejection with a stable diagnostic.")
	fmt.Fprintln(w, "- `typed`: owning type is type-known (compiles) but no executable verdict.")
	fmt.Fprintln(w, "- `unknown`: owning type is not type-known; references will not resolve.")
	fmt.Fprintln(w, "- `doc`: language or guide surface, not a runtime target.")

	fmt.Fprintf(w, "\nSource: %d documents, %d members, %d catalog entries.\n",
		rec.SourceDocuments, rec.SourceMembers, rec.TotalEntries)

	rt := rec.RuntimeTargets
	fmt.Fprintln(w, "\n## Runtime target coverage")
	fmt.Fprintln(w, "\nExecutable-parity and local-model surfaces (the areas glade intends to run).")
	fmt.Fprintln(w, "\n| Metric | Count |")
	fmt.Fprintln(w, "| --- | --- |")
	fmt.Fprintf(w, "| Total | %d |\n", rt.Total)
	fmt.Fprintf(w, "| Supported | %d |\n", rt.Supported)
	fmt.Fprintf(w, "| Partial | %d |\n", rt.Partial)
	fmt.Fprintf(w, "| Unsupported | %d |\n", rt.Unsupported)
	fmt.Fprintf(w, "| Typed (no verdict) | %d |\n", rt.Typed)
	fmt.Fprintf(w, "| Unknown (not type-known) | %d |\n", rt.Unknown)
	fmt.Fprintf(w, "| Coverage %% | %.2f |\n", rt.CoveragePct)

	fmt.Fprintln(w, "\n## By status")
	fmt.Fprintln(w, "\n| Status | Count |")
	fmt.Fprintln(w, "| --- | --- |")
	for _, s := range rec.ByStatus {
		fmt.Fprintf(w, "| `%s` | %d |\n", s.Status, s.Count)
	}

	fmt.Fprintln(w, "\n## By area")
	fmt.Fprintln(w, "\n| Area | Total | Breakdown |")
	fmt.Fprintln(w, "| --- | --- | --- |")
	for _, a := range rec.ByArea {
		parts := make([]string, 0, len(a.Breakdown))
		for _, b := range a.Breakdown {
			parts = append(parts, fmt.Sprintf("%s=%d", b.Status, b.Count))
		}
		fmt.Fprintf(w, "| %s | %d | %s |\n", a.Area, a.Total, strings.Join(parts, ", "))
	}

	fmt.Fprintln(w, "\n## Worklist")
	worklistTotal := 0
	for _, t := range rec.WorklistTotals {
		worklistTotal += t.Count
	}
	parts := make([]string, 0, len(rec.WorklistTotals))
	for _, t := range rec.WorklistTotals {
		parts = append(parts, fmt.Sprintf("%s=%d", t.Status, t.Count))
	}
	fmt.Fprintf(w, "\n%d surfaces need runtime work (%s). Showing %d, highest impact first.\n",
		worklistTotal, strings.Join(parts, ", "), len(rec.Worklist))
	fmt.Fprintln(w, "\n| Symbol | Area | Target | Owner type | Status |")
	fmt.Fprintln(w, "| --- | --- | --- | --- | --- |")
	for _, item := range rec.Worklist {
		fmt.Fprintf(w, "| `%s` | %s | %s | %s | `%s` |\n",
			item.Symbol, item.Area, item.Target, item.OwnerType, item.Status)
	}
	return nil
}
