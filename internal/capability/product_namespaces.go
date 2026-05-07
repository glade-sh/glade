package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/typesys"
)

const ProductNamespaceSchemaVersion = 1

type ProductNamespaceReport struct {
	SchemaVersion       int                                 `json:"schemaVersion"`
	Namespaces          []ProductNamespaceSummary           `json:"namespaces"`
	Totals              ProductNamespaceTotals              `json:"totals"`
	DeclarationCoverage ProductNamespaceDeclarationCoverage `json:"declarationCoverage"`
}

type ProductNamespaceTotals struct {
	Namespaces int `json:"namespaces"`
	Types      int `json:"types"`
	Members    int `json:"members"`
	Entries    int `json:"entries"`
	Inputs     int `json:"inputs"`
	Outputs    int `json:"outputs"`
}

type ProductNamespaceDeclarationCoverage struct {
	NamespacesWithDeclarations int `json:"namespacesWithDeclarations"`
	TypesWithDeclarations      int `json:"typesWithDeclarations"`
	TypesMissingDeclarations   int `json:"typesMissingDeclarations"`
	MembersWithDeclarations    int `json:"membersWithDeclarations"`
	MembersMissingDeclarations int `json:"membersMissingDeclarations"`
	EntriesWithDeclarations    int `json:"entriesWithDeclarations"`
	EntriesMissingDeclarations int `json:"entriesMissingDeclarations"`
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
	DeclarationStatus  string                 `json:"declarationStatus"`
	DeclaredTypes      int                    `json:"declaredTypes"`
	MissingTypes       int                    `json:"missingTypes"`
	DeclaredMembers    int                    `json:"declaredMembers"`
	MissingMembers     int                    `json:"missingMembers"`
	DeclaredEntries    int                    `json:"declaredEntries"`
	MissingEntries     int                    `json:"missingEntries"`
	UnsupportedReasons []string               `json:"unsupportedReasons,omitempty"`
}

type ProductNamespaceType struct {
	Name                  string                   `json:"name"`
	Kind                  string                   `json:"kind"`
	MemberCount           int                      `json:"memberCount"`
	DocsSource            string                   `json:"docsSource,omitempty"`
	HasTypedDeclaration   bool                     `json:"hasTypedDeclaration"`
	DeclaredMemberCount   int                      `json:"declaredMemberCount"`
	MissingMemberCount    int                      `json:"missingMemberCount,omitempty"`
	TypedDeclarationKinds []string                 `json:"typedDeclarationKinds,omitempty"`
	Members               []ProductNamespaceMember `json:"members,omitempty"`
}

type ProductNamespaceMember struct {
	Name                string `json:"name"`
	Kind                string `json:"kind"`
	Signature           string `json:"signature,omitempty"`
	HasTypedDeclaration bool   `json:"hasTypedDeclaration"`
}

type ProductNamespaceDeclarations struct {
	Types map[string]ProductNamespaceDeclaredType
}

type ProductNamespaceDeclaredType struct {
	Namespace string
	Name      string
	Kind      string
	Members   map[string][]string
}

func BuildProductNamespaceReport(catalog Catalog) ProductNamespaceReport {
	return BuildProductNamespaceReportWithDeclarations(catalog, ProductNamespaceDeclarationsFromStandardSymbols())
}

