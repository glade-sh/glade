package apextest

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// childRelationshipLookupShapes are the org phases the lookup runs on: the
// plain lazy seed, and the seed after a project shape applied.
var childRelationshipLookupShapes = map[string][]string{
	"plain":       nil,
	"unspecified": {project.UnspecifiedStandardFieldShape},
	"networks":    {"Communities", "NetworksEnabled", "Quotes", "MultiCurrency"},
}

func childRelationshipLookupOrg(features []string, namespace string) storage.OrgState {
	org := lazyStandardApexTestOrg()
	org.APIVersion = storage.DefaultRESTAPIVersion
	if features != nil {
		storage.ApplyProjectOrgShape(&org, features)
	}
	org.Namespace = namespace
	return org
}

// Building the child relationship lookup only reads the org. It installs no
// catalog object, so every clone of one lazy org gives the same lookup.
func TestChildRelationshipLookupIsDeterministicAndInstallsNothing(t *testing.T) {
	for name, features := range childRelationshipLookupShapes {
		t.Run(name, func(t *testing.T) {
			seed := childRelationshipLookupOrg(features, "")
			if !seed.StandardSpreadSourceBacked() {
				t.Fatal("seed org is not source backed")
			}
			var first map[string]struct{}
			for i := 0; i < 20; i++ {
				org := seed.CloneRuntimeFrozenDefinition()
				before := len(org.Objects)
				lookup := projectReferencedChildRelationshipLookup(org)
				if after := len(org.Objects); after != before {
					t.Fatalf("clone %d: the lookup installed objects: %d before, %d after", i, before, after)
				}
				if i == 0 {
					first = lookup
					continue
				}
				if !reflect.DeepEqual(first, lookup) {
					t.Fatalf("clone %d: lookup differs from clone 0 (%d vs %d keys)", i, len(lookup), len(first))
				}
			}
		})
	}
}

// The parent name the lookup keys on is the one ResolveObjectName gives, up
// to case, for every known standard object in every casing, whether or not
// the name would install an object.
func TestChildRelationshipParentNameKeyMatchesResolveObjectName(t *testing.T) {
	names := storage.KnownStandardObjectNames()
	if len(names) < 1000 {
		t.Fatalf("only %d known standard object names", len(names))
	}
	for shape, features := range childRelationshipLookupShapes {
		for _, namespace := range []string{"", "pkgns"} {
			t.Run(shape+"/"+namespace, func(t *testing.T) {
				seed := childRelationshipLookupOrg(features, namespace)
				fast := seed.CloneRuntimeFrozenDefinition()
				before := len(fast.Objects)
				resolved := seed.CloneRuntimeFrozenDefinition()
				for _, name := range names {
					for _, parent := range []string{name, strings.ToLower(name), strings.ToUpper(name)} {
						got := strings.ToLower(resolvedChildRelationshipParentName(fast, map[string]string{}, parent))
						want, ok := storage.ResolveObjectName(resolved, parent)
						if !ok {
							want = parent
						}
						if got != strings.ToLower(want) {
							t.Fatalf("%q: lookup key %q, ResolveObjectName key %q", parent, got, strings.ToLower(want))
						}
					}
				}
				if after := len(fast.Objects); after != before {
					t.Fatalf("resolving parent names installed objects: %d before, %d after", before, after)
				}
			})
		}
	}
}
