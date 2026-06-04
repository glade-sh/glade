package surfaceledger

import (
	"testing"

	"github.com/glade-sh/glade/internal/capability"
)

func TestRowsFromToolingCompletions(t *testing.T) {
	rows := RowsFromToolingCompletions(capability.ToolingCompletions{
		PublicDeclarations: map[string]map[string]capability.ToolingClassDecl{
			"System": {
				"Label": {
					Methods: []capability.ToolingMethod{{
						Name:       "get",
						ReturnType: "String",
						Parameters: []capability.ToolingParameter{
							{Name: "section", Type: "String"},
							{Name: "key", Type: "String"},
						},
					}},
				},
			},
		},
	})
	byID := rowsByID(rows)
	id := ApexMemberID("System", "Label", "get", []string{"String", "String"})
	if byID[id].Org != SourcePresent {
		t.Fatalf("org state for %s = %q", id, byID[id].Org)
	}
	if byID[id].ReturnType != "String" {
		t.Fatalf("return type = %q", byID[id].ReturnType)
	}
}

func TestRowsFromToolingCompletionsNormalizesApexObjectParameters(t *testing.T) {
	rows := RowsFromToolingCompletions(capability.ToolingCompletions{
		PublicDeclarations: map[string]map[string]capability.ToolingClassDecl{
			"System": {
				"Database": {
					Methods: []capability.ToolingMethod{{
						Name:       "executeBatch",
						ReturnType: "ID",
						Parameters: []capability.ToolingParameter{
							{Name: "batchClassObject", Type: "APEX_OBJECT"},
							{Name: "scope", Type: "Integer"},
						},
					}},
				},
			},
		},
	})

	id := ApexMemberID("System", "Database", "executeBatch", []string{"Object", "Integer"})
	if rowsByID(rows)[id].Org != SourcePresent {
		t.Fatalf("Tooling APEX_OBJECT parameter did not normalize to %s: %#v", id, rows)
	}
}

func TestRowsFromToolingCompletionsNormalizesGenericAnyParameters(t *testing.T) {
	rows := RowsFromToolingCompletions(capability.ToolingCompletions{
		PublicDeclarations: map[string]map[string]capability.ToolingClassDecl{
			"Database": {
				"Batchable": {
					Methods: []capability.ToolingMethod{{
						Name:       "execute",
						ReturnType: "void",
						Parameters: []capability.ToolingParameter{
							{Name: "context", Type: "Database.BatchableContext"},
							{Name: "scope", Type: "List<ANY>"},
						},
					}},
				},
			},
		},
	})

	id := ApexMemberID("Database", "Batchable", "execute", []string{"Database.BatchableContext", "List<Object>"})
	if rowsByID(rows)[id].Org != SourcePresent {
		t.Fatalf("Tooling List<ANY> parameter did not normalize to %s: %#v", id, rows)
	}
}

func TestCanonicalParameterTypeNormalizesSObjectGenericParameters(t *testing.T) {
	got := ApexMemberID("Database", "Batchable", "execute", []string{"Database.BatchableContext", "List<sObject>"})
	want := ApexMemberID("Database", "Batchable", "execute", []string{"Database.BatchableContext", "List<Object>"})
	if got != want {
		t.Fatalf("List<sObject> canonical id = %q, want %q", got, want)
	}
}

func TestDecodeToolingCompletionsAcceptsWrappedResult(t *testing.T) {
	completions, err := decodeToolingCompletions([]byte(`{"result":{"publicDeclarations":{"System":{"Label":{}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := completions.PublicDeclarations["System"]["Label"]; !ok {
		t.Fatalf("wrapped completions not decoded: %#v", completions)
	}
}
