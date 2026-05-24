package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func BenchmarkAnalyzeIndex(b *testing.B) {
	index := benchmarkIndex(200)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := Analyze(index)
		if result.HasErrors() {
			b.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
		}
	}
}

func benchmarkIndex(classes int) typesys.Index {
	index := typesys.Index{
		Objects: []schema.Object{{Name: "Account"}, {Name: "Contact"}, {Name: "Thing__c"}},
	}
	for i := 0; i < classes; i++ {
		index.Types = append(index.Types, typesys.TypeSymbol{
			Kind: apexast.DeclarationClass,
			Name: fmt.Sprintf("Service%03d", i),
			File: fmt.Sprintf("Service%03d.cls", i),
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationField, Name: "account", Type: "Account"},
				{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<Thing__c>"},
				{Kind: apexast.DeclarationMethod, Name: "contacts", Type: "Map<String,List<Contact>>"},
			},
		})
	}
	return index
}
