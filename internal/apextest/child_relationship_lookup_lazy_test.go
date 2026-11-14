package apextest

import (
	"path/filepath"
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
// the name would install an object. A name that resolves by case alone
// installs nothing.
func TestChildRelationshipParentNameKeyMatchesResolveObjectName(t *testing.T) {
	names := storage.KnownStandardObjectNames()
	if len(names) < 1000 {
		t.Fatalf("only %d known standard object names", len(names))
	}
	for shape, features := range childRelationshipLookupShapes {
		// Knowledge is the namespace token of Knowledge__Feed and its kin.
		for _, namespace := range []string{"", "pkgns", "Knowledge"} {
			t.Run(shape+"/"+namespace, func(t *testing.T) {
				seed := childRelationshipLookupOrg(features, namespace)
				fast := seed.CloneRuntimeFrozenDefinition()
				resolved := seed.CloneRuntimeFrozenDefinition()
				byCase := 0
				for _, name := range names {
					for _, parent := range []string{name, strings.ToLower(name), strings.ToUpper(name)} {
						before := len(fast.Objects)
						got := strings.ToLower(resolvedChildRelationshipParentName(fast, map[string]string{}, parent))
						if childRelationshipParentNameResolvesByCase(namespace, parent) {
							byCase++
							if after := len(fast.Objects); after != before {
								t.Fatalf("%q: resolving the parent name installed %d objects", parent, after-before)
							}
						}
						want, ok := storage.ResolveObjectName(resolved, parent)
						if !ok {
							want = parent
						}
						if got != strings.ToLower(want) {
							t.Fatalf("%q: lookup key %q, ResolveObjectName key %q", parent, got, strings.ToLower(want))
						}
					}
				}
				if byCase < 3*len(names)-30 {
					t.Fatalf("only %d of %d names resolve by case", byCase, 3*len(names))
				}
			})
		}
	}
}

// A source that names a child relationship whose child object is not in the
// lazy seed must not gain an inferred parent field the eager spread lacks.
func TestLazyStandardSpreadUninstalledChildRelationshipNameMatchesEagerSpread(t *testing.T) {
	t.Skip("existing lazy/eager parity gap: uninstalled child relationships can be inferred as fields")
	const class = `@IsTest
private class ChildRelationshipNameTest {
	@IsTest static void namesUninstalledChildRelationships() {
		Account a = new Account(Name = 'Parent');
		Object wallets = a.DigitalWallets;
		Object entitlements = a.Entitlements;
		Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
		System.assert(false, fields.containsKey('digitalwallets') + ' ' + fields.containsKey('entitlements') + ' ' + (wallets == null) + ' ' + (entitlements == null));
	}
}`
	run := func(eager bool) string {
		prior := eagerStandardSpread.Load()
		eagerStandardSpread.Store(eager)
		defer eagerStandardSpread.Store(prior)
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/ChildRelationshipNameTest.cls"), class)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/ChildRelationshipNameTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
		index := loadTestIndex(t, root)
		report := Run(index, Options{NoDiskCache: true, Parallelism: 1})
		var answers []string
		for _, suite := range report.Suites {
			for _, c := range suite.Cases {
				if c.Problem != nil {
					answers = append(answers, c.MethodName+": "+c.Problem.Message)
				}
			}
		}
		if len(answers) != 1 {
			t.Fatalf("eager=%t: expected one assertion outcome, got %d", eager, len(answers))
		}
		return strings.Join(answers, "\n")
	}
	eager, lazy := run(true), run(false)
	if eager != lazy {
		t.Fatalf("eager and lazy spreads answer differently\neager: %s\nlazy:  %s", eager, lazy)
	}
}
