package apextest

import (
	"crypto/sha256"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func orgDigest(t *testing.T, org storage.OrgState) [32]byte {
	t.Helper()
	// Digest the logical org. A lazy org holds the catalog objects something
	// touched, and what touched them depends on process-wide caches, so
	// complete a private copy first.
	org = org.Clone()
	storage.MaterializeStandardObjectSpread(&org)
	return sha256.Sum256([]byte(stringJSON(t, org)))
}

// Releasing the unshaped seed and the decoded catalogs must not change what a
// later caller sees: the shaped seed is unchanged and an unshaped rebuild
// produces the same org as before the release.
func TestReleaseStandardSeedMemoryKeepsSeeds(t *testing.T) {
	features := []string{"MultiCurrency"}
	domain := "https://release.my.salesforce.com"
	unshapedBefore := orgDigest(t, standardApexTestOrg())
	shapedBefore := orgDigest(t, shapedStandardApexTestOrg(features, domain))

	previous := storage.SetStandardCatalogReleaseEnabled(true)
	t.Cleanup(func() { storage.SetStandardCatalogReleaseEnabled(previous) })

	// A new shape key forces a shaped build, which releases at the end.
	otherDomain := "https://release-other.my.salesforce.com"
	_ = shapedStandardApexTestOrg(features, otherDomain)
	standardApexTestOrgCache.mu.Lock()
	built := standardApexTestOrgCache.built
	standardApexTestOrgCache.mu.Unlock()
	if built {
		t.Fatal("unshaped seed was not released after the shaped build")
	}
	if got := orgDigest(t, shapedStandardApexTestOrg(features, otherDomain)); got == shapedBefore {
		t.Fatal("different domain should differ from the first shaped seed")
	}
	if got := orgDigest(t, standardApexTestOrg()); got != unshapedBefore {
		t.Fatal("unshaped rebuild after release differs from the pre-release org")
	}
	if got := orgDigest(t, shapedStandardApexTestOrg(features, domain)); got != shapedBefore {
		t.Fatal("shaped seed rebuilt after release differs from the pre-release seed")
	}
}