func BuildProductNamespaceReportWithDeclarations(catalog Catalog, declarations ProductNamespaceDeclarations) ProductNamespaceReport {
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
		typeKey := productNamespaceDeclarationTypeKey(entry.Namespace, entry.TypeName)
		if entry.Target != TargetTypedStub {
			b.summary.UnsupportedReasons = appendUniqueString(b.summary.UnsupportedReasons, fmt.Sprintf("%s uses target %s", entry.Symbol, entry.Target))
		}
		if entry.Status != StatusUnknown && entry.Status != StatusStub && entry.Status != StatusUnsupported {
			b.summary.UnsupportedReasons = appendUniqueString(b.summary.UnsupportedReasons, fmt.Sprintf("%s has promoted status %s without namespace model", entry.Symbol, entry.Status))
		}
		typ := b.types[entry.TypeName]
		if typ == nil {
			typ = &ProductNamespaceType{Name: entry.TypeName, Kind: entry.Kind, DocsSource: entry.DocsSource}
			if decl, ok := declarations.Types[typeKey]; ok {
				typ.HasTypedDeclaration = true
				if decl.Kind != "" {
					typ.TypedDeclarationKinds = appendUniqueString(typ.TypedDeclarationKinds, decl.Kind)
				}
			}
			b.types[entry.TypeName] = typ
		}
		if entry.MemberName != "" {
			b.summary.MemberCount++
			typ.MemberCount++
			memberDeclared := false
			if decl, ok := declarations.Types[typeKey]; ok {
				_, memberDeclared = decl.Members[productNamespaceDeclarationMemberKey(entry.MemberName)]
			}
			if memberDeclared {
				typ.DeclaredMemberCount++
			} else {
				typ.MissingMemberCount++
			}
			typ.Members = append(typ.Members, ProductNamespaceMember{
				Name:                entry.MemberName,
				Kind:                entry.Kind,
				Signature:           entry.Signature,
				HasTypedDeclaration: memberDeclared,
			})
			if memberDeclared {
				b.summary.DeclaredEntries++
			} else {
				b.summary.MissingEntries++
			}
			continue
		}
		if typ.HasTypedDeclaration {
			b.summary.DeclaredEntries++
		} else {
			b.summary.MissingEntries++
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
			sort.Slice(typ.Members, func(i, j int) bool {
				if typ.Members[i].Name != typ.Members[j].Name {
					return typ.Members[i].Name < typ.Members[j].Name
				}
				if typ.Members[i].Kind != typ.Members[j].Kind {
					return typ.Members[i].Kind < typ.Members[j].Kind
				}
				return typ.Members[i].Signature < typ.Members[j].Signature
			})
			sort.Strings(typ.TypedDeclarationKinds)
			if typ.HasTypedDeclaration {
				b.summary.DeclaredTypes++
			} else {
				b.summary.MissingTypes++
			}
			b.summary.DeclaredMembers += typ.DeclaredMemberCount
			b.summary.MissingMembers += typ.MissingMemberCount
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
		b.summary.DeclarationStatus = productNamespaceDeclarationStatus(b.summary.DeclaredEntries, b.summary.MissingEntries)
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
		if ns.DeclaredEntries > 0 {
			report.DeclarationCoverage.NamespacesWithDeclarations++
		}
		report.DeclarationCoverage.TypesWithDeclarations += ns.DeclaredTypes
		report.DeclarationCoverage.TypesMissingDeclarations += ns.MissingTypes
		report.DeclarationCoverage.MembersWithDeclarations += ns.DeclaredMembers
		report.DeclarationCoverage.MembersMissingDeclarations += ns.MissingMembers
		report.DeclarationCoverage.EntriesWithDeclarations += ns.DeclaredEntries
		report.DeclarationCoverage.EntriesMissingDeclarations += ns.MissingEntries
	}
	return report
}

func ProductNamespaceDeclarationsFromStandardSymbols() ProductNamespaceDeclarations {
	out := ProductNamespaceDeclarations{Types: map[string]ProductNamespaceDeclaredType{}}
	for _, symbol := range typesys.StandardPlatformSymbols() {
		if symbol.Namespace == "" {
			continue
		}
		key := productNamespaceDeclarationTypeKey(symbol.Namespace, symbol.Name)
		decl := ProductNamespaceDeclaredType{
			Namespace: symbol.Namespace,
			Name:      symbol.Name,
			Kind:      string(symbol.Kind),
			Members:   map[string][]string{},
		}
		for _, member := range symbol.Members {
			memberKey := productNamespaceDeclarationMemberKey(member.Name)
			if memberKey == "" {
				continue
			}
			decl.Members[memberKey] = appendUniqueString(decl.Members[memberKey], string(member.Kind))
		}
		out.Types[key] = decl
	}
	return out
}

