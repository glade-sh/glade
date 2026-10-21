package automation

import (
	"fmt"
	"strings"
	"testing"
)

func TestFlowFormulaResourceExpansionSafety(t *testing.T) {
	t.Run("quoted_text", func(t *testing.T) {
		got := expandFlowFormulaReferences(`"{!Leaf}" & {!Leaf}`, map[string]string{"leaf": "1"}, map[string]bool{})
		if got != `"{!Leaf}" & (1)` {
			t.Fatalf("quoted resource expanded: %s", got)
		}
	})
	t.Run("cycle", func(t *testing.T) {
		got := flowFormulaMap([]flowFormulaXML{{Name: "Alpha", Expression: "{!Beta}+1"}, {Name: "Beta", Expression: "{!Alpha}+2"}})
		for _, expression := range got {
			if len(expression) > 100 {
				t.Fatal("cyclic resources expanded without a bound")
			}
		}
	})
	for _, tc := range []struct {
		name         string
		width, depth int
	}{
		{"repeated_tree", 2, 16}, {"deep_chain", 1, 80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			declarations := []flowFormulaXML{{Name: "Base", Expression: "1"}}
			previous := "Base"
			for i := 0; i < tc.depth; i++ {
				name := fmt.Sprintf("Node%d", i)
				reference := "{!" + previous + "}"
				expression := reference + "+1"
				if tc.width == 2 {
					expression = reference + "+" + reference
				}
				declarations = append(declarations, flowFormulaXML{Name: name, Expression: expression})
				previous = name
			}
			expression := flowFormulaMap(declarations)[strings.ToLower(previous)]
			if tc.width == 2 && len(expression) > 64<<10 {
				t.Fatalf("resource tree expanded to %d bytes", len(expression))
			}
			if tc.width == 1 && strings.Count(expression, "(") > 32 {
				t.Fatalf("resource expansion nested %d levels", strings.Count(expression, "("))
			}
		})
	}
}
