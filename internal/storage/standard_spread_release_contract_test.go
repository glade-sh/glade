package storage

import "testing"

// A completed source-backed spread releases its decoded catalog when the
// opt-in is enabled, while keeping the completed org usable. This test also
// compiles on the release implementation parent f06f2a723b^, where it fails
// because the release count remains zero.
func TestCompletedSpreadReleasesCatalogContract(t *testing.T) {
	old := SetStandardCatalogReleaseEnabled(true)
	defer SetStandardCatalogReleaseEnabled(old)
	lazy := sourceSpreadLazyOrg([]string{"Communities"})
	before := StandardCatalogReleaseCount()
	if !MaterializeStandardObjectSpread(&lazy) {
		t.Fatal("spread did not complete")
	}
	if got := StandardCatalogReleaseCount(); got <= before {
		t.Fatalf("catalog releases = %d before, %d after", before, got)
	}
	if got := StandardCatalogDecodedObjectCount(); got != 0 {
		t.Fatalf("decoded objects = %d, want 0", got)
	}
}
