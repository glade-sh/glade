package capability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexdocs"
)

func TestBuildCatalogClassifiesInventoryEntries(t *testing.T) {
	inv := apexdocs.Inventory{
		SchemaVersion: 1,
		TotalFiles:    5,
		TotalMembers:  5,
		Documents: []apexdocs.Document{{
			SourcePath: "apex_methods_system_string.md",
			Kind:       "class",
			Namespace:  "System",
			Name:       "String",
			Members: []apexdocs.Member{{
				Kind:      "method",
				Name:      "trim",
				Signature: "trim()",
			}},
		}, {
			SourcePath: "apex_methods_system_database.md",
			Kind:       "class",
			Namespace:  "System",
			Name:       "Database",
			Members: []apexdocs.Member{{
				Kind:      "method",
				Name:      "insert",
				Signature: "insert(records)",
			}},
		}, {
			SourcePath: "apex_connectapi_output_FeedElement.md",
			Kind:       "output",
			Namespace:  "ConnectApi",
			Name:       "FeedElement",
			Members: []apexdocs.Member{{
				Kind:      "property",
				Name:      "body",
				Signature: "body",
			}},
		}, {
			SourcePath: "apex_class_System_Assert.md",
			Kind:       "class",
			Namespace:  "System",
			Name:       "Assert",
			Members: []apexdocs.Member{{
				Kind:      "method",
				Name:      "areEqual",
				Signature: "areEqual(expected, actual)",
			}},
		}, {
			SourcePath: "apex_language_variables.md",
			Kind:       "guide",
			Name:       "Variables",
			Members: []apexdocs.Member{{
				Kind:      "section",
				Name:      "local variables",
				Signature: "local variables",
			}},
		}},
	}

	catalog := BuildCatalog(inv)
	if catalog.SchemaVersion != CatalogSchemaVersion || catalog.SourceDocuments != 5 || catalog.SourceMembers != 5 {
		t.Fatalf("catalog summary = %#v", catalog)
	}

	stringTrim := findCatalogEntry(t, catalog, "String.trim")
	if stringTrim.Area != "Core stdlib" || stringTrim.Target != TargetExecutableParity || stringTrim.Status != StatusSupported {
		t.Fatalf("String.trim entry = %#v", stringTrim)
	}

	databaseInsert := findCatalogEntry(t, catalog, "Database.insert")
	if databaseInsert.Area != "Data platform" || databaseInsert.Target != TargetLocalModel || databaseInsert.Status != StatusSupported {
		t.Fatalf("Database.insert entry = %#v", databaseInsert)
	}

	connectBody := findCatalogEntry(t, catalog, "ConnectApi.FeedElement.body")
	if connectBody.Area != "Product namespaces" || connectBody.Target != TargetTypedStub || connectBody.Status != StatusUnknown {
		t.Fatalf("ConnectApi entry = %#v", connectBody)
	}

	systemAssert := findCatalogEntry(t, catalog, "Assert.areEqual")
	if systemAssert.Area != "Core stdlib" || systemAssert.Target != TargetExecutableParity {
		t.Fatalf("System.Assert entry = %#v", systemAssert)
	}

	variables := findCatalogEntry(t, catalog, "Variables")
	if variables.Target != TargetUnsupported || variables.Status != StatusUnsupported || !strings.Contains(variables.Notes, "not an executable") {
		t.Fatalf("Variables entry = %#v", variables)
	}
}

