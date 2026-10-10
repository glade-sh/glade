package vm

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// buildStandardOrgForDeterminism mirrors the apextest standard org: every known
// standard object is ensured once on a fresh org.
func buildStandardOrgForDeterminism() storage.OrgState {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000001"
	for _, objectName := range storage.KnownStandardObjectNames() {
		storage.EnsureStandardObject(&org, objectName)
	}
	return org
}

// Separately built standard orgs must be identical. The runtime schema stamp
// keys the disk and template caches across processes, so any ordered slice
// that takes its order from map iteration (for example Opportunity's
// ContractId and RecordTypeId relations) turns into a cache miss.
func TestStandardOrgBuildIsDeterministic(t *testing.T) {
	const builds = 20
	var wantStamp string
	var wantRelations map[string]string
	var wantDefinitions map[string]string
	for build := 0; build < builds; build++ {
		org := buildStandardOrgForDeterminism()
		stamp := schemaCacheStampForOrg(&org)
		relations := make(map[string]string, len(org.Objects))
		definitions := make(map[string]string, len(org.Objects))
		for name, object := range org.Objects {
			order := make([]string, 0, len(object.Definition.Relations))
			for _, relation := range object.Definition.Relations {
				order = append(order, relation.Field+"/"+relation.ChildRelationship)
			}
			relations[name] = strings.Join(order, ",")
			encoded, err := json.Marshal(object.Definition)
			if err != nil {
				t.Fatalf("build %d: marshal %s: %v", build, name, err)
			}
			definitions[name] = string(encoded)
		}
		if build == 0 {
			wantStamp, wantRelations, wantDefinitions = stamp, relations, definitions
			continue
		}
		if len(relations) != len(wantRelations) {
			t.Fatalf("build %d has %d objects, want %d", build, len(relations), len(wantRelations))
		}
		names := make([]string, 0, len(relations))
		for name := range relations {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			want, ok := wantRelations[name]
			if !ok {
				t.Fatalf("build %d has object %s missing from build 0", build, name)
			}
			if got := relations[name]; got != want {
				t.Errorf("build %d %s relations order:\n got %s\nwant %s", build, name, got, want)
			}
			if definitions[name] != wantDefinitions[name] {
				t.Errorf("build %d %s definition differs from build 0", build, name)
			}
		}
		if stamp != wantStamp {
			t.Errorf("build %d schema stamp = %s, want %s", build, stamp, wantStamp)
		}
		if t.Failed() {
			return
		}
	}
}
