package codeintel_test

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
)

func TestStableSymbolIDs(t *testing.T) {
	tests := []struct {
		name string
		got  codeintel.SymbolID
		want codeintel.SymbolID
	}{
		{"type", codeintel.ApexTypeID("pkg", "InvoiceService"), "apex:type:pkg:InvoiceService"},
		{"member", codeintel.ApexMemberID("pkg", "InvoiceService", "method", "total", "Decimal(Account)"), "apex:member:pkg:InvoiceService:method:total:Decimal(Account)"},
		{"object", codeintel.SObjectID("Account"), "schema:object:Account"},
		{"field", codeintel.SObjectFieldID("Account", "Name"), "schema:field:Account:Name"},
		{"escaped", codeintel.ApexTypeID("pkg:core", "Thing:Service"), "apex:type:pkg%3Acore:Thing%3AService"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s ID = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestParseIDUnescapesParts(t *testing.T) {
	got := codeintel.ParseID(codeintel.ApexTypeID("pkg:core", "Thing:Service"))
	want := []string{"apex", "type", "pkg:core", "Thing:Service"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseID = %#v, want %#v", got, want)
	}
}
