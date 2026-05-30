package surface

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinRegistryNamesParallelSurfaceSlices(t *testing.T) {
	registry := BuiltinRegistry()
	if err := registry.Validate(); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, descriptor := range registry.Surfaces {
		seen[descriptor.Name] = true
		if descriptor.RuntimePackage == "" && descriptor.ServerPackage == "" {
			t.Fatalf("%s has no runtime or server owner package", descriptor.Name)
		}
		if len(descriptor.TestCommands) == 0 {
			t.Fatalf("%s has no focused test command", descriptor.Name)
		}
	}

	for _, name := range []string{
		"ConnectApi",
		"Metadata",
		"Reports",
		"ApexPages",
		"Tooling",
		"Bulk",
		"Composite",
		"DML automation",
		"SOQL/SOSL",
		"Platform Cache",
	} {
		if !seen[name] {
			t.Fatalf("builtin surface registry missing %q", name)
		}
	}
}

// TestSurfaceOwnerPackagesExist keeps the surface registry honest: every owner
// package named by a descriptor must exist on disk, and the runbook that points
// contributors at these packages must be present. This is the enforcement that
// makes internal/surface a usable index for "where do I add this Salesforce
// API?" rather than documentation that silently drifts.
func TestSurfaceOwnerPackagesExist(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	for _, descriptor := range BuiltinRegistry().Surfaces {
		for _, pkg := range []string{descriptor.RuntimePackage, descriptor.ServerPackage, descriptor.CapabilityPackage, descriptor.CompatPackage} {
			if pkg == "" {
				continue
			}
			dir := filepath.Join(repoRoot, filepath.FromSlash(pkg))
			info, err := os.Stat(dir)
			if err != nil || !info.IsDir() {
				t.Fatalf("surface %q names package %q that does not exist at %s", descriptor.Name, pkg, dir)
			}
		}
	}

	runbook := filepath.Join(repoRoot, "docs", "ADDING_A_PLATFORM_API.md")
	if _, err := os.Stat(runbook); err != nil {
		t.Fatalf("missing platform-API runbook at %s: %v", runbook, err)
	}
}