func TestWriteCatalogJSON(t *testing.T) {
	catalog := Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Entries: []CatalogEntry{{
			ID:         "string/trim#trim-method",
			Area:       "Core stdlib",
			TypeName:   "String",
			MemberName: "trim",
			Symbol:     "String.trim",
			Kind:       "method",
			Signature:  "trim()",
			Target:     TargetExecutableParity,
			Status:     StatusSupported,
			Owner:      "internal/vm",
		}},
	}
	var out bytes.Buffer
	if err := WriteCatalogJSON(&out, catalog); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"target": "executable-parity"`) {
		t.Fatalf("json = %q", out.String())
	}
	var decoded Catalog
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Entries[0].Symbol != "String.trim" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestBuildProductNamespaceReport(t *testing.T) {
	catalog := Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Entries: []CatalogEntry{{
			ID:         "connectapi/feedelement#output",
			Area:       "Product namespaces",
			Namespace:  "ConnectApi",
			TypeName:   "FeedElement",
			Symbol:     "ConnectApi.FeedElement",
			Kind:       "output",
			Target:     TargetTypedStub,
			Status:     StatusUnknown,
			Owner:      "generated declarations",
			DocsSource: "apex_connectapi_output_FeedElement.md",
		}, {
			ID:         "connectapi/feedelement/body#property",
			Area:       "Product namespaces",
			Namespace:  "ConnectApi",
			TypeName:   "FeedElement",
			MemberName: "body",
			Symbol:     "ConnectApi.FeedElement.body",
			Kind:       "property",
			Signature:  "body",
			Target:     TargetTypedStub,
			Status:     StatusUnknown,
			Owner:      "generated declarations",
			DocsSource: "apex_connectapi_output_FeedElement.md",
		}, {
			ID:        "metadata/deploycontainer#class",
			Area:      "Product namespaces",
			Namespace: "Metadata",
			TypeName:  "DeployContainer",
			Symbol:    "Metadata.DeployContainer",
			Kind:      "class",
			Target:    TargetTypedStub,
			Status:    StatusUnknown,
			Owner:     "generated declarations",
		}, {
			ID:         "string/trim#method",
			Area:       "Core stdlib",
			TypeName:   "String",
			Symbol:     "String.trim",
			Kind:       "method",
			Target:     TargetExecutableParity,
			Status:     StatusSupported,
			DocsSource: "apex_methods_system_string.md",
		}},
	}

	report := BuildProductNamespaceReportWithDeclarations(catalog, ProductNamespaceDeclarations{
		Types: map[string]ProductNamespaceDeclaredType{
			"connectapi.feedelement": {
				Namespace: "ConnectApi",
				Name:      "FeedElement",
				Kind:      "class",
				Members: map[string][]string{
					"body": {"property"},
				},
			},
		},
	})
	if report.Totals.Namespaces != 2 || report.Totals.Types != 2 || report.Totals.Members != 1 || report.Totals.Outputs != 1 {
		t.Fatalf("report totals = %#v", report.Totals)
	}
	if report.DeclarationCoverage.TypesWithDeclarations != 1 || report.DeclarationCoverage.TypesMissingDeclarations != 1 || report.DeclarationCoverage.MembersWithDeclarations != 1 || report.DeclarationCoverage.EntriesWithDeclarations != 2 {
		t.Fatalf("declaration coverage = %#v", report.DeclarationCoverage)
	}
	if report.Namespaces[0].Namespace != "ConnectApi" || report.Namespaces[0].Types[0].MemberCount != 1 {
		t.Fatalf("report namespaces = %#v", report.Namespaces)
	}
	if !report.Namespaces[0].Types[0].HasTypedDeclaration || !report.Namespaces[0].Types[0].Members[0].HasTypedDeclaration {
		t.Fatalf("typed declaration flags = %#v", report.Namespaces[0].Types[0])
	}
	if report.Namespaces[1].DeclarationStatus != "missing" || report.Namespaces[1].MissingTypes != 1 {
		t.Fatalf("missing declaration summary = %#v", report.Namespaces[1])
	}
	var out bytes.Buffer
	if err := WriteProductNamespaceJSON(&out, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"declarationPolicy": "generate typed declarations from public docs inventory"`) {
		t.Fatalf("json = %q", out.String())
	}
	if !strings.Contains(out.String(), `"hasTypedDeclaration": true`) {
		t.Fatalf("json missing declaration availability = %q", out.String())
	}
}

