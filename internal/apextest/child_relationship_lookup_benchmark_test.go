package apextest

import "testing"

func BenchmarkChildRelationshipLookupLazyShapedOrg(b *testing.B) {
	seed := childRelationshipLookupOrg(childRelationshipLookupShapes["unspecified"], "")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		org := seed.CloneRuntimeFrozenDefinition()
		b.StartTimer()
		projectReferencedChildRelationshipLookup(org)
	}
}
