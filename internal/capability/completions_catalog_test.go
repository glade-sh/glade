package capability

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexdocs"
)

func sampleToolingCompletions() ToolingCompletions {
	return ToolingCompletions{
		PublicDeclarations: map[string]map[string]ToolingClassDecl{
			"System": {
				"Math": {
					Methods: []ToolingMethod{
						{Name: "abs", ReturnType: "Decimal", IsStatic: true, Parameters: []ToolingParameter{{Name: "x", Type: "Decimal"}}},
						{Name: "max", ReturnType: "Integer", IsStatic: true, Parameters: []ToolingParameter{{Name: "a", Type: "Integer"}, {Name: "b", Type: "Integer"}}},
					},
					Properties: []ToolingProperty{{Name: "PI", Type: "Double"}},
				},
			},
			"Approval": {
				"ProcessSubmitRequest": {
					Constructors: []ToolingConstructor{{Name: "ProcessSubmitRequest"}},
					Methods: []ToolingMethod{
						{Name: "setObjectId", ReturnType: "void", Parameters: []ToolingParameter{{Name: "objectId", Type: "Id"}}},
					},
				},
			},
		},
	}
}

func TestInventoryFromToolingCompletions(t *testing.T) {
	inv := InventoryFromToolingCompletions(sampleToolingCompletions())
	if inv.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2", inv.TotalFiles)
	}
	if inv.TotalMembers != 5 {
		t.Fatalf("TotalMembers = %d, want 5", inv.TotalMembers)
	}

	math := findDocument(inv, "Math")
	if math == nil {
		t.Fatal("Math document missing")
	}
	if math.Namespace != "System" {
		t.Fatalf("Math namespace = %q, want System", math.Namespace)
	}
	sigs := map[string]string{}
	kinds := map[string]string{}
	for _, m := range math.Members {
		sigs[m.Name] = m.Signature
		kinds[m.Name] = m.Kind
	}
	if got := sigs["abs"]; got != "abs(Decimal x) returns Decimal" {
		t.Fatalf("abs signature = %q", got)
	}
	if got := sigs["max"]; got != "max(Integer a, Integer b) returns Integer" {
		t.Fatalf("max signature = %q", got)
	}
	if got := sigs["PI"]; got != "PI : Double" {
		t.Fatalf("PI signature = %q", got)
	}
	if kinds["abs"] != "method" || kinds["PI"] != "property" {
		t.Fatalf("unexpected kinds: %v", kinds)
	}
}

func TestBuildCatalogFromCompletions(t *testing.T) {
	catalog := BuildCatalogFromCompletions(sampleToolingCompletions())
	if catalog.SchemaVersion != CatalogSchemaVersion {
		t.Fatalf("schema version = %d", catalog.SchemaVersion)
	}

	var mathAbs *CatalogEntry
	for i := range catalog.Entries {
		e := &catalog.Entries[i]
		if e.TypeName == "Math" && e.MemberName == "abs" {
			mathAbs = e
			break
		}
	}
	if mathAbs == nil {
		t.Fatal("Math.abs entry missing")
	}
	// Math is core stdlib under System: executable-parity target.
	if mathAbs.Target != TargetExecutableParity {
		t.Fatalf("Math.abs target = %q, want executable-parity", mathAbs.Target)
	}
	if mathAbs.Area != "Core stdlib" {
		t.Fatalf("Math.abs area = %q", mathAbs.Area)
	}

	// Product namespace types flow through as typed stubs, proving breadth
	// beyond the documented core.
	var approval *CatalogEntry
	for i := range catalog.Entries {
		e := &catalog.Entries[i]
		if e.Namespace == "Approval" && e.TypeName == "ProcessSubmitRequest" && e.MemberName == "" {
			approval = e
			break
		}
	}
	if approval == nil {
		t.Fatal("Approval.ProcessSubmitRequest entry missing")
	}
}

func TestBuildCatalogFromCompletionsDeterministic(t *testing.T) {
	a := BuildCatalogFromCompletions(sampleToolingCompletions())
	b := BuildCatalogFromCompletions(sampleToolingCompletions())
	if len(a.Entries) != len(b.Entries) {
		t.Fatalf("entry count drift: %d vs %d", len(a.Entries), len(b.Entries))
	}
	for i := range a.Entries {
		if a.Entries[i].ID != b.Entries[i].ID {
			t.Fatalf("entry order drift at %d", i)
		}
	}
}

func findDocument(inv apexdocs.Inventory, name string) *apexdocs.Document {
	for i := range inv.Documents {
		if inv.Documents[i].Name == name {
			return &inv.Documents[i]
		}
	}
	return nil
}