func TestWriteProductNamespaceSymbolsGoNormalizesCatalogAndTooling(t *testing.T) {
	catalog := Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Entries: []CatalogEntry{{
			ID:        "cache/orgpartition#class",
			Area:      "Product namespaces",
			Namespace: "cache",
			TypeName:  "OrgPartition",
			Symbol:    "cache.OrgPartition",
			Kind:      "class",
			Target:    TargetTypedStub,
			Status:    StatusUnknown,
		}, {
			ID:         "cache/orgpartition/doconly",
			Area:       "Product namespaces",
			Namespace:  "cache",
			TypeName:   "OrgPartition",
			MemberName: "docOnly",
			Symbol:     "cache.OrgPartition.docOnly",
			Kind:       "method",
			Signature:  "docOnly(value, other)",
			Target:     TargetTypedStub,
			Status:     StatusUnknown,
		}, {
			ID:         "cache/orgpartition/get",
			Area:       "Product namespaces",
			Namespace:  "cache",
			TypeName:   "OrgPartition",
			MemberName: "get",
			Symbol:     "cache.OrgPartition.get",
			Kind:       "method",
			Signature:  "get(key)",
			Target:     TargetTypedStub,
			Status:     StatusUnknown,
		}, {
			ID:        "connectapi/weakoutput#output",
			Area:      "Product namespaces",
			Namespace: "ConnectApi",
			TypeName:  "WeakOutput",
			Symbol:    "ConnectApi.WeakOutput",
			Kind:      "output",
			Target:    TargetTypedStub,
			Status:    StatusUnknown,
		}, {
			ID:         "connectapi/weakoutput/value",
			Area:       "Product namespaces",
			Namespace:  "ConnectApi",
			TypeName:   "WeakOutput",
			MemberName: "value",
			Symbol:     "ConnectApi.WeakOutput.value",
			Kind:       "property",
			Signature:  "value",
			Target:     TargetTypedStub,
			Status:     StatusUnknown,
		}},
	}
	tooling := ToolingCompletions{PublicDeclarations: map[string]map[string]ToolingClassDecl{
		"cache": {
			"OrgPartition": {
				Methods: []ToolingMethod{{
					Name:       "get",
					ReturnType: "System.Object",
					IsStatic:   true,
					Parameters: []ToolingParameter{{Name: "key", Type: "System.String"}},
				}, {
					Name:       "put",
					ReturnType: "void",
					Parameters: []ToolingParameter{{Name: "key", Type: "System.String"}, {Name: "value", Type: "APEX_OBJECT"}},
				}},
			},
		},
	}}

	var out bytes.Buffer
	if err := WriteProductNamespaceSymbolsGo(&out, catalog, &tooling); err != nil {
		t.Fatal(err)
	}
	goSource := out.String()
	for _, want := range []string{
		`Name: "Cache.OrgPartition"`,
		`{Name: "docOnly", ReturnType: "Object", Parameters: []string{"Object", "Object"}}`,
		`{Name: "get", ReturnType: "Object", Parameters: []string{"String"}, Static: true}`,
		`{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object"}}`,
		`Name: "ConnectApi.WeakOutput"`,
		`{Name: "value", Type: "Object"}`,
	} {
		if !strings.Contains(goSource, want) {
			t.Fatalf("generated symbols missing %q:\n%s", want, goSource)
		}
	}
	if strings.Contains(goSource, `Name: "cache.`) || strings.Contains(goSource, `System.String`) || strings.Contains(goSource, `APEX_OBJECT`) {
		t.Fatalf("generated symbols were not normalized:\n%s", goSource)
	}
	if strings.Count(goSource, `Name: "get"`) != 1 {
		t.Fatalf("weak docs shape shadowed typed Tooling shape:\n%s", goSource)
	}
	if got := normalizeProductNamespaceType("Map<System.String,ANY>"); got != "Map<String,Object>" {
		t.Fatalf("generic weak type normalization = %q", got)
	}
}

func TestBuildSalesforceCoverageReport(t *testing.T) {
	catalog := Catalog{
		SchemaVersion:   CatalogSchemaVersion,
		SourceDocuments: 2,
		SourceMembers:   2,
		Entries: []CatalogEntry{{
			ID:         "string/trim#method",
			Area:       "Core stdlib",
			TypeName:   "String",
			MemberName: "trim",
			Symbol:     "String.trim",
			Kind:       "method",
			Target:     TargetExecutableParity,
			Status:     StatusSupported,
			Owner:      "internal/vm",
			DocsSource: "apex_methods_system_string.md",
		}, {
			ID:         "connectapi/feedelement/body#property",
			Area:       "Product namespaces",
			Namespace:  "ConnectApi",
			TypeName:   "FeedElement",
			MemberName: "body",
			Symbol:     "ConnectApi.FeedElement.body",
			Kind:       "property",
			Target:     TargetTypedStub,
			Status:     StatusUnknown,
			Owner:      "generated declarations",
			DocsSource: "apex_connectapi_output_FeedElement.md",
		}},
	}

	report := BuildSalesforceCoverageReport(catalog)
	if report.SchemaVersion != SalesforceCoverageSchemaVersion || report.SourceDocuments != 2 || report.Entries != 2 {
		t.Fatalf("report = %#v", report)
	}
	if report.Totals.Supported != 1 || report.Totals.Unknown != 1 {
		t.Fatalf("totals = %#v", report.Totals)
	}
	if len(report.Areas) != 2 {
		t.Fatalf("areas = %#v", report.Areas)
	}
	var out bytes.Buffer
	if err := WriteSalesforceCoverageMarkdown(&out, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "# Salesforce Coverage Manifest") || !strings.Contains(out.String(), "Core stdlib") {
		t.Fatalf("markdown = %q", out.String())
	}
}

