package apextest

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// applyApexTypeImplementorRecords retains the direct and public-inner enumeration
// covered by the owned concrete/nested fixtures, including public test-class
// inners. The full API67 helper payload fixtures also observe null class and
// interface namespaces. This runs once after standard object initialization and
// uses the org-aware generator to preserve existing metadata record IDs.
func applyApexTypeImplementorRecords(org *storage.OrgState, index typesys.Index) {
	if org == nil {
		return
	}
	state, ok := org.Objects["ApexTypeImplementor"]
	if !ok {
		return
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}

	generator := storage.NewRuntimeIDGeneratorForOrg(org)
	seen := make(map[string]struct{})
	sourceTypes := make(map[string]typesys.TypeSymbol, len(index.Types))
	for _, candidate := range index.Types {
		if candidate.Kind == apexast.DeclarationClass && candidate.SourceBacked && !candidate.Dependency && candidate.Namespace == "" {
			sourceTypes[candidate.Name] = candidate
		}
	}
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass || !typ.SourceBacked || typ.Dependency || typ.Namespace != "" || hasModifier(typ.Modifiers, "abstract") {
			continue
		}
		if strings.Contains(typ.Name, ".") {
			owner, ok := sourceTypes[typ.OwnerName]
			if typ.NestingDepth != 1 || typ.OwnerName == "" || !ok || !hasModifier(owner.Modifiers, "public") || !hasModifier(typ.Modifiers, "public") || len(typ.Interfaces) < 1 || len(typ.Interfaces) > 2 {
				continue
			}
		}
		for _, rawInterfaceName := range typ.Interfaces {
			interfaceName := strings.TrimSpace(rawInterfaceName)
			if interfaceName == "" || strings.ContainsAny(interfaceName, ".<>") {
				continue
			}
			key := typ.Name + "\x00" + interfaceName
			if _, ok := seen[key]; ok {
				continue
			}
			id, err := generator.Next("ApexTypeImplementor")
			if err != nil {
				return
			}
			seen[key] = struct{}{}
			state.Records[id] = storage.Record{
				ID:     id,
				Object: "ApexTypeImplementor",
				Fields: map[string]storage.Value{
					"ClassName":                storage.StringValue(typ.Name),
					"ClassNamespacePrefix":     storage.NullValue(),
					"InterfaceName":            storage.StringValue(interfaceName),
					"InterfaceNamespacePrefix": storage.NullValue(),
					"IsConcrete":               storage.BooleanValue(true),
				},
			}
		}
	}
	org.Objects["ApexTypeImplementor"] = state
}
