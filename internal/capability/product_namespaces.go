package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const ProductNamespaceSchemaVersion = 1

type ProductNamespaceReport struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Namespaces    []ProductNamespaceSummary `json:"namespaces"`
	Totals        ProductNamespaceTotals    `json:"totals"`
}

type ProductNamespaceTotals struct {
	Namespaces int `json:"namespaces"`
	Types      int `json:"types"`
	Members    int `json:"members"`
	Entries    int `json:"entries"`
	Inputs     int `json:"inputs"`
	Outputs    int `json:"outputs"`
}

type ProductNamespaceSummary struct {
	Namespace          string                 `json:"namespace"`
	Target             SupportTarget          `json:"target"`
	Status             Status                 `json:"status"`
	Owner              string                 `json:"owner"`
	DeclarationPolicy  string                 `json:"declarationPolicy"`
	ExecutionPolicy    string                 `json:"executionPolicy"`
	Types              []ProductNamespaceType `json:"types"`
	TypeCount          int                    `json:"typeCount"`
	MemberCount        int                    `json:"memberCount"`
	EntryCount         int                    `json:"entryCount"`
	InputCount         int                    `json:"inputCount,omitempty"`
	OutputCount        int                    `json:"outputCount,omitempty"`
	UnsupportedReasons []string               `json:"unsupportedReasons,omitempty"`
}

type ProductNamespaceType struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	MemberCount int    `json:"memberCount"`
	DocsSource  string `json:"docsSource,omitempty"`
}

func BuildProductNamespaceReport(catalog Catalog) ProductNamespaceReport {
	type bucket struct {
		summary ProductNamespaceSummary
		types   map[string]*ProductNamespaceType
	}
	buckets := map[string]*bucket{}
	for _, entry := range catalog.Entries {
		if entry.Area != "Product namespaces" || entry.Namespace == "" {
			continue
		}
		b := buckets[entry.Namespace]
		if b == nil {
			b = &bucket{
				summary: ProductNamespaceSummary{
					Namespace:         entry.Namespace,
					Target:            TargetTypedStub,
					Status:            StatusUnknown,
					Owner:             "generated declarations",
					DeclarationPolicy: "generate typed declarations from public docs inventory",
					ExecutionPolicy:   "return deterministic unsupported diagnostics until a local model is chosen",
				},
				types: map[string]*ProductNamespaceType{},
			}
			buckets[entry.Namespace] = b
		}
		b.summary.EntryCount++
		if entry.Target != TargetTypedStub {
			b.summary.UnsupportedReasons = appendUniqueString(b.summary.UnsupportedReasons, fmt.Sprintf("%s uses target %s", entry.Symbol, entry.Target))
		}
		if entry.Status != StatusUnknown && entry.Status != StatusStub && entry.Status != StatusUnsupported {
			b.summary.UnsupportedReasons = appendUniqueString(b.summary.UnsupportedReasons, fmt.Sprintf("%s has promoted status %s without namespace model", entry.Symbol, entry.Status))
		}
		if entry.MemberName != "" {
			b.summary.MemberCount++
			if typ := b.types[entry.TypeName]; typ != nil {
				typ.MemberCount++
			}
			continue
		}
		typ := b.types[entry.TypeName]
		if typ == nil {
			typ = &ProductNamespaceType{Name: entry.TypeName, Kind: entry.Kind, DocsSource: entry.DocsSource}
			b.types[entry.TypeName] = typ
		}
		switch strings.ToLower(entry.Kind) {
		case "input":
			b.summary.InputCount++
		case "output":
			b.summary.OutputCount++
		}
	}

	namespaces := make([]ProductNamespaceSummary, 0, len(buckets))
	for _, b := range buckets {
		types := make([]ProductNamespaceType, 0, len(b.types))
		for _, typ := range b.types {
			types = append(types, *typ)
		}
		sort.Slice(types, func(i, j int) bool {
			if types[i].Name != types[j].Name {
				return types[i].Name < types[j].Name
			}
			return types[i].Kind < types[j].Kind
		})
		b.summary.Types = types
		b.summary.TypeCount = len(types)
		sort.Strings(b.summary.UnsupportedReasons)
		namespaces = append(namespaces, b.summary)
	}
	sort.Slice(namespaces, func(i, j int) bool {
		if namespaces[i].EntryCount != namespaces[j].EntryCount {
			return namespaces[i].EntryCount > namespaces[j].EntryCount
		}
		return namespaces[i].Namespace < namespaces[j].Namespace
	})

	report := ProductNamespaceReport{SchemaVersion: ProductNamespaceSchemaVersion, Namespaces: namespaces}
	for _, ns := range namespaces {
		report.Totals.Namespaces++
		report.Totals.Types += ns.TypeCount
		report.Totals.Members += ns.MemberCount
		report.Totals.Entries += ns.EntryCount
		report.Totals.Inputs += ns.InputCount
		report.Totals.Outputs += ns.OutputCount
	}
	return report
}

func WriteProductNamespaceJSON(w io.Writer, report ProductNamespaceReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteProductNamespaceText(w io.Writer, report ProductNamespaceReport) error {
	fmt.Fprintf(w, "schemaVersion: %d\n", report.SchemaVersion)
	fmt.Fprintf(w, "namespaces: %d\n", report.Totals.Namespaces)
	fmt.Fprintf(w, "types: %d\n", report.Totals.Types)
	fmt.Fprintf(w, "members: %d\n", report.Totals.Members)
	fmt.Fprintf(w, "entries: %d\n", report.Totals.Entries)
	if len(report.Namespaces) == 0 {
		return nil
	}
	fmt.Fprintln(w, "namespace summary:")
	for _, ns := range report.Namespaces {
		fmt.Fprintf(w, "  %s: target=%s status=%s types=%d members=%d entries=%d", ns.Namespace, ns.Target, ns.Status, ns.TypeCount, ns.MemberCount, ns.EntryCount)
		if ns.InputCount > 0 {
			fmt.Fprintf(w, " inputs=%d", ns.InputCount)
		}
		if ns.OutputCount > 0 {
			fmt.Fprintf(w, " outputs=%d", ns.OutputCount)
		}
		if len(ns.UnsupportedReasons) > 0 {
			fmt.Fprintf(w, " issues=%d", len(ns.UnsupportedReasons))
		}
		fmt.Fprintln(w)
	}
	return nil
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