func TestBuildSalesforceToolingAlignmentNormalizesCompletionsAndSymbolTables(t *testing.T) {
	catalog := Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Entries: []CatalogEntry{{
			ID:       "list/class",
			Area:     "Core stdlib",
			TypeName: "List",
			Symbol:   "List",
			Kind:     "class",
			Target:   TargetExecutableParity,
			Status:   StatusUnknown,
		}, {
			ID:         "connectapi/chatterfeeds/postfeedelement",
			Area:       "Product namespaces",
			Namespace:  "ConnectApi",
			TypeName:   "ChatterFeeds",
			MemberName: "postFeedElement",
			Symbol:     "ConnectApi.ChatterFeeds.postFeedElement",
			Kind:       "method",
			Target:     TargetLocalModel,
			Status:     StatusUnknown,
		}, {
			ID:         "pkg/managed/doit",
			Area:       "Product namespaces",
			Namespace:  "pkg",
			TypeName:   "Managed",
			MemberName: "doIt",
			Symbol:     "pkg.Managed.doIt",
			Kind:       "method",
			Target:     TargetLocalModel,
			Status:     StatusUnknown,
		}},
	}
	completions := ToolingCompletions{PublicDeclarations: map[string]map[string]ToolingClassDecl{
		"System": {
			"LIST":          {},
			"CallException": {},
		},
		"ConnectApi": {},
	}}
	NormalizeToolingCompletions(&completions)
	symbols := ToolingApexClassSymbols{Records: []ToolingApexClassRecord{{
		Name:            "Managed",
		NamespacePrefix: "pkg",
		SymbolTable: &ToolingSymbolTable{
			Methods: []ToolingSymbolMethod{{Name: "doIt"}},
			InnerClasses: []ToolingSymbolTable{{
				Name:       "Nested",
				Properties: []ToolingSymbolProperty{{Name: "label", Type: "String"}},
			}},
		},
	}}}

	alignment := BuildSalesforceToolingAlignment(catalog, &completions, &symbols)
	if alignment.SystemDefaultNamespaceClasses != 2 || alignment.Constructors != 4 {
		t.Fatalf("system alignment = %#v", alignment)
	}
	if alignment.SymbolTableClasses != 2 || alignment.SymbolTableMethods != 1 || alignment.SymbolTableProperties != 1 {
		t.Fatalf("symbol table alignment = %#v", alignment)
	}
	if alignment.CatalogSystemEntriesInTooling != 3 || alignment.CatalogSystemEntriesMissing != 0 {
		t.Fatalf("catalog alignment = %#v", alignment)
	}
}

func TestBuildStandardObjectCoverageReport(t *testing.T) {
	report := BuildStandardObjectCoverageReport()
	if report.SchemaVersion != StandardObjectCoverageSchemaVersion || report.Totals.Objects == 0 || report.Totals.Fields == 0 {
		t.Fatalf("report totals = %#v", report.Totals)
	}
	account := StandardObjectCoverageEntry{}
	for _, entry := range report.Objects {
		if entry.Object == "Account" {
			account = entry
			break
		}
	}
	if account.Object != "Account" || account.KeyPrefix != "001" || account.Fields == 0 {
		t.Fatalf("account coverage = %#v", account)
	}
	var out bytes.Buffer
	if err := WriteStandardObjectCoverageMarkdown(&out, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "# Standard Object Coverage") || !strings.Contains(out.String(), "`Account`") {
		t.Fatalf("markdown = %q", out.String())
	}
}

func findCatalogEntry(t *testing.T, catalog Catalog, symbol string) CatalogEntry {
	t.Helper()
	for _, entry := range catalog.Entries {
		if entry.Symbol == symbol {
			return entry
		}
	}
	t.Fatalf("missing catalog entry %s in %#v", symbol, catalog.Entries)
	return CatalogEntry{}
}
