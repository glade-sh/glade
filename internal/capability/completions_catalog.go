package capability

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexdocs"
)

// BuildCatalogFromCompletions builds a capability catalog from the org's
// Tooling API Apex completions. The scraped reference guide covers a curated
// member list per documented type; the completions cover the full breadth of
// types and namespaces the org compiler knows about, including product
// namespaces the docs omit. Folding completions through the same classifier
// gives the reconcile/oracle loop a complete worklist of the real surface.
func BuildCatalogFromCompletions(tooling ToolingCompletions) Catalog {
	return BuildCatalog(InventoryFromToolingCompletions(tooling))
}

// InventoryFromToolingCompletions converts the completions catalog into an
// apexdocs.Inventory so it flows through BuildCatalog unchanged. Each type
// becomes a document; each method, constructor, and property becomes a member
// with a signature reconstructed from the org-reported types.
func InventoryFromToolingCompletions(tooling ToolingCompletions) apexdocs.Inventory {
	nsNames := make([]string, 0, len(tooling.PublicDeclarations))
	for ns := range tooling.PublicDeclarations {
		nsNames = append(nsNames, ns)
	}
	sort.Strings(nsNames)

	docs := make([]apexdocs.Document, 0, len(tooling.PublicDeclarations))
	namespaces := make([]apexdocs.NamespaceSummary, 0, len(tooling.PublicDeclarations))
	totalMembers := 0

	for _, ns := range nsNames {
		decls := tooling.PublicDeclarations[ns]
		typeNames := make([]string, 0, len(decls))
		for tn := range decls {
			typeNames = append(typeNames, tn)
		}
		sort.Strings(typeNames)

		nsMembers := 0
		for _, tn := range typeNames {
			decl := decls[tn]
			members := make([]apexdocs.Member, 0, len(decl.Constructors)+len(decl.Methods)+len(decl.Properties))
			for _, ctor := range decl.Constructors {
				members = append(members, apexdocs.Member{
					Kind:      "constructor",
					Name:      ctor.Name,
					Signature: completionsConstructorSignature(ctor),
				})
			}
			for _, method := range decl.Methods {
				members = append(members, apexdocs.Member{
					Kind:      "method",
					Name:      method.Name,
					Signature: completionsMethodSignature(method),
				})
			}
			for _, prop := range decl.Properties {
				members = append(members, apexdocs.Member{
					Kind:      "property",
					Name:      prop.Name,
					Signature: completionsPropertySignature(prop),
				})
			}
			sort.SliceStable(members, func(i, j int) bool {
				if members[i].Name != members[j].Name {
					return members[i].Name < members[j].Name
				}
				return members[i].Signature < members[j].Signature
			})
			docs = append(docs, apexdocs.Document{
				SourcePath: completionsSource(ns, tn),
				Kind:       "class",
				Namespace:  ns,
				Name:       tn,
				Members:    members,
			})
			nsMembers += len(members)
		}
		totalMembers += nsMembers
		namespaces = append(namespaces, apexdocs.NamespaceSummary{
			Namespace: ns,
			Documents: len(typeNames),
			Classes:   len(typeNames),
			Members:   nsMembers,
		})
	}

	return apexdocs.Inventory{
		SchemaVersion: apexdocs.InventorySchemaVersion,
		TotalFiles:    len(docs),
		TotalMembers:  totalMembers,
		Namespaces:    namespaces,
		Documents:     docs,
	}
}

func completionsMethodSignature(m ToolingMethod) string {
	return m.Name + "(" + completionsParams(m.Parameters) + ")" + completionsReturns(m.ReturnType)
}

func completionsConstructorSignature(c ToolingConstructor) string {
	return c.Name + "(" + completionsParams(c.Parameters) + ")"
}

func completionsPropertySignature(p ToolingProperty) string {
	if p.Type != "" {
		return p.Name + " : " + p.Type
	}
	return p.Name
}

func completionsParams(params []ToolingParameter) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		switch {
		case p.Type != "" && p.Name != "":
			parts = append(parts, p.Type+" "+p.Name)
		case p.Type != "":
			parts = append(parts, p.Type)
		case p.Name != "":
			parts = append(parts, p.Name)
		}
	}
	return strings.Join(parts, ", ")
}

func completionsReturns(returnType string) string {
	if returnType == "" {
		return ""
	}
	return " returns " + returnType
}

func completionsSource(ns, typeName string) string {
	if ns == "" {
		return "completions://" + typeName
	}
	return "completions://" + ns + "/" + typeName
}
