package apextest

import "testing"

func dynamicBindClosedIDSetNativeSpec() catalogBoundaryNativeSpec {
	return catalogBoundaryNativeSpec{
		file: "dynamic_bind_closed_id_set_native.json", observer: "OwnedClosedIdSetObserver", testClass: "OwnedClosedIdSetTest",
		rows: 40, cells: 80, methods: 5, matched: 40, carried: 0, matchedCells: 80,
		routes: map[string][]string{
			"CB01": {"anonymous", "istest"},
			"CB02": {"anonymous", "istest"},
			"CB03": {"anonymous", "istest"},
			"CB04": {"anonymous", "istest"},
			"CB05": {"anonymous", "istest"},
		},
	}
}

func TestDynamicBindClosedIDSetNativeFixture(t *testing.T) {
	validateCatalogBoundaryNativeFixture(t, dynamicBindClosedIDSetNativeSpec())
}

func TestDynamicBindClosedIDSetNativeConformance(t *testing.T) {
	runCatalogBoundaryNativeConformance(t, dynamicBindClosedIDSetNativeSpec())
}
