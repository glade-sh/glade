package scripts

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The checked-in catalog must be reproducible from its native captures and
// reviewed picklist allowlist. The generator's check mode does not write files.
func TestStandardStubRestrictedPicklistCatalogGenerated(t *testing.T) {
	cmd := exec.Command("python3", "generate-standard-stub-restricted-picklists.py", "--check")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restricted picklist catalog generation: %v\n%s", err, output)
	}
	target := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(target, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("python3", "generate-standard-stub-restricted-picklists.py", "--check", "--output", target)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("stale catalog passed generation check: %s", output)
	}
	cmd = exec.Command("python3", "generate-standard-stub-restricted-picklists.py", "--output", target)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("regenerate catalog: %v\n%s", err, output)
	}
	want, err := os.ReadFile("../internal/storage/standard_stub_restricted_picklists.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("regenerated restricted picklist catalog differs from the checked-in file")
	}
}
