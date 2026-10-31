package storage

import (
	"fmt"
	"strings"
	"testing"
)

func TestOptionalGatePresenceCountBoundaries(t *testing.T) {
	for _, tc := range []struct {
		count       int
		wantIndexed bool
	}{{7, false}, {8, true}, {64, true}, {65, false}} {
		t.Run(fmt.Sprint(tc.count), func(t *testing.T) {
			policy := &standardFieldPolicyObject{gates: make(map[string]standardFieldPolicyGate)}
			for i := 0; i < tc.count; i++ {
				name := fmt.Sprintf("SyntheticGate%02d", i)
				policy.gates[strings.ToLower(name)] = standardFieldPolicyGate{Field: standardDescribeField{Name: name}}
			}
			definition := ObjectDefinition{Fields: map[string]Field{"syntheticgate00": {}}}
			index, indexed := optionalGatePresence(definition, policy)
			if indexed != tc.wantIndexed {
				t.Fatalf("indexed=%v, want %v", indexed, tc.wantIndexed)
			}
			if indexed && (!index.has("SyntheticGate00") || index.has("SyntheticGate01")) {
				t.Fatal("index changed field presence")
			}
		})
	}
}
