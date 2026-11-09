package storage

import "testing"

// standard_webstore_preserve_null/org.tsv:2-7 records the forced insert value.
func TestStandardInsertForcedValuesPolicy(t *testing.T) {
	for _, object := range []string{"WebStore", "webstore"} {
		values := StandardObjectInsertForcedValues(object)
		if len(values) != 1 || !values["OptionsPreserveGuestCartEnabled"] {
			t.Fatalf("%s: %v", object, values)
		}
		values["OptionsPreserveGuestCartEnabled"] = false
		if !StandardObjectInsertForcedValues(object)["OptionsPreserveGuestCartEnabled"] {
			t.Fatalf("%s: policy was mutated", object)
		}
	}
	if got := StandardObjectInsertForcedValues("Account"); len(got) != 0 {
		t.Fatalf("Account: %v", got)
	}
}
