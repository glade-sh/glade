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

func TestDecodeToolingCompletionsAcceptsWrappedResult(t *testing.T) {
	completions, err := decodeToolingCompletions([]byte(`{"result":{"publicDeclarations":{"System":{"Label":{}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := completions.PublicDeclarations["System"]["Label"]; !ok {
		t.Fatalf("wrapped completions not decoded: %#v", completions)
	}
}
