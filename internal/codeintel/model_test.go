package codeintel_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/diagnostic"
)

func TestGraphAddSymbolMergesMetadataWithoutClobberingLocation(t *testing.T) {
	graph := codeintel.NewGraph("/tmp/project")
	id := codeintel.ApexTypeID("", "InvoiceService")
	firstRange := diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}, End: diagnostic.Position{Line: 5, Column: 2}}
	secondRange := diagnostic.Range{Start: diagnostic.Position{Line: 10, Column: 1}, End: diagnostic.Position{Line: 12, Column: 2}}

	graph.AddSymbol(codeintel.Symbol{
		ID:       id,
		Kind:     codeintel.SymbolApexType,
		Name:     "InvoiceService",
		File:     "force-app/main/default/classes/InvoiceService.cls",
		Range:    firstRange,
		Metadata: map[string]string{"first": "kept"},
	})
	graph.AddSymbol(codeintel.Symbol{
		ID:       id,
		Kind:     codeintel.SymbolUnknown,
		Name:     "InvoiceService",
		File:     "other.cls",
		Range:    secondRange,
		Metadata: map[string]string{"second": "merged"},
	})

	symbol, ok := graph.Definition(id)
	if !ok {
		t.Fatalf("missing symbol %s", id)
	}
	if symbol.File != "force-app/main/default/classes/InvoiceService.cls" || symbol.Range != firstRange {
		t.Fatalf("location was clobbered: %#v", symbol)
	}
	if symbol.Metadata["first"] != "kept" || symbol.Metadata["second"] != "merged" {
		t.Fatalf("metadata = %#v", symbol.Metadata)
	}
}

func TestGraphSortedSymbols(t *testing.T) {
	graph := codeintel.NewGraph("/tmp/project")
	graph.AddSymbol(codeintel.Symbol{ID: codeintel.SObjectID("Account"), Kind: codeintel.SymbolSObject, Name: "Account"})
	graph.AddSymbol(codeintel.Symbol{ID: codeintel.ApexTypeID("", "BillingService"), Kind: codeintel.SymbolApexType, Name: "BillingService"})
	graph.AddSymbol(codeintel.Symbol{ID: codeintel.ApexTypeID("", "AccountService"), Kind: codeintel.SymbolApexType, Name: "AccountService"})

	got := graph.SortedSymbols()
	if len(got) != 3 {
		t.Fatalf("symbols = %d", len(got))
	}
	names := []string{got[0].Name, got[1].Name, got[2].Name}
	want := []string{"AccountService", "BillingService", "Account"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %#v, want %#v", names, want)
		}
	}
}

func TestGraphReferencesCanExcludeDeclarations(t *testing.T) {
	graph := codeintel.NewGraph("/tmp/project")
	id := codeintel.SObjectFieldID("Account", "Name")
	graph.AddUse(codeintel.Use{SymbolID: id, Kind: codeintel.UseDeclaration, Name: "Name", File: "objects/Account/fields/Name.field-meta.xml", Resolved: true})
	graph.AddUse(codeintel.Use{SymbolID: id, Kind: codeintel.UseRead, Name: "Name", File: "classes/Reader.cls", Resolved: true})
	graph.AddUse(codeintel.Use{SymbolID: id, Kind: codeintel.UseWrite, Name: "Name", File: "classes/Writer.cls", Resolved: true})

	withDeclarations := graph.References(id, true)
	if len(withDeclarations) != 3 {
		t.Fatalf("with declarations = %d", len(withDeclarations))
	}
	withoutDeclarations := graph.References(id, false)
	if len(withoutDeclarations) != 2 {
		t.Fatalf("without declarations = %d", len(withoutDeclarations))
	}
	if withoutDeclarations[0].Kind == codeintel.UseDeclaration || withoutDeclarations[1].Kind == codeintel.UseDeclaration {
		t.Fatalf("declaration leaked: %#v", withoutDeclarations)
	}
}

func TestGraphUsesByFileSortsByLocation(t *testing.T) {
	graph := codeintel.NewGraph("/tmp/project")
	id := codeintel.ApexTypeID("", "InvoiceService")
	graph.AddUse(codeintel.Use{SymbolID: id, Kind: codeintel.UseRead, Name: "InvoiceService", File: "classes/Consumer.cls", Range: diagnostic.Range{Start: diagnostic.Position{Line: 4, Column: 9}}, Resolved: true})
	graph.AddUse(codeintel.Use{SymbolID: id, Kind: codeintel.UseRead, Name: "InvoiceService", File: "classes/Consumer.cls", Range: diagnostic.Range{Start: diagnostic.Position{Line: 2, Column: 3}}, Resolved: true})

	uses := graph.UsesByFile("classes/Consumer.cls")
	if len(uses) != 2 {
		t.Fatalf("uses = %d", len(uses))
	}
	if uses[0].Range.Start.Line != 2 || uses[1].Range.Start.Line != 4 {
		t.Fatalf("uses not sorted: %#v", uses)
	}
}
