package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const releaseProbeObject = `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <label>Gizmo</label>
    <pluralLabel>Gizmos</pluralLabel>
    <nameField><label>Gizmo Name</label><type>Text</type></nameField>
    <deploymentStatus>Deployed</deploymentStatus>
    <sharingModel>ReadWrite</sharingModel>
</CustomObject>`

func releaseProbeIndex(t *testing.T) typesys.Index {
	t.Helper()
	index := nativeBoundaryIndex(t, "67.0", map[string]string{
		"Probe": "public class Probe { public static Integer n(List<Gizmo__c> rows) { return rows.size(); } }",
	}, map[string]string{"Gizmo__c": releaseProbeObject})
	if len(index.Objects) == 0 {
		t.Fatal("index has no custom object")
	}
	return index
}

// resetUnshapedCatalogRelease lets a test observe the first unshaped release
// of a process again.
func resetUnshapedCatalogRelease(t *testing.T) {
	t.Helper()
	was := unshapedCatalogReleased.Swap(false)
	t.Cleanup(func() { unshapedCatalogReleased.Store(was) })
}

// A project with a custom object cannot start from the shaped seed. Its org
// build must still release the decoded describe catalogs when the process
// opted in, and the org it returns must be the same as without the release.
func TestOrgFromIndexCustomObjectReleasesCatalogs(t *testing.T) {
	index := releaseProbeIndex(t)
	previous := storage.SetStandardCatalogReleaseEnabled(false)
	t.Cleanup(func() { storage.SetStandardCatalogReleaseEnabled(previous) })
	resetUnshapedCatalogRelease(t)

	kept, err := orgFromIndexWithError(index)
	if err != nil {
		t.Fatal(err)
	}
	want := orgDigest(t, kept)
	if storage.StandardCatalogDecodedObjectCount() == 0 {
		t.Fatal("org build without release left no decoded catalog objects")
	}

	storage.SetStandardCatalogReleaseEnabled(true)
	released, err := orgFromIndexWithError(index)
	if err != nil {
		t.Fatal(err)
	}
	if got := storage.StandardCatalogDecodedObjectCount(); got != 0 {
		t.Fatalf("custom-object org build kept %d decoded catalog objects", got)
	}
	if got := orgDigest(t, released); got != want {
		t.Fatal("org built with the catalog release differs from the org built without it")
	}
	standardApexTestOrgCache.mu.Lock()
	built := standardApexTestOrgCache.built
	standardApexTestOrgCache.mu.Unlock()
	if !built {
		t.Fatal("unshaped seed was dropped; only the catalogs should be released")
	}
	// A rebuild after the release sees the same org again.
	if got := orgDigest(t, func() storage.OrgState { org, _ := orgFromIndexWithError(index); return org }()); got != want {
		t.Fatal("org rebuilt after the release differs")
	}
}

// Long-lived processes (server, daemon, dev loop) build the unshaped org
// repeatedly. Only the first build releases; later builds keep what they
// decode, so each one does not pay the whole catalog decode again.
func TestOrgFromIndexCustomObjectReleasesCatalogsOnce(t *testing.T) {
	index := releaseProbeIndex(t)
	previous := storage.SetStandardCatalogReleaseEnabled(true)
	t.Cleanup(func() { storage.SetStandardCatalogReleaseEnabled(previous) })
	resetUnshapedCatalogRelease(t)

	before := storage.StandardCatalogReleaseCount()
	var digests [3][32]byte
	var decoded [3]int
	for i := range digests {
		org, err := orgFromIndexWithError(index)
		if err != nil {
			t.Fatal(err)
		}
		digests[i] = orgDigest(t, org)
		decoded[i] = storage.StandardCatalogDecodedObjectCount()
	}
	if got := storage.StandardCatalogReleaseCount() - before; got != 1 {
		t.Fatalf("three unshaped builds released the catalogs %d times; want 1", got)
	}
	if decoded[0] != 0 {
		t.Fatalf("first build kept %d decoded catalog objects", decoded[0])
	}
	if decoded[2] < decoded[1] {
		t.Fatalf("third build dropped decoded catalog objects: %d after the second, %d after the third", decoded[1], decoded[2])
	}
	if digests[1] != digests[0] || digests[2] != digests[0] {
		t.Fatal("repeated builds differ")
	}
}
