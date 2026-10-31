package storage

import (
	"fmt"
	"testing"
)

// A synthetic wide standard object exercises repeated absent optional fields.
// Compare against the same run's simple gate-by-gate lookup so the bound is
// relative to the runner rather than a fixed wall-clock duration.
func TestOptionalFieldNoopGateLookupWork(t *testing.T) {
	org := NewOrgState()
	org.Features = []string{}
	EnsureStandardObject(&org, "Account")
	definition := org.Objects["Account"].Definition.Clone()
	for i := 0; i < 2048; i++ {
		name := fmt.Sprintf("Synthetic%04d__c", i)
		definition.Fields[name] = Field{APIName: name, Type: FieldString}
	}
	signature := definition.Metadata[standardFieldsOverlayMarker]
	if standardOptionalFieldsNeedRepair(definition, signature) {
		t.Fatal("synthetic prepared definition unexpectedly needs repair")
	}
	policy := standardFieldPolicyFor("Account")
	if policy == nil || len(policy.gates) < 8 {
		t.Fatal("fixture does not exercise multiple optional fields")
	}
	reference := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			for _, gate := range policy.gates {
				_, _ = ResolveFieldName(definition, "", gate.Field.Name)
			}
		}
	})
	actual := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			if standardOptionalFieldsNeedRepair(definition, signature) {
				b.Fatal("prepared definition changed")
			}
		}
	})
	t.Logf("no-op check %d ns, gate-by-gate reference %d ns", actual.NsPerOp(), reference.NsPerOp())
	if actual.NsPerOp()*2 >= reference.NsPerOp() {
		t.Fatalf("no-op check still repeats absent gate scans: %d ns vs %d ns", actual.NsPerOp(), reference.NsPerOp())
	}
}
