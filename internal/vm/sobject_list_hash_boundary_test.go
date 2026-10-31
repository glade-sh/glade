package vm

import "testing"

// Uncaptured list shapes and APIs preserve the pre-existing hash path.
func TestSObjectListHashPreservesUncapturedBoundaries(t *testing.T) {
	for _, api := range []string{"61.0", "67.0", "69.0"} {
		for _, tc := range []struct {
			name  string
			other Value
		}{
			{"unknown-record", Object("Uncaptured__c")},
			{"primitive", String("other")},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				machine := New(nil)
				machine.currentMethod = Method{APIVersion: api}
				list := typedList("List<Object>")
				list.List = []Value{Object("Account"), tc.other}
				got, err := machine.systemCustomHashCode(list, &Result{})
				want := int64(valueHashCode(list))
				if err != nil || got.Kind != ValueInt || got.Int != want {
					t.Fatalf("hash = %v, err %v; preserved hash = %d", got, err, want)
				}
			})
		}
	}
}
