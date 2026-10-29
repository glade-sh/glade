package storage

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// deriveReferenceOnlyStandardObjectNames returns every reference target named by
// a standard describe catalog field that is not a known standard object.
func deriveReferenceOnlyStandardObjectNames(t *testing.T) []string {
	t.Helper()
	known := map[string]bool{}
	for name := range buildKnownStandardObjectNameSetWithoutReferenceOnly() {
		known[standardObjectLookupKey(name)] = true
	}
	found := map[string]string{}
	for _, entry := range standardDescribeCatalogV2Index {
		describe, ok, err := lookupStandardDescribeCatalogV2(entry.Name)
		if err != nil || !ok {
			t.Fatalf("catalog member %s: found=%v err=%v", entry.Name, ok, err)
		}
		for _, field := range describe.Fields {
			for _, target := range field.ReferenceTo {
				if target == "" || strings.Contains(target, "__") || known[standardObjectLookupKey(target)] {
					continue
				}
				found[standardObjectLookupKey(target)] = target
			}
		}
	}
	out := make([]string, 0, len(found))
	for _, name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Regenerate with: GLADE_UPDATE_REFERENCE_ONLY=1 go test -run TestGenerateReferenceOnlyStandardObjectNames ./internal/storage
func TestGenerateReferenceOnlyStandardObjectNames(t *testing.T) {
	names := deriveReferenceOnlyStandardObjectNames(t)
	var b strings.Builder
	b.WriteString("// Code generated from standard_describe_catalog_v2.pack by TestGenerateReferenceOnlyStandardObjectNames; DO NOT EDIT.\n\npackage storage\n\n")
	b.WriteString("// Reference targets named by catalog fields that are not known standard objects.\nvar referenceOnlyStandardObjectNames = []string{\n")
	for _, name := range names {
		fmt.Fprintf(&b, "\t%q,\n", name)
	}
	b.WriteString("}\n")
	if os.Getenv("GLADE_UPDATE_REFERENCE_ONLY") == "1" {
		if err := os.WriteFile("standard_reference_only_generated.go", []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile("standard_reference_only_generated.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != b.String() {
		t.Fatalf("standard_reference_only_generated.go is stale; rerun with GLADE_UPDATE_REFERENCE_ONLY=1 (derived %d names)", len(names))
	}
}

func TestReferenceOnlyStandardObjectsAreDerivedFromCatalog(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	defer resetKnownStandardObjectCacheForTest()
	names := ReferenceOnlyStandardObjectNames()
	if len(names) != 99 {
		t.Fatalf("reference-only names = %d, want 99 (derived from the catalog; regenerate deliberately)", len(names))
	}
	for _, name := range []string{"AssessmentStagedData", "DocGenerationBatchProcess", "ActivationTarget", "DataAction", "DataStream", "ExpressionSetDefinition", "BusinessUnit"} {
		if !IsReferenceOnlyStandardObject(strings.ToLower(name)) {
			t.Errorf("%s is not reference-only", name)
		}
	}
	for _, name := range names {
		if canonical, ok := ResolveKnownStandardObjectName(name); ok {
			t.Fatalf("reference-only %s resolves as standalone object %q", name, canonical)
		}
		if _, found, _ := lookupStandardDescribeCatalogV2(name); found {
			t.Fatalf("reference-only %s has a catalog describe", name)
		}
	}
	for _, name := range []string{"Account", "Network", "Assessment", "Foo__c"} {
		if IsReferenceOnlyStandardObject(name) {
			t.Errorf("%s must not be reference-only", name)
		}
	}
}
