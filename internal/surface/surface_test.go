package surface

import "testing"

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
