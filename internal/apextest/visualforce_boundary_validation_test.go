package apextest

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Visualforce inputs are observed when a run captures them. Later cache
// boundaries of the same run do not stat them again until a snapshot stage
// advances the epoch; the raw check always rereads them.
func TestVisualforceInputsAreCheckedOncePerSnapshotEpoch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Home.page")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("<apex:page/>")
	digest := sha256.Sum256([]byte("<apex:page/>"))
	cache := newSourceCache()
	cache.capturedMetadata = map[string]typesys.ApexMetadataInput{path: {Present: true, Digest: digest}}
	cache.visualforceInputs = map[string]bool{path: true}

	// Unverified binding: the first boundary reads the file and passes.
	if err := cache.validateCapturedSourceGenerationAtBoundary(); err != nil {
		t.Fatal(err)
	}
	write("<apex:page>changed</apex:page>")
	if err := cache.validateCapturedSourceGenerationAtBoundary(); err != nil {
		t.Fatalf("same-epoch boundary rechecked a verified Visualforce input: %v", err)
	}
	snapshotValidationEpoch.Add(1)
	if err := cache.validateCapturedSourceGenerationAtBoundary(); err == nil {
		t.Fatal("a new snapshot epoch did not detect the changed Visualforce input")
	}

	if err := cache.validateCapturedSourceGenerationRaw(); err == nil {
		t.Fatal("the raw check did not reread the changed Visualforce input")
	}

	// A fresh capture of unchanged inputs starts verified, and a rebinding that
	// drops the Visualforce set clears the shortcut.
	cache.vfVerified, cache.vfEpoch = false, 0
	write("<apex:page/>")
	if err := cache.validateCapturedSourceGenerationAtBoundary(); err != nil {
		t.Fatal(err)
	}
	cache.visualforceInputs, cache.vfVerified = nil, false
	write("<apex:page>again</apex:page>")
	if err := cache.validateCapturedSourceGenerationAtBoundary(); err == nil {
		t.Fatal("changed input passed after the Visualforce binding was cleared")
	}
}
