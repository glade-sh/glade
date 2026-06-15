package codeintel_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestBuildDeclarationsIndexesApexSchemaAndMetadata(t *testing.T) {
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: "/tmp/project", Namespace: "pkg"},
		Types: []typesys.TypeSymbol{{
			Kind:      apexast.DeclarationClass,
			Name:      "InvoiceService",
			Namespace: "pkg",
			File:      "force-app/main/default/classes/InvoiceService.cls",
			Range:     testRange(1, 1, 12, 2),
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationField, Name: "total", Type: "Decimal", Range: testRange(2, 3, 2, 24)},
				{Kind: apexast.DeclarationConstructor, Name: "InvoiceService", Parameters: []apexast.Parameter{{Name: "account", Type: "Account"}}, Range: testRange(3, 3, 3, 37)},
				{Kind: apexast.DeclarationMethod, Name: "calculate", Type: "Decimal", Parameters: []apexast.Parameter{{Name: "account", Type: "Account"}}, Range: testRange(4, 3, 4, 48)},
				{Kind: apexast.DeclarationMethod, Name: "calculate", Type: "Decimal", Parameters: []apexast.Parameter{{Name: "invoice", Type: "Invoice__c"}}, Range: testRange(5, 3, 5, 51)},
				{Kind: apexast.DeclarationProperty, Name: "Name", Type: "String", Range: testRange(6, 3, 6, 30)},
			},
		}},
		Triggers: []typesys.TriggerSymbol{{
			Name:       "InvoiceTrigger",
			Namespace:  "pkg",
			ObjectName: "Invoice__c",
			File:       "force-app/main/default/triggers/InvoiceTrigger.trigger",
			Range:      testRange(1, 1, 3, 2),
		}},
		Objects: []schema.Object{{
			Name:  "Invoice__c",
			Label: "Invoice",
			Fields: []schema.Field{
				{Name: "Amount__c", Label: "Amount", Type: "Currency"},
				{Name: "Account__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
			},
		}},
		CustomMetadataRecords: []schema.CustomMetadataRecord{{
			FullName:      "Feature.Default",
			ObjectName:    "Feature__mdt",
			DeveloperName: "Default",
			File:          "force-app/main/default/customMetadata/Feature.Default.md-meta.xml",
		}},
	}

	graph := codeintel.BuildDeclarations(index)

	assertSymbol(t, graph, codeintel.ApexTypeID("pkg", "InvoiceService"), codeintel.SymbolApexType, "InvoiceService")
	assertSymbol(t, graph, codeintel.ApexMemberID("pkg", "InvoiceService", "method", "calculate", "Decimal(Account)"), codeintel.SymbolApexMember, "calculate")
	assertSymbol(t, graph, codeintel.ApexMemberID("pkg", "InvoiceService", "method", "calculate", "Decimal(Invoice__c)"), codeintel.SymbolApexMember, "calculate")
	assertSymbol(t, graph, codeintel.TriggerID("pkg", "InvoiceTrigger"), codeintel.SymbolTrigger, "InvoiceTrigger")
	assertSymbol(t, graph, codeintel.SObjectID("Invoice__c"), codeintel.SymbolSObject, "Invoice__c")
	assertSymbol(t, graph, codeintel.SObjectFieldID("Invoice__c", "Amount__c"), codeintel.SymbolSObjectField, "Amount__c")
	assertSymbol(t, graph, codeintel.CustomMetadataID("Feature__mdt", "Default"), codeintel.SymbolCustomMetadata, "Feature.Default")

	for _, symbol := range graph.SortedSymbols() {
		refs := graph.References(symbol.ID, true)
		if len(refs) == 0 || refs[0].Kind != codeintel.UseDeclaration {
			t.Fatalf("symbol %s missing declaration use: %#v", symbol.ID, refs)
		}
	}
}

func TestBuildDeclarationsPreservesDependencyAndArtifactFlags(t *testing.T) {
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: "/tmp/project"},
		Types: []typesys.TypeSymbol{{
			Kind:       apexast.DeclarationClass,
			Name:       "Address",
			Namespace:  "pkg",
			File:       "pkg/Address.cls",
			Dependency: true,
			Artifact:   true,
			Range:      testRange(1, 1, 1, 24),
		}},
	}

	graph := codeintel.BuildDeclarations(index)
	symbol, ok := graph.Definition(codeintel.ApexTypeID("pkg", "Address"))
	if !ok {
		t.Fatal("missing Address symbol")
	}
	if !symbol.Dependency || !symbol.Artifact {
		t.Fatalf("flags not preserved: %#v", symbol)
	}
}

func assertSymbol(t *testing.T, graph codeintel.Graph, id codeintel.SymbolID, kind codeintel.SymbolKind, name string) {
	t.Helper()
	symbol, ok := graph.Definition(id)
	if !ok {
		t.Fatalf("missing symbol %s", id)
	}
	if symbol.Kind != kind || symbol.Name != name {
		t.Fatalf("symbol %s = %#v", id, symbol)
	}
}

func testRange(startLine, startColumn, endLine, endColumn int) diagnostic.Range {
	return diagnostic.Range{
		Start: diagnostic.Position{Line: startLine, Column: startColumn},
		End:   diagnostic.Position{Line: endLine, Column: endColumn},
	}
}
