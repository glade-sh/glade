package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexdocs"
)

// DocContract is a single behavioral constraint mined from Salesforce
// documentation prose, pinned to the symbol it governs. These are the
// "can't be used in test methods / treated as a callout / throws X"
// sentences that the runtime must honor. They turn doc prose into a
// checkable worklist so behavior like PageReference.getContentAsPDF is
// driven by the documented contract instead of a hand-patched VM branch.
type DocContract struct {
	Symbol     string `json:"symbol"`
	Namespace  string `json:"namespace,omitempty"`
	Type       string `json:"type,omitempty"`
	Member     string `json:"member,omitempty"`
	Behavior   string `json:"behavior"`
	Evidence   string `json:"evidence,omitempty"`
	DocsSource string `json:"docsSource,omitempty"`
}

// DocContractCount tallies contracts by behavior kind.
type DocContractCount struct {
	Behavior string `json:"behavior"`
	Count    int    `json:"count"`
}

// DocContractReport is the full set of mined contracts plus summary counts.
type DocContractReport struct {
	TotalDocuments    int                `json:"totalDocuments"`
	DocsWithContracts int                `json:"docsWithContracts"`
	TotalContracts    int                `json:"totalContracts"`
	ByBehavior        []DocContractCount `json:"byBehavior"`
	Contracts         []DocContract      `json:"contracts"`
}

// BuildDocContracts mines behavioral contracts from a parsed docs inventory.
func BuildDocContracts(inv apexdocs.Inventory) DocContractReport {
	report := DocContractReport{TotalDocuments: len(inv.Documents)}
	counts := map[string]int{}
	docsWith := map[string]bool{}

	for _, doc := range inv.Documents {
		if len(doc.Behaviors) == 0 {
			continue
		}
		ownerType := typeFromDocsSource(doc.SourcePath)
		member := ""
		typeName := ownerType
		// A per-method doc carries the method as Name and the owning type in
		// the path. A type-level doc carries the type as Name and no owner.
		if ownerType != "" && !strings.EqualFold(ownerType, doc.Name) {
			member = doc.Name
		} else {
			typeName = doc.Name
		}
		symbol := catalogSymbol(doc.Namespace, typeName, member)
		if symbol == "" {
			symbol = doc.Name
		}
		for _, behavior := range doc.Behaviors {
			report.Contracts = append(report.Contracts, DocContract{
				Symbol:     symbol,
				Namespace:  doc.Namespace,
				Type:       typeName,
				Member:     member,
				Behavior:   behavior.Kind,
				Evidence:   behavior.Evidence,
				DocsSource: doc.SourcePath,
			})
			counts[behavior.Kind]++
			docsWith[doc.SourcePath] = true
		}
	}

	sort.SliceStable(report.Contracts, func(i, j int) bool {
		a, b := report.Contracts[i], report.Contracts[j]
		if a.Behavior != b.Behavior {
			return a.Behavior < b.Behavior
		}
		if a.Symbol != b.Symbol {
			return a.Symbol < b.Symbol
		}
		return a.DocsSource < b.DocsSource
	})

	report.TotalContracts = len(report.Contracts)
	report.DocsWithContracts = len(docsWith)
	report.ByBehavior = sortedContractCounts(counts)
	return report
}

// FilterByBehavior returns a copy of the report holding only contracts of the
// given behavior kind. An empty kind returns the report unchanged.
func (r DocContractReport) FilterByBehavior(kind string) DocContractReport {
	if kind == "" {
		return r
	}
	out := DocContractReport{TotalDocuments: r.TotalDocuments}
	counts := map[string]int{}
	docsWith := map[string]bool{}
	for _, c := range r.Contracts {
		if c.Behavior != kind {
			continue
		}
		out.Contracts = append(out.Contracts, c)
		counts[c.Behavior]++
		docsWith[c.DocsSource] = true
	}
	out.TotalContracts = len(out.Contracts)
	out.DocsWithContracts = len(docsWith)
	out.ByBehavior = sortedContractCounts(counts)
	return out
}

func sortedContractCounts(counts map[string]int) []DocContractCount {
	out := make([]DocContractCount, 0, len(counts))
	for kind, n := range counts {
		out = append(out, DocContractCount{Behavior: kind, Count: n})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Behavior < out[j].Behavior
	})
	return out
}

// WriteDocContractsJSON writes the report as indented JSON.
func WriteDocContractsJSON(w io.Writer, report DocContractReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteDocContractsMarkdown writes a deterministic human-readable report.
func WriteDocContractsMarkdown(w io.Writer, report DocContractReport) error {
	var b strings.Builder
	b.WriteString("# Documented behavioral contracts\n\n")
	fmt.Fprintf(&b, "Documents scanned: %d\n\n", report.TotalDocuments)
	fmt.Fprintf(&b, "Documents with contracts: %d\n\n", report.DocsWithContracts)
	fmt.Fprintf(&b, "Total contracts: %d\n\n", report.TotalContracts)

	b.WriteString("## By behavior\n\n")
	b.WriteString("| Behavior | Count |\n| --- | ---: |\n")
	for _, c := range report.ByBehavior {
		fmt.Fprintf(&b, "| %s | %d |\n", c.Behavior, c.Count)
	}
	b.WriteString("\n## Contracts\n\n")
	b.WriteString("| Behavior | Symbol | Evidence | Source |\n| --- | --- | --- | --- |\n")
	for _, c := range report.Contracts {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			c.Behavior, c.Symbol, mdCell(c.Evidence), c.DocsSource)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}