func WriteProductNamespaceJSON(w io.Writer, report ProductNamespaceReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteProductNamespaceMarkdown(w io.Writer, report ProductNamespaceReport) error {
	if _, err := fmt.Fprintln(w, "# Product Namespace Coverage"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\nGenerated from the Salesforce Apex docs inventory and capability catalog."); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n- Namespaces: %d\n", report.Totals.Namespaces); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Types: %d\n", report.Totals.Types); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Members: %d\n", report.Totals.Members); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Entries: %d\n", report.Totals.Entries); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Input DTO types: %d\n", report.Totals.Inputs); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Output DTO types: %d\n", report.Totals.Outputs); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Typed declaration entries: %d\n", report.DeclarationCoverage.EntriesWithDeclarations); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Missing typed declaration entries: %d\n", report.DeclarationCoverage.EntriesMissingDeclarations); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\n`Target` and `Status` describe runtime capability classification. Declaration columns describe generated type/member availability only."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\n| Namespace | Target | Status | Declaration status | Types | Declared types | Members | Declared members | Missing members | Entries | Declared entries | Inputs | Outputs |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |"); err != nil {
		return err
	}
	for _, ns := range report.Namespaces {
		if _, err := fmt.Fprintf(w, "| %s | `%s` | `%s` | `%s` | %d | %d | %d | %d | %d | %d | %d | %d | %d |\n", ns.Namespace, ns.Target, ns.Status, ns.DeclarationStatus, ns.TypeCount, ns.DeclaredTypes, ns.MemberCount, ns.DeclaredMembers, ns.MissingMembers, ns.EntryCount, ns.DeclaredEntries, ns.InputCount, ns.OutputCount); err != nil {
			return err
		}
	}
	return nil
}

func WriteProductNamespaceText(w io.Writer, report ProductNamespaceReport) error {
	fmt.Fprintf(w, "schemaVersion: %d\n", report.SchemaVersion)
	fmt.Fprintf(w, "namespaces: %d\n", report.Totals.Namespaces)
	fmt.Fprintf(w, "types: %d\n", report.Totals.Types)
	fmt.Fprintf(w, "members: %d\n", report.Totals.Members)
	fmt.Fprintf(w, "entries: %d\n", report.Totals.Entries)
	fmt.Fprintf(w, "typedDeclarationEntries: %d\n", report.DeclarationCoverage.EntriesWithDeclarations)
	fmt.Fprintf(w, "missingTypedDeclarationEntries: %d\n", report.DeclarationCoverage.EntriesMissingDeclarations)
	if len(report.Namespaces) == 0 {
		return nil
	}
	fmt.Fprintln(w, "namespace summary:")
	for _, ns := range report.Namespaces {
		fmt.Fprintf(w, "  %s: target=%s status=%s declarationStatus=%s types=%d declaredTypes=%d members=%d declaredMembers=%d missingMembers=%d entries=%d declaredEntries=%d", ns.Namespace, ns.Target, ns.Status, ns.DeclarationStatus, ns.TypeCount, ns.DeclaredTypes, ns.MemberCount, ns.DeclaredMembers, ns.MissingMembers, ns.EntryCount, ns.DeclaredEntries)
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

func productNamespaceDeclarationStatus(declared, missing int) string {
	switch {
	case declared == 0 && missing == 0:
		return "empty"
	case missing == 0:
		return "complete"
	case declared == 0:
		return "missing"
	default:
		return "partial"
	}
}

func productNamespaceDeclarationTypeKey(namespace, name string) string {
	return strings.ToLower(strings.TrimSpace(namespace) + "." + strings.TrimSpace(name))
}

func productNamespaceDeclarationMemberKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
