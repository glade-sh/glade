package codeintel_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/schema"
)

func TestOfflineSchemaCacheReaderLoadsObjectAndFieldSymbols(t *testing.T) {
	root := t.TempDir()
	s := schema.Schema{Objects: []schema.Object{{
		Name:  "Account",
		Label: "Account",
		Fields: []schema.Field{
			{Name: "Id", Type: "Id", Label: "Account ID"},
			{Name: "Name", Type: "Text", Label: "Account Name"},
		},
	}, {
		Name:  "Widget__c",
		Label: "Widget",
		Fields: []schema.Field{
			{Name: "Title__c", Type: "Text", Label: "Title"},
		},
	}}}

	if err := codeintel.WriteSchemaCache(root, s); err != nil {
		t.Fatalf("WriteSchemaCache: %v", err)
	}
	graph, _, err := codeintel.ReadCache(root)
	if err != nil {
		t.Fatalf("ReadCache: %v", err)
	}

	if got := graph.Symbols[codeintel.SObjectID("Widget__c")]; got.Kind != codeintel.SymbolSObject || got.Metadata["label"] != "Widget" {
		t.Fatalf("Widget symbol = %#v", got)
	}
	if got := graph.Symbols[codeintel.SObjectFieldID("Account", "Name")]; got.Kind != codeintel.SymbolSObjectField || got.Type != "Text" || got.Metadata["label"] != "Account Name" {
		t.Fatalf("Account.Name symbol = %#v", got)
	}
}
