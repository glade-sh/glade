package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// receiver_member_routes/org.tsv:30-37,70-77 supplies both rejected methods;
// qualified_receivers/org.tsv:2-5 supplies the accepted long describe route.
func TestReceiverMemberShortFieldsCallRecognizerNativeRoutes(t *testing.T) {
	model := buildSemaTypeMemberView(typesys.Index{Objects: []schema.Object{{Name: "Account"}}})
	for _, tc := range []struct {
		name, receiver, method string
		short                  bool
	}{
		{"short-getmap", "Account.SObjectType.fields", "getMap", true},
		{"short-get", "Account.SObjectType.fields", "get", true},
		{"long-describe-getmap", "Schema.SObjectType.Account.fields", "getMap", false},
		{"field-token-describe", "Account.SObjectType.fields.Name", "getDescribe", false},
		{"missing-object", "MissingObject__c.SObjectType.fields", "getMap", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := semaShortSObjectFieldsCall(NewAnalyzer(), tc.receiver, tc.method, model); got != tc.short {
				t.Fatalf("short fields call=%v, want %v", got, tc.short)
			}
		})
	}
}

// A source class with the same spelling is not a schema token receiver.
func TestReceiverMemberShortFieldsRecognizerSourceShadowRoute(t *testing.T) {
	model := buildSemaTypeMemberView(typesys.Index{Types: []typesys.TypeSymbol{{Kind: apexast.DeclarationClass, Name: "Account"}}})
	if semaShortSObjectFieldsCall(NewAnalyzer(), "Account.SObjectType.fields", "getMap", model) {
		t.Fatal("source class was treated as an SObject token")
	}
}
func TestShortSObjectFieldsProxyBoundary(t *testing.T) {
	index := typesys.Index{}
	a := NewAnalyzer()
	a.prepareAnalysisContext(index, AnalyzeOptions{})
	a.prepareAnalysisModel(index)
	model := buildSemaTypeMemberView(index)
	for _, receiver := range []string{"Account.fields", "Account.SObjectType.fields"} {
		for _, method := range []string{"getMap", "get"} {
			if !semaShortSObjectFieldsCall(a, receiver, method, model) {
				t.Errorf("captured field proxy bypassed: %s.%s", receiver, method)
			}
			a.namespace = "Account"
			if semaShortSObjectFieldsCall(a, receiver, method, model) {
				t.Errorf("namespace alias borrowed an SObject proxy: %s.%s", receiver, method)
			}
			a.namespace = ""
		}
	}
	inferred := buildSemaTypeMemberView(typesys.Index{Objects: []schema.Object{{Name: "OpaqueNs", Partial: true}}})
	if semaShortSObjectFieldsCall(a, "OpaqueNs.fields", "getMap", inferred) {
		t.Error("inferred object closed an opaque namespace")
	}
}
