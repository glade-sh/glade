package sema

import (
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
)

func TestAnalyzeResolvesMemberTypes(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<Thing__c>"},
				},
			},
		},
		Objects: []schema.Object{{Name: "Thing__c"}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUnknownMemberType(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "MissingType", Range: diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}}},
				},
			},
		},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "OAERSEMA002" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func TestAnalyzeTriggerObject(t *testing.T) {
	index := typesys.Index{
		Triggers: []typesys.TriggerSymbol{{Name: "ThingTrigger", ObjectName: "Missing__c", File: "Thing.trigger"}},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "OAERSEMA001" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func TestExtractTypeNames(t *testing.T) {
	got := extractTypeNames("Map<String,List<Account>>")
	want := []string{"String", "Account"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v", got)
		}
	}
}
