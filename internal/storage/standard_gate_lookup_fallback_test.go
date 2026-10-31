package storage

import (
	"fmt"
	"strings"
	"testing"
)

func TestOptionalGatePresenceMatchesResolver(t *testing.T) {
	policy := &standardFieldPolicyObject{gates: make(map[string]standardFieldPolicyGate)}
	for _, name := range []string{"Status", "Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta"} {
		policy.gates[strings.ToLower(name)] = standardFieldPolicyGate{Field: standardDescribeField{Name: name}}
	}
	definition := ObjectDefinition{Fields: map[string]Field{"STATUS": {}, "beta": {}, "Other__c": {}}}
	presence, indexed := optionalGatePresence(definition, policy)
	if !indexed {
		t.Fatal("ordinary ASCII inputs did not use the index")
	}
	for _, gate := range policy.gates {
		_, want := ResolveFieldName(definition, "", gate.Field.Name)
		if got := presence.has(gate.Field.Name); got != want {
			t.Fatalf("%s: indexed=%v resolver=%v", gate.Field.Name, got, want)
		}
	}
	// Unicode case-folding includes the long s, which ASCII lowercasing alone
	// cannot match. The whole check must fall back to the normal resolver.
	definition.Fields["ſtatus"] = Field{}
	delete(definition.Fields, "STATUS")
	if _, indexed = optionalGatePresence(definition, policy); indexed {
		t.Fatal("non-ASCII field did not fall back")
	}
	if _, ok := ResolveFieldName(definition, "", "Status"); !ok {
		t.Fatal("Unicode case-fold control did not resolve")
	}
	for _, name := range []string{"Id", "Place__Latitude__s", "naïve"} {
		t.Run(fmt.Sprintf("special=%s", name), func(t *testing.T) {
			copyPolicy := &standardFieldPolicyObject{gates: make(map[string]standardFieldPolicyGate)}
			for key, gate := range policy.gates {
				copyPolicy.gates[key] = gate
			}
			copyPolicy.gates[strings.ToLower(name)] = standardFieldPolicyGate{Field: standardDescribeField{Name: name}}
			if _, indexed := optionalGatePresence(ObjectDefinition{Fields: map[string]Field{}}, copyPolicy); indexed {
				t.Fatalf("special gate %q did not fall back", name)
			}
		})
	}
}
