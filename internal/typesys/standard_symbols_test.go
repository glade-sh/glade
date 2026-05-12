package typesys

import (
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
)

func TestStandardPlatformSymbolsMergeProductNamespaceDeclarations(t *testing.T) {
	symbols := StandardPlatformSymbols()

	operations := requireStandardSymbol(t, symbols, "Metadata.Operations")
	requireStandardMethod(t, operations, "retrieve", []string{"Metadata.MetadataType", "List<String>", "Boolean"}, true)
	requireStandardMethod(t, operations, "checkDeployStatus", []string{"Id", "Boolean"}, true)

	container := requireStandardSymbol(t, symbols, "Metadata.DeployContainer")
	requireStandardConstructor(t, container, []string{})
	requireStandardMethod(t, container, "addMetadata", []string{"Metadata.Metadata"}, false)

	settings := requireStandardSymbol(t, symbols, "ConnectApi.OrganizationSettings")
	requireStandardProperty(t, settings, "userSettings", "ConnectApi.UserSettings")

	visibility := requireStandardSymbol(t, symbols, "Cache.Visibility")
	if visibility.Kind != apexast.DeclarationEnum {
		t.Fatalf("Cache.Visibility kind = %q, want enum", visibility.Kind)
	}
	requireStandardProperty(t, visibility, "ALL", "Cache.Visibility")
}

func TestStandardSymbolsFromSpecsMergesDuplicateTypesCaseInsensitively(t *testing.T) {
	symbols := StandardSymbolsFromSpecs([]StandardSymbolSpec{{
		Name: "ConnectApi.Organization",
		Methods: []StandardMethodSpec{{
			Name:       "getSettings",
			ReturnType: "ConnectApi.OrganizationSettings",
			Static:     true,
		}},
	}, {
		Name: "connectapi.organization",
		Methods: []StandardMethodSpec{{
			Name:       "getSettings",
			ReturnType: "ConnectApi.OrganizationSettings",
			Static:     true,
		}, {
			Name:       "getHealth",
			ReturnType: "Object",
			Static:     true,
		}},
	}})

	if len(symbols) != 1 {
		t.Fatalf("symbol count = %d, want 1: %#v", len(symbols), symbols)
	}
	requireStandardMethod(t, symbols[0], "getSettings", nil, true)
	requireStandardMethod(t, symbols[0], "getHealth", nil, true)
}

func TestStandardPlatformSymbolsIncludeInstallVersion(t *testing.T) {
	symbols := StandardPlatformSymbols()

	version := requireStandardSymbol(t, symbols, "Version")
	requireStandardConstructor(t, version, []string{"Integer", "Integer"})
	requireStandardConstructor(t, version, []string{"Integer", "Integer", "Integer"})
	requireStandardMethod(t, version, "compareTo", []string{"Version"}, false)

	installContext := requireStandardSymbol(t, symbols, "InstallContext")
	requireStandardMethod(t, installContext, "previousVersion", nil, false)
}

func TestStandardPlatformSymbolsIncludeSearchQuery(t *testing.T) {
	symbols := StandardPlatformSymbols()

	search := requireStandardSymbol(t, symbols, "Search")
	requireStandardMethod(t, search, "query", []string{"String"}, true)
	requireStandardMethodType(t, search, "query", "List<List<SObject>>")
}

func requireStandardSymbol(t *testing.T, symbols []TypeSymbol, name string) TypeSymbol {
	t.Helper()
	for _, symbol := range symbols {
		if strings.EqualFold(standardSymbolFullName(symbol), name) {
			return symbol
		}
	}
	t.Fatalf("missing standard symbol %s", name)
	return TypeSymbol{}
}

func requireStandardConstructor(t *testing.T, symbol TypeSymbol, params []string) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind != apexast.DeclarationConstructor {
			continue
		}
		if standardMemberParamsEqual(member, params) {
			return
		}
	}
	t.Fatalf("missing constructor on %s with params %#v: %#v", standardSymbolFullName(symbol), params, symbol.Members)
}

func requireStandardMethod(t *testing.T, symbol TypeSymbol, name string, params []string, static bool) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(member.Name, name) {
			continue
		}
		if !standardMemberParamsEqual(member, params) {
			continue
		}
		if memberHasModifier(member, "static") != static {
			continue
		}
		return
	}
	t.Fatalf("missing method %s.%s with params %#v static=%v: %#v", standardSymbolFullName(symbol), name, params, static, symbol.Members)
}

func requireStandardProperty(t *testing.T, symbol TypeSymbol, name, typ string) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind == apexast.DeclarationProperty && strings.EqualFold(member.Name, name) && strings.EqualFold(member.Type, typ) {
			return
		}
	}
	t.Fatalf("missing property %s.%s type %s: %#v", standardSymbolFullName(symbol), name, typ, symbol.Members)
}

func requireStandardMethodType(t *testing.T, symbol TypeSymbol, name, typ string) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, name) && strings.EqualFold(member.Type, typ) {
			return
		}
	}
	t.Fatalf("missing method %s.%s type %s: %#v", standardSymbolFullName(symbol), name, typ, symbol.Members)
}

func standardMemberParamsEqual(member MemberSymbol, params []string) bool {
	if len(member.Parameters) != len(params) {
		return false
	}
	for i := range params {
		if !strings.EqualFold(member.Parameters[i].Type, params[i]) {
			return false
		}
	}
	return true
}

func memberHasModifier(member MemberSymbol, modifier string) bool {
	for _, candidate := range member.Modifiers {
		if strings.EqualFold(candidate, modifier) {
			return true
		}
	}
	return false
}

func standardSymbolFullName(symbol TypeSymbol) string {
	if symbol.Namespace == "" {
		return symbol.Name
	}
	return symbol.Namespace + "." + symbol.Name
}
