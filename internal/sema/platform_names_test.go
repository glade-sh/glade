package sema

import (
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSemaCanonicalPlatformAliasCoversDocumentedSystemNamespace(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"System.Blob", "Blob"},
		{"System.Boolean", "Boolean"},
		{"System.Database", "Database"},
		{"System.Date", "Date"},
		{"System.Datetime", "Datetime"},
		{"System.Decimal", "Decimal"},
		{"System.Exception", "Exception"},
		{"System.HttpRequest", "HttpRequest"},
		{"System.HttpResponse", "HttpResponse"},
		{"System.JSON", "JSON"},
		{"System.Cookie", "Cookie"},
		{"System.Limits", "Limits"},
		{"System.Location", "Location"},
		{"System.Math", "Math"},
		{"System.RestContext", "RestContext"},
		{"System.RestRequest", "RestRequest"},
		{"System.RestResponse", "RestResponse"},
		{"System.String", "String"},
		{"System.System", "System"},
		{"System.URL", "URL"},
		{"List<System.RestRequest>", "List<RestRequest>"},
		{"Map<String,System.HttpResponse>", "Map<String,HttpResponse>"},
	} {
		if got := semaCanonicalPlatformAlias(tc.in); got != tc.want {
			t.Fatalf("semaCanonicalPlatformAlias(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSemaCanonicalPlatformAliasCoversEverySystemNamespaceTypeName(t *testing.T) {
	for _, name := range typesys.StandardSystemNamespaceTypeNames() {
		qualified := "System." + name
		if got := semaCanonicalPlatformAlias(qualified); got == qualified {
			t.Fatalf("semaCanonicalPlatformAlias(%q) did not resolve", qualified)
		}
	}
}

func TestSemaPlatformAliasesDoesNotExposeSharedCache(t *testing.T) {
	key := normalizeName("System.String")
	want := semaCanonicalPlatformAlias("System.String")
	aliases := semaPlatformAliases()
	aliases[key] = "CallerCorruption"
	if got := semaCanonicalPlatformAlias("System.String"); got != want {
		t.Fatalf("caller mutation changed System.String alias from %q to %q", want, got)
	}
}

func TestSemaCanonicalPlatformAliasCoversDocumentedSchemaImplicitImports(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"ChildRelationship", "Schema.ChildRelationship"},
		{"DataCategory", "Schema.DataCategory"},
		{"DataCategoryGroupSobjectTypePair", "Schema.DataCategoryGroupSobjectTypePair"},
		{"DescribeColorResult", "Schema.DescribeColorResult"},
		{"DescribeDataCategoryGroupResult", "Schema.DescribeDataCategoryGroupResult"},
		{"DescribeDataCategoryGroupStructureResult", "Schema.DescribeDataCategoryGroupStructureResult"},
		{"DescribeFieldResult", "Schema.DescribeFieldResult"},
		{"DescribeIconResult", "Schema.DescribeIconResult"},
		{"DescribeSObjectResult", "Schema.DescribeSObjectResult"},
		{"DisplayType", "Schema.DisplayType"},
		{"FieldDescribeOptions", "Schema.FieldDescribeOptions"},
		{"FieldSet", "Schema.FieldSet"},
		{"FieldSetMember", "Schema.FieldSetMember"},
		{"FilteredLookupInfo", "Schema.FilteredLookupInfo"},
		{"PicklistEntry", "Schema.PicklistEntry"},
		{"RecordTypeInfo", "Schema.RecordTypeInfo"},
		{"SObjectDescribeOptions", "Schema.SObjectDescribeOptions"},
		{"SObjectField", "Schema.SObjectField"},
		{"SObjectType", "Schema.SObjectType"},
		{"SObjectTypeFields", "Schema.SObjectTypeFields"},
		{"SObjectTypeFieldSets", "Schema.SObjectTypeFieldSets"},
		{"SoapType", "Schema.SoapType"},
		{"List<FieldDescribeOptions>", "List<Schema.FieldDescribeOptions>"},
	} {
		if got := semaCanonicalPlatformAlias(tc.in); got != tc.want {
			t.Fatalf("semaCanonicalPlatformAlias(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSemaCanonicalNamesMatchNormalizeNameForSalesforceNames(t *testing.T) {
	names := newSemaCanonicalNames(64)
	for _, name := range []string{
		" Account__c ",
		"pkg__Account__r",
		"Feature__mdt",
		"Case__Share",
		"System.String",
		"pkg.Outer.Inner",
		"Straße",
		"İstanbul__c",
	} {
		if got, want := names.canonical(name), normalizeName(name); got != want {
			t.Fatalf("canonical(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSemaCanonicalNamesRemainBounded(t *testing.T) {
	names := newSemaCanonicalNames(2)
	for _, name := range []string{"FirstName", "SecondName", "ThirdName"} {
		_ = names.canonical(name)
	}
	if got := names.size(); got != 2 {
		t.Fatalf("canonical cache size = %d, want 2", got)
	}
}

func TestSemaCanonicalNamesRepeatedLookupDoesNotAllocate(t *testing.T) {
	names := newSemaCanonicalNames(8)
	const input = "pkg__Account_Relationship__r"
	want := normalizeName(input)
	if got := names.canonical(input); got != want {
		t.Fatalf("canonical(%q) = %q, want %q", input, got, want)
	}
	var got string
	if allocs := testing.AllocsPerRun(1000, func() {
		got = names.canonical(input)
	}); allocs != 0 {
		t.Fatalf("cached canonical lookup allocations = %.2f, want 0", allocs)
	}
	if got != want {
		t.Fatalf("cached canonical(%q) = %q, want %q", input, got, want)
	}
}

func TestSemaCanonicalNamesConcurrentLookupsMatchNormalizeName(t *testing.T) {
	names := newSemaCanonicalNames(64)
	inputs := []string{
		"Account__c",
		"pkg__Account__r",
		"Feature__mdt",
		"Case__Share",
		"System.String",
		"pkg.Outer.Inner",
		"Straße",
	}
	var workers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for iteration := 0; iteration < 100; iteration++ {
				for _, input := range inputs {
					if got, want := names.canonical(input), normalizeName(input); got != want {
						t.Errorf("canonical(%q) = %q, want %q", input, got, want)
						return
					}
				}
			}
		}()
	}
	workers.Wait()
}

var benchmarkCanonicalNameSink string

func BenchmarkNormalizeNameRepeated(b *testing.B) {
	inputs := []string{
		"Account__c",
		"pkg__Account__r",
		"Feature__mdt",
		"Case__Share",
		"System.String",
		"pkg.Outer.Inner",
	}
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkCanonicalNameSink = normalizeName(inputs[i%len(inputs)])
		}
	})
	b.Run("analysis_local", func(b *testing.B) {
		names := newSemaCanonicalNames(64)
		for _, input := range inputs {
			_ = names.canonical(input)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkCanonicalNameSink = names.canonical(inputs[i%len(inputs)])
		}
	})
}
