package vm

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The golden bytes were captured from origin/main using the same fixture and call sequence.
var childDescribeGoldenCalls = map[string]int{}

func goldenChildDescribe(t *testing.T, _ *VM, name string) []byte {
	t.Helper()
	base := t.Name() + "|" + name
	ordinal := childDescribeGoldenCalls[base]
	if ordinal == 0 {
		t.Cleanup(func() { delete(childDescribeGoldenCalls, base) })
	}
	childDescribeGoldenCalls[base] = ordinal + 1
	key := fmt.Sprintf("%s|%d", base, ordinal)
	file, err := os.Open("testdata/child_describe_parent.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	want, ok := entries[key]
	if !ok {
		t.Fatalf("missing parent golden for %s", key)
	}
	return want
}

func compareChildDescribeWithBaseline(t *testing.T, machine *VM, name string) {
	t.Helper()
	got, err := json.Marshal(machine.describeChildRelationships(name))
	if err != nil {
		t.Fatal(err)
	}
	want := goldenChildDescribe(t, machine, name)
	if !bytes.Equal(got, want) {
		t.Fatalf("child describe %q differs from parent (got %d bytes, want %d)", name, len(got), len(want))
	}
}

func describePruningTemplate() RestoredRuntimeTemplate {
	org := shapedLazyOrg([]string{"MultiCurrency"})
	for i := 0; i < 120; i++ {
		parent := fmt.Sprintf("Root%03d__c", i)
		child := fmt.Sprintf("Branch%03d__c", i)
		org.Objects[parent] = candidatePlanObject(parent)
		state := candidatePlanChild(child, parent, "Children__r", 4)
		storage.EnsureStandardObjectFields(&state.Definition)
		org.Objects[child] = state
	}
	org.ClearRuntimeSchemaStamp()
	return NewRestoredRuntimeTemplate(org, New(nil))
}

func describePruningClone(template RestoredRuntimeTemplate) *VM {
	org := template.CloneOrg()
	machine := template.CloneMachine(nil)
	machine.SetRuntimeTemplateOrg(&org)
	return machine
}

func TestDescribeChildPruningMatchesBaseline(t *testing.T) {
	template := describePruningTemplate()
	for _, name := range []string{"Root000__c", "root001__C", "Account", "User", "Missing__c", "Ωther__c"} {
		t.Run(name, func(t *testing.T) {
			compareChildDescribeWithBaseline(t, describePruningClone(template), name)
		})
	}
}

func TestDescribeChildPruningInvalidation(t *testing.T) {
	template := describePruningTemplate()
	machine := describePruningClone(template)
	_ = machine.describeChildRelationships("Root000__c")
	definition, ok := storage.EnsureMutableObjectDefinition(machine.Org, "Branch000__c")
	if !ok {
		t.Fatal("missing child definition")
	}
	for i := range definition.Relations {
		if definition.Relations[i].Field == "Parent__c" {
			definition.Relations[i].ParentObjects = []string{"Root001__c"}
		}
	}
	field := definition.Fields["Parent__c"]
	field.ReferenceTo = []string{"Root001__c"}
	definition.Fields["Parent__c"] = field
	machine.SetRuntimeTemplateOrg(machine.Org)
	compareChildDescribeWithBaseline(t, machine, "Root001__c")
	compareChildDescribeWithBaseline(t, describePruningClone(template), "Root000__c")
}

func BenchmarkDescribeChildRelationshipsSpread(b *testing.B) {
	template := describePruningTemplate()
	names := []string{"Root000__c", "Root001__c", "Root002__c", "Root003__c", "Account", "Contact"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		machine := describePruningClone(template)
		for _, name := range names {
			_ = machine.describeChildRelationships(name)
		}
	}
}
