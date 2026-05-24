package vm

import "testing"

func TestPlatformObjectMemberSurfaceRegistryNamesStable(t *testing.T) {
	names := platformObjectMemberSurfaceNames()
	if len(names) == 0 {
		t.Fatal("platform object member surface registry is empty")
	}

	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" {
			t.Fatal("platform object member surface registry contains an empty name")
		}
		if seen[name] {
			t.Fatalf("platform object member surface registry contains duplicate name %q", name)
		}
		seen[name] = true
	}

	for _, name := range []string{
		"sfsqlquery-harness",
		"org-instrumentation",
		"commerce-inventory",
		"packaged-controller",
		"wave-query",
		"slack-local-harness",
	} {
		if !seen[name] {
			t.Fatalf("platform object member surface registry missing %q", name)
		}
	}
}
