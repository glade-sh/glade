package storage

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestSpreadSuppressionConcurrentAccess(t *testing.T) {
	registry := newSpreadSuppression()
	org := NewOrgState()
	org.StandardSpreadSuppressed = registry
	other := org
	other.StandardSpreadSuppressed = newSpreadSuppression()
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				name := fmt.Sprintf("object-%d-%d", worker, i)
				registry.put(name, "fingerprint")
				if got, ok := registry.lookup(name); !ok || got != "fingerprint" {
					t.Errorf("missing registry entry %s", name)
				}
				if _, err := json.Marshal(registry); err != nil {
					t.Error(err)
				}
				if org.PersistedFieldsIdentical(other) {
					t.Error("different registries compared equal")
				}
			}
		}(worker)
	}
	wg.Wait()
	if got := len(registry.snapshot()); got != 160 {
		t.Fatalf("registry entries %d, want 160", got)
	}
}

func TestSpreadSuppressionIsolatedAcrossRuntimeClones(t *testing.T) {
	org := NewOrgState()
	org.EnableStandardSpreadSource()
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := ResolveObjectName(org, "CareProgram"); !ok {
		t.Fatal("inventory object did not resolve")
	}
	before, ok := org.StandardSpreadSuppressed.lookup("CareProgram")
	if !ok {
		t.Fatal("by-value lookup lost suppression state")
	}
	for name, clone := range map[string]OrgState{
		"clone": org.Clone(), "runtime": org.CloneRuntime(), "frozen": org.CloneRuntimeFrozenDefinition(), "shared": org.CloneRuntimeFrozenShared(), "shallow": org.CloneObjectsShallow(0),
	} {
		t.Run(name, func(t *testing.T) {
			clone.StandardSpreadSuppressed.put("CareProgram", "changed")
			if got, _ := org.StandardSpreadSuppressed.lookup("CareProgram"); got != before {
				t.Fatal("clone changed original suppression")
			}
		})
	}
}

func TestOrgStateInvalidJSONRetainsDecodeError(t *testing.T) {
	org := NewOrgState()
	err := json.Unmarshal([]byte(`{"features":5}`), &org)
	const want = "json: cannot unmarshal number into Go struct field OrgState.features of type []string"
	if err == nil || err.Error() != want {
		t.Fatalf("decode error %v, want %q", err, want)
	}
}
