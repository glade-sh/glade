package sema

import (
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/typesys"
)

func TestNoPanicOnMalformedIndex(t *testing.T) {
	indexes := []typesys.Index{
		{},
		{
			Types: []typesys.TypeSymbol{{
				Kind: apexast.DeclarationClass,
				Name: "Broken",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "Map<String,List<MissingType>>"},
					{Kind: apexast.DeclarationField, Name: "field", Type: "List<"},
				},
			}},
			Triggers: []typesys.TriggerSymbol{{Name: "BrokenTrigger", ObjectName: "Missing__c"}},
		},
	}
	for _, index := range indexes {
		assertNoPanic(t, func() {
			_ = Analyze(index)
		})
	}
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
