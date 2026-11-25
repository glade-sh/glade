package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func fieldScanFixture() (*storage.OrgState, string) {
	org := storage.NewOrgState()
	for _, name := range []string{"Account", "Contact"} {
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{}}}
	}
	var source strings.Builder
	source.WriteString("public class FieldScanFixture { public void run() {\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&source, "Account a%d = new Account(Name = 'a', Extra%d__c = 'x'); Contact c%d = new Contact(AccountId = a%d.Id, Other%d__c = 'y');\n", i, i, i, i, i)
	}
	source.WriteString("// new Account(CommentOnly__c = 'x')\nString ignored = 'Contact(Fake__c = 1)';\n} }")
	return &org, source.String()
}

func TestFieldScanLargeClassOutcome(t *testing.T) {
	org, source := fieldScanFixture()
	result := scanProjectReferencedStandardFieldsFromSource(org, source, nil)
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "fbac41f4fc3219a108c2d370edc7530a775c2cf2d14f6e9b30925a3b56c57278" {
		t.Fatalf("field scan outcome differs from baseline: %s", got)
	}
	if _, ok := result.Inferred["Account"]["CommentOnly__c"]; ok {
		t.Fatal("comment became a field")
	}
	if _, ok := result.Inferred["Contact"]["Fake__c"]; ok {
		t.Fatal("string became a field")
	}
}

func BenchmarkFieldScanLargeClass(b *testing.B) {
	org, source := fieldScanFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanProjectReferencedStandardFieldsFromSource(org, source, nil)
	}
}

func TestFieldScanDottedReferenceAllocationBudget(t *testing.T) {
	source := strings.Repeat("a.Field;\n", 300)
	allocs := testing.AllocsPerRun(10, func() {
		matches := standardFieldMatchesFromSource(source)
		if len(matches.staticFields) != 300 || len(matches.variableField) != 300 {
			t.Fatal("dotted reference matches changed")
		}
	})
	if allocs > 450 {
		t.Fatalf("dotted references allocated %.0f times, want at most 450", allocs)
	}
}

func TestFieldScanPreservesLookupSpellingAndPrecedence(t *testing.T) {
	org, _ := fieldScanFixture()
	source := `Account parent = new Account();
		Contact c = new Contact(Link__c = parent.Id, link__C = null);
		Contact.LINK__c;
		Contact.Visible__c;
		c.Other__c;
		Setting__c . GeTaLl ();
		// FROM Account.Children__r
		String ignored = 'Contact.Hidden__c';`
	result := scanProjectReferencedStandardFieldsFromSource(org, source, projectReferencedChildRelationshipLookup(*org))
	fields := result.Inferred["Contact"]
	link, ok := fields["Link__c"]
	if !ok || link.Type != storage.FieldReference || len(link.ReferenceTo) != 1 || link.ReferenceTo[0] != "Account" {
		t.Fatalf("inferred lookup = %#v", link)
	}
	for _, name := range []string{"Visible__c", "Other__c"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("missing field %s", name)
		}
	}
	if len(fields) != 3 {
		t.Fatalf("inferred %d Contact fields, want 3", len(fields))
	}
	if _, ok := result.ListCustomSettings["Setting__c"]; !ok || len(result.ChildRelationshipRefs) != 1 {
		t.Fatal("expected list setting and raw child relationship reference")
	}
}

func BenchmarkFieldScanNoReferences(b *testing.B) {
	org, _ := fieldScanFixture()
	lookup := projectReferencedChildRelationshipLookup(*org)
	source := "public class Plain { void run() {\n" + strings.Repeat("Integer value = 1;\n", 800) + "} }"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanProjectReferencedStandardFieldsFromSource(org, source, lookup)
	}
}
