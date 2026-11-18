package storage

import (
	"reflect"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// owedNameShapes extends the lazy parity shapes with the captured plain
// Developer shape and the captured B2B Commerce shape.
func owedNameShapes() map[string][]string {
	shapes := make(map[string][]string, len(lazySpreadShapes)+2)
	for name, features := range lazySpreadShapes {
		shapes[name] = features
	}
	shapes["plainDeveloper"] = []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
	shapes["capturedB2B"] = []string{"B2BCommerce", "Communities", "NetworksEnabled", "CommerceEnabled"}
	return shapes
}

// installedPlusOwed is the name set a names-only global describe reads.
func installedPlusOwed(t *testing.T, org *OrgState) []string {
	t.Helper()
	set := make(map[string]struct{}, len(org.Objects))
	for name := range org.Objects {
		set[name] = struct{}{}
	}
	owed := SpreadOwedObjectNames(org)
	if !sort.StringsAreSorted(owed) {
		t.Fatal("SpreadOwedObjectNames is not in name order")
	}
	for _, name := range owed {
		if _, dup := set[name]; dup {
			t.Fatalf("SpreadOwedObjectNames returned an installed object: %s", name)
		}
		set[name] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func allowedObjectNames(org *OrgState, names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if org.AllowsStandardObject(name) {
			out = append(out, name)
		}
	}
	return out
}

// The installed names plus the owed names are the object set the org holds
// once the spread is complete, among the objects the shape allows, for every
// shape, both lazy modes, both source phases, with and without an org
// namespace, before and after a few objects are touched. On the source-backed
// routes the raw name sets are equal too. A names-only global describe
// depends on this.
func TestSpreadOwedObjectNamesMatchCompletedSpread(t *testing.T) {
	type build struct {
		name string
		make func(features []string) OrgState
	}
	builds := []build{
		{"source", sourceSpreadLazyOrg},
		{"sourcePlain", func(features []string) OrgState {
			org := NewOrgState()
			org.OrgID = DefaultOrgID
			ApplyProjectOrgShape(&org, features)
			org.EnableStandardSpreadSource()
			return org
		}},
		{"legacy", lazySpreadOrg},
	}
	for shapeName, features := range owedNameShapes() {
		for _, b := range builds {
			for _, namespace := range []string{"", "nsx"} {
				for _, touch := range []bool{false, true} {
					label := shapeName + "/" + b.name
					if namespace != "" {
						label += "/namespaced"
					}
					if touch {
						label += "/touched"
					}
					t.Run(label, func(t *testing.T) {
						org := b.make(features)
						org.Namespace = namespace
						if !org.LazyStandardSpreadEnabled() {
							t.Fatal("org is not lazy")
						}
						if b.name == "sourcePlain" && org.StandardSpread != standardSpreadPlain {
							t.Fatalf("phase %q, want plain", org.StandardSpread)
						}
						if touch {
							for _, objectName := range []string{"opportunity", "AsyncApexJob", "Case", "WebStore", "OrderSummary", "Quote"} {
								ResolveObjectName(org, objectName)
								EnsureLazyStandardObject(&org, objectName)
							}
						}
						before := len(org.Objects)
						got := installedPlusOwed(t, &org)
						if len(org.Objects) != before {
							t.Fatalf("SpreadOwedObjectNames installed objects: %d -> %d", before, len(org.Objects))
						}
						if !MaterializeStandardObjectSpread(&org) {
							t.Fatal("materialize reported no change")
						}
						want := sortedObjectNames(org)
						// Every route: equal once both sides keep only the
						// objects the shape allows, which is the filter the
						// global describe applies.
						gotAllowed, wantAllowed := allowedObjectNames(&org, got), allowedObjectNames(&org, want)
						if !reflect.DeepEqual(gotAllowed, wantAllowed) {
							t.Fatalf("allowed installed+owed has %d names, allowed completed spread %d; differing: %v", len(gotAllowed), len(wantAllowed), symmetricDifference(gotAllowed, wantAllowed))
						}
						// The source-backed routes also match name for name.
						// The legacy route completes with objects the shape
						// does not allow (plain Developer), which the filter
						// above removes.
						if b.name != "legacy" && !reflect.DeepEqual(got, want) {
							t.Fatalf("installed+owed has %d names, completed spread %d; differing: %v", len(got), len(want), symmetricDifference(got, want))
						}
						if owed := SpreadOwedObjectNames(&org); len(owed) != 0 {
							t.Fatalf("complete org still owes %d objects", len(owed))
						}
					})
				}
			}
		}
	}
}

func TestSpreadOwedObjectNamesNonLazy(t *testing.T) {
	org := NewOrgState()
	if names := SpreadOwedObjectNames(&org); names != nil {
		t.Fatalf("non-lazy org owes %v", names)
	}
	if names := SpreadOwedObjectNames(nil); names != nil {
		t.Fatalf("nil org owes %v", names)
	}
}
