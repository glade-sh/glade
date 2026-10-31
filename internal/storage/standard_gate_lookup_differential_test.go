package storage

import (
	"strings"
	"testing"
)

// The production gate set is static, but field keys remain mutable. Compare
// every gate with the established resolver across two synthetic key shapes.
func TestOptionalGatePresenceMatchesEveryPolicyGate(t *testing.T) {
	policy := standardFieldPolicyFor("Account")
	if policy == nil || len(policy.gates) < 8 || len(policy.gates) > 64 {
		t.Fatal("fixture has no eligible gate set")
	}
	for _, upper := range []bool{false, true} {
		definition := ObjectDefinition{Fields: make(map[string]Field)}
		i := 0
		for _, gate := range policy.gates {
			if i%2 == 0 {
				name := strings.ToLower(gate.Field.Name)
				if upper {
					name = strings.ToUpper(name)
				}
				definition.Fields[name] = Field{APIName: name}
			}
			i++
		}
		index, indexed := optionalGatePresence(definition, policy)
		if !indexed {
			t.Fatal("ordinary policy inputs did not use the index")
		}
		for _, gate := range policy.gates {
			_, want := ResolveFieldName(definition, "", gate.Field.Name)
			if got := index.has(gate.Field.Name); got != want {
				t.Fatalf("%s: indexed=%v resolver=%v", gate.Field.Name, got, want)
			}
		}
	}
}
