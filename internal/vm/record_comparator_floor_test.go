package vm

import "testing"

// API61 named rows establish the admitted floor; preserve the existing
// rejection for unobserved concrete-record source versions and class shapes.
func TestRecordComparatorCapturedFloorBounds(t *testing.T) {
	for _, api := range []string{"60.0", "61.0", "62.0", "67.0", "68.0", "69.0"} {
		for _, argument := range []string{"SObject", "Opportunity"} {
			t.Run(api+"/"+argument, func(t *testing.T) {
				machine := New(nil)
				machine.currentMethod.APIVersion = api
				class := Class{Name: "RecordOrderingFixture.Order"}
				want := api != "60.0" && api != "69.0" && (argument == "SObject" || api == "61.0")
				if got := machine.comparatorClassCastPair(class, argument, "Object"); got != want {
					t.Fatalf("captured cast=%t want=%t", got, want)
				}
				class.Namespace = "provider"
				if machine.comparatorClassCastPair(class, argument, "Object") {
					t.Fatal("namespace scope widened")
				}
				class.Namespace = ""
				class.SuperClass = "ParentOrder"
				if machine.comparatorClassCastPair(class, argument, "Object") {
					t.Fatal("inherited scope widened")
				}
			})
		}
	}
}
