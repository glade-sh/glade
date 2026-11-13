package vm

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// completeShapedOrg is the org a shaped lazy test run holds once the spread is
// complete, for a feature list.
func completeShapedOrg(features []string) storage.OrgState {
	org := shapedLazyOrg(features)
	storage.MaterializeStandardObjectSpread(&org)
	return org
}

// eagerShapedOrg starts from the whole catalog and applies the shape after.
func eagerShapedOrg(features []string) storage.OrgState {
	org := storage.NewOrgState()
	org.OrgID = storage.DefaultOrgID
	storage.EnsureStandardObjectSpread(&org)
	org.APIVersion = storage.DefaultRESTAPIVersion
	storage.ApplyProjectOrgShape(&org, features)
	return org
}

// childRelFilterFacts are the aliases every object of a complete org answers
// for, and the parent candidate keys its definitions name, as hashes.
type childRelFilterFacts struct {
	pairs map[uint64][2]string
	keys  map[uint64]string
}

func newChildRelFilterFacts() *childRelFilterFacts {
	return &childRelFilterFacts{pairs: map[uint64][2]string{}, keys: map[uint64]string{}}
}

func (f *childRelFilterFacts) collect(org *storage.OrgState) *VM {
	machine := New(nil)
	machine.SetOrg(org)
	for name := range org.Objects {
		parentKey := strings.ToLower(strings.TrimSpace(name))
		for alias := range machine.buildJSONSObjectChildRelationshipTypeIndex(name) {
			f.pairs[childRelFilterHash(parentKey, alias)] = [2]string{parentKey, alias}
		}
	}
	for key := range buildChildRelationshipCandidates(org) {
		if key != loadedChildRelationshipFullScanKey {
			f.keys[childRelFilterHash(key, "")] = key
		}
	}
	return machine
}

func sortedHashes[V any](m map[uint64]V) []uint64 {
	out := make([]uint64, 0, len(m))
	for h := range m {
		out = append(out, h&childRelFilterHashMask)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// childRelFilterSignatures are the signatures the generated data is built
// from: no features, each feature alone, and every covered feature at once.
// Features only add to a shape (the plain developer marker is excluded), so
// the full set contains the rest; the singles guard that assumption.
func childRelFilterSignatures() [][]string {
	var all []string
	all = append(all, childRelFilterFeatureNames...)
	all = append(all, childRelFilterShapeTokens...)
	sets := [][]string{nil}
	for _, name := range all {
		sets = append(sets, []string{name})
	}
	return append(sets, all)
}

// Regenerates child_relationship_filter.txt. Without the variable it only
// checks that the checked-in file decodes.
func TestChildRelationshipFilterGenerate(t *testing.T) {
	if os.Getenv("GLADE_UPDATE_CHILD_REL_FILTER") == "" {
		if loadChildRelFilter() == nil {
			t.Fatal("child_relationship_filter.txt does not decode against this catalog; regenerate it with GLADE_UPDATE_CHILD_REL_FILTER=1")
		}
		return
	}
	facts := newChildRelFilterFacts()
	for _, set := range childRelFilterSignatures() {
		org := completeShapedOrg(set)
		facts.collect(&org)
		runtime.GC()
	}
	if os.Getenv("GLADE_UPDATE_CHILD_REL_FILTER") == "sample" {
		// Slower check of the superset assumption before writing: random
		// pairs and larger random shapes add nothing the union lacks.
		all := append(slices.Clone(childRelFilterFeatureNames), childRelFilterShapeTokens...)
		random := rand.New(rand.NewSource(101))
		var shapes [][]string
		for i := 0; i < 60; i++ {
			a, b := random.Intn(len(all)), random.Intn(len(all))
			shapes = append(shapes, []string{all[a], all[b]})
		}
		for i := 0; i < 30; i++ {
			var set []string
			for _, name := range all {
				if random.Intn(4) == 0 {
					set = append(set, name)
				}
			}
			shapes = append(shapes, set)
		}
		for _, set := range shapes {
			extra := newChildRelFilterFacts()
			org := completeShapedOrg(set)
			extra.collect(&org)
			for h, pair := range extra.pairs {
				if _, ok := facts.pairs[h]; !ok {
					t.Errorf("%s adds %v", childRelFilterName(set), pair)
				}
			}
			runtime.GC()
		}
	}
	if os.Getenv("GLADE_UPDATE_CHILD_REL_FILTER") == "pairs" {
		// Exhaustive check of the superset assumption: every unordered pair
		// of covered features. Reports what the union lacks and writes nothing.
		all := append(slices.Clone(childRelFilterFeatureNames), childRelFilterShapeTokens...)
		checked, added := 0, 0
		for i := 0; i < len(all); i++ {
			for j := i + 1; j < len(all); j++ {
				set := []string{all[i], all[j]}
				extra := newChildRelFilterFacts()
				org := completeShapedOrg(set)
				extra.collect(&org)
				for h, pair := range extra.pairs {
					if _, ok := facts.pairs[h]; !ok {
						added++
						t.Errorf("%s adds %v", childRelFilterName(set), pair)
					}
				}
				for h, key := range extra.keys {
					if _, ok := facts.keys[h]; !ok {
						added++
						t.Errorf("%s adds parent key %v", childRelFilterName(set), key)
					}
				}
				checked++
				runtime.GC()
			}
		}
		t.Logf("pairs mode: %d feature pairs checked, %d additions", checked, added)
		return
	}
	blob := encodeChildRelFilter(storage.StandardCatalogStamp(), sortedHashes(facts.pairs), sortedHashes(facts.keys))
	if err := os.WriteFile("child_relationship_filter.txt", blob, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d bytes: %d pairs, %d parent keys", len(blob), len(facts.pairs), len(facts.keys))
}

// The covered feature list must name every feature storage canonicalizes.
func TestChildRelationshipFilterFeatureListIsComplete(t *testing.T) {
	fset := gotoken.NewFileSet()
	parsed, err := goparser.ParseFile(fset, "../storage/standard_fields.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var returned []string
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "canonicalFeatureName" {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			if ret, ok := n.(*ast.ReturnStmt); ok {
				for _, result := range ret.Results {
					if lit, ok := result.(*ast.BasicLit); ok && lit.Kind == gotoken.STRING {
						value, _ := strconv.Unquote(lit.Value)
						returned = append(returned, value)
					}
				}
			}
			return true
		})
	}
	if len(returned) == 0 {
		t.Fatal("canonicalFeatureName not found")
	}
	for _, name := range returned {
		if !slices.Contains(childRelFilterFeatureNames, name) {
			t.Errorf("feature %q is not in childRelFilterFeatureNames; add it and regenerate the filter", name)
		}
	}
	for _, name := range childRelFilterFeatureNames {
		if !slices.Contains(returned, name) {
			t.Errorf("childRelFilterFeatureNames lists %q, which storage no longer canonicalizes", name)
		}
	}
	for _, shapeToken := range childRelFilterShapeTokens {
		if _, ok := childRelFilterCovered[shapeToken]; !ok {
			t.Errorf("shape token %q is not covered", shapeToken)
		}
	}
	if _, ok := childRelFilterCovered[project.PlainDeveloperObjectShape]; ok {
		t.Error("the plain developer marker removes objects and must not be covered")
	}
}

func childRelFilterTestConfigs(t *testing.T) [][]string {
	all := append(slices.Clone(childRelFilterFeatureNames), childRelFilterShapeTokens...)
	configs := [][]string{nil, all, {"MultiCurrency"}, {"PersonAccounts", "Quotes"}, {project.UnspecifiedStandardFieldShape, "MultiCurrency"}}
	if testing.Short() {
		return configs
	}
	random := rand.New(rand.NewSource(7))
	for i := 0; i < 10; i++ {
		var set []string
		for _, name := range all {
			if random.Intn(3) == 0 {
				set = append(set, name)
			}
		}
		configs = append(configs, set)
	}
	return configs
}

func childRelFilterName(set []string) string {
	return "[" + strings.Join(set, ",") + "]"
}

// The staleness guard: every pair an org holds is in the filter. The eager org
// and the completed lazy org both stand in for the runtime's complete org.
func TestChildRelationshipFilterCoversEagerOrgs(t *testing.T) {
	filter := loadChildRelFilter()
	if filter == nil {
		t.Fatal("filter data does not decode against this catalog; regenerate it")
	}
	for i, set := range childRelFilterTestConfigs(t) {
		orgs := map[string]func() storage.OrgState{
			"lazy-complete": func() storage.OrgState { return completeShapedOrg(set) },
		}
		if i < 5 {
			orgs["eager"] = func() storage.OrgState { return eagerShapedOrg(set) }
		}
		for kind, build := range orgs {
			org := build()
			facts := newChildRelFilterFacts()
			facts.collect(&org)
			missing := 0
			for h, pair := range facts.pairs {
				if !childRelFilterContains(filter.pairs, h) {
					if missing++; missing <= 5 {
						t.Errorf("%s %s: pair %v is not in the filter; regenerate it", kind, childRelFilterName(set), pair)
					}
				}
			}
			for h, key := range facts.keys {
				if !childRelFilterContains(filter.keys, h) {
					t.Errorf("%s %s: parent key %q is not in the filter; regenerate it", kind, childRelFilterName(set), key)
				}
			}
			if missing > 0 {
				t.Fatalf("%s %s: %d pairs missing", kind, childRelFilterName(set), missing)
			}
		}
		runtime.GC()
	}
}

// Whenever the filter declares a miss final, the complete org answers no
// relationship for it. Every real alias and a set of misses are checked per
// parent, with the lookup the runtime makes.
func TestChildRelationshipFilterAgreesWithCompleteOrg(t *testing.T) {
	for _, set := range [][]string{nil, {"MultiCurrency"}, {"PersonAccounts", "Quotes", "NetworksEnabled"}} {
		lazy := shapedLazyOrg(set)
		complete := completeShapedOrg(set)
		completeVM := New(nil)
		completeVM.SetOrg(&complete)
		lazyVM := New(nil)
		lazyVM.SetOrg(&lazy)
		parents := make([]string, 0, len(complete.Objects))
		for name := range complete.Objects {
			parents = append(parents, name)
		}
		sort.Strings(parents)
		random := rand.New(rand.NewSource(11))
		real, finalMisses := 0, 0
		for _, parent := range parents {
			index := completeVM.buildJSONSObjectChildRelationshipTypeIndex(parent)
			names := []string{"Nonsense__r", "NoSuchField", "name", "Id", parent + "s", "x" + strconv.Itoa(random.Intn(1000)) + "__c"}
			for alias := range index {
				names = append(names, alias, strings.ToUpper(alias), alias+"x")
			}
			for _, name := range names {
				key := strings.ToLower(strings.TrimSpace(name))
				_, want := index[key]
				final := lazyVM.childRelFilterMissIsFinal(parent, key)
				if want && final {
					t.Fatalf("%s: filter rules out %s.%s, which the complete org answers", childRelFilterName(set), parent, name)
				}
				if want {
					real++
				}
				if final {
					finalMisses++
				}
			}
		}
		if real == 0 || finalMisses == 0 {
			t.Fatalf("checked %d real aliases and %d final misses", real, finalMisses)
		}
	}
}

// End to end on fresh lazy orgs: the lazy answer equals the complete org's, and
// a final miss keeps the spread lazy.
func TestChildRelationshipFilterLookupMatchesCompleteOrg(t *testing.T) {
	for _, set := range [][]string{nil, {"MultiCurrency"}, {"PersonAccounts", "Contacts__x"}} {
		complete := completeShapedOrg(set)
		completeVM := New(nil)
		completeVM.SetOrg(&complete)
		var parents []string
		for name := range complete.Objects {
			parents = append(parents, name)
		}
		sort.Strings(parents)
		random := rand.New(rand.NewSource(3))
		random.Shuffle(len(parents), func(i, j int) { parents[i], parents[j] = parents[j], parents[i] })
		checked := 0
		for _, parent := range parents {
			if checked >= 20 {
				break
			}
			index := completeVM.buildJSONSObjectChildRelationshipTypeIndex(parent)
			names := []string{"Nonsense__r", "Status__c", "NAME"}
			for alias := range index {
				names = append(names, alias)
				break
			}
			for _, name := range names {
				lazy := shapedLazyOrg(set)
				lazyVM := New(nil)
				lazyVM.SetOrg(&lazy)
				wantType, wantOK := completeVM.jsonSObjectChildRelationshipType(parent, name)
				gotType, gotOK := lazyVM.jsonSObjectChildRelationshipType(parent, name)
				if gotType != wantType || gotOK != wantOK {
					t.Fatalf("%s %s.%s: lazy %q, %v; complete %q, %v", childRelFilterName(set), parent, name, gotType, gotOK, wantType, wantOK)
				}
				checked++
			}
		}
	}
}

// A name the filter cannot rule out still completes the spread, even when the
// parent's index was first built by a miss the filter answered.
func TestChildRelationshipFilterDeclinedIndexStillMaterializes(t *testing.T) {
	complete := completeShapedOrg(nil)
	completeVM := New(nil)
	completeVM.SetOrg(&complete)
	var alias, want string
	for key, lookup := range completeVM.buildJSONSObjectChildRelationshipTypeIndex("Account") {
		if lookup.OK && (alias == "" || key < alias) {
			alias, want = key, lookup.Type
		}
	}
	lazy := shapedLazyOrg(nil)
	if _, held := lazy.Objects[want[len("List<"):len(want)-1]]; held {
		t.Skipf("%s is a core object; the spread has nothing to complete", want)
	}
	machine := New(nil)
	machine.SetOrg(&lazy)
	if _, ok := machine.jsonSObjectChildRelationshipType("Account", "Nonsense__r"); ok || !lazy.LazyStandardSpreadEnabled() {
		t.Fatal("setup: a final miss should keep the spread lazy")
	}
	got, ok := machine.jsonSObjectChildRelationshipType("Account", alias)
	if !ok || got != want {
		t.Fatalf("Account.%s after a declined miss = %q, %v; want %q", alias, got, ok, want)
	}
}

// Each condition that can change an owed object's relations disables the
// filter: the miss completes the spread as before.
func TestChildRelationshipFilterFallsBackWhenUnsound(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(org *storage.OrgState)
		keep    bool
	}{
		{name: "covered shaped org", prepare: func(*storage.OrgState) {}, keep: true},
		{name: "plain phase", prepare: func(org *storage.OrgState) { org.StandardSpread = "plain" }},
		{name: "namespace", prepare: func(org *storage.OrgState) { org.Namespace = "pkg" }},
		{name: "unknown feature", prepare: func(org *storage.OrgState) { org.Features = append(slices.Clone(org.Features), "SomethingNew") }},
		{name: "non-canonical feature", prepare: func(org *storage.OrgState) { org.Features = []string{"multicurrency"} }},
		{name: "plain developer shape", prepare: func(org *storage.OrgState) {
			org.Features = append(slices.Clone(org.Features), project.PlainDeveloperObjectShape)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			org := shapedLazyOrg([]string{"MultiCurrency"})
			tc.prepare(&org)
			machine := New(nil)
			machine.SetOrg(&org)
			if _, ok := machine.jsonSObjectChildRelationshipType("Account", "Nonsense__r"); ok {
				t.Fatal("nonsense name resolved")
			}
			if got := org.LazyStandardSpreadEnabled(); got != tc.keep {
				t.Fatalf("spread still lazy = %v, want %v", got, tc.keep)
			}
		})
	}
	t.Run("stale catalog stamp", func(t *testing.T) {
		stale := encodeChildRelFilter("not-this-catalog", nil, nil)
		if decodeChildRelFilter(stale, storage.StandardCatalogStamp()) != nil {
			t.Fatal("data from another catalog decoded")
		}
		if decodeChildRelFilter(childRelFilterBlob[:len(childRelFilterBlob)-1], storage.StandardCatalogStamp()) != nil {
			t.Fatal("truncated data decoded")
		}
		if decodeChildRelFilter(childRelFilterBlob, "") != nil {
			t.Fatal("empty stamp decoded")
		}
	})
	t.Run("junk data", func(t *testing.T) {
		if decodeChildRelFilter([]byte("junk"), storage.StandardCatalogStamp()) != nil {
			t.Fatal("junk decoded")
		}
	})
	t.Run("no filter falls back", func(t *testing.T) {
		org := shapedLazyOrg([]string{"MultiCurrency"})
		machine := New(nil)
		machine.SetOrg(&org)
		childRelFilterOverride = &childRelFilter{}
		defer func() { childRelFilterOverride = nil }()
		// An empty filter holds no pairs, so it would rule everything out;
		// the check below proves the override is what answered.
		if !machine.childRelFilterMissIsFinal("Account", "nonsense__r") {
			t.Fatal("override not used")
		}
	})
}

// A custom parent no catalog definition names stays lazy; a custom parent that
// shares a stripped name with a catalog parent falls back. Both match the
// complete org.
func TestChildRelationshipFilterCustomParents(t *testing.T) {
	customOrg := func(org *storage.OrgState) {
		for _, name := range []string{"Widget__c", "Account__c"} {
			org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: name,
				Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
			}, Records: map[storage.ID]storage.Record{}}
		}
		org.Objects["WidgetLine__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: "WidgetLine__c",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Widget__c": {APIName: "Widget__c", Type: storage.FieldReference, ReferenceTo: []string{"Widget__c"}, RelationshipName: "Widget__r", ChildRelationshipName: "Lines__r"},
				"Owner__c":  {APIName: "Owner__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Owner__r", ChildRelationshipName: "WidgetLines__r"},
			},
		}, Records: map[storage.ID]storage.Record{}}
	}
	lazy := shapedLazyOrg([]string{"MultiCurrency"})
	customOrg(&lazy)
	complete := completeShapedOrg([]string{"MultiCurrency"})
	customOrg(&complete)
	lazyVM, completeVM := New(nil), New(nil)
	lazyVM.SetOrg(&lazy)
	completeVM.SetOrg(&complete)
	for _, tc := range []struct {
		parent, name string
		wantLazy     bool
	}{
		{"Widget__c", "Status__c", true},
		{"Widget__c", "Lines__r", true},
		{"Account", "WidgetLines__r", true},
		{"Account", "Nonsense__r", true},
	} {
		want, wantOK := completeVM.jsonSObjectChildRelationshipType(tc.parent, tc.name)
		got, gotOK := lazyVM.jsonSObjectChildRelationshipType(tc.parent, tc.name)
		if got != want || gotOK != wantOK {
			t.Fatalf("%s.%s: lazy %q, %v; complete %q, %v", tc.parent, tc.name, got, gotOK, want, wantOK)
		}
		if tc.wantLazy && !lazy.LazyStandardSpreadEnabled() {
			t.Fatalf("%s.%s completed the spread", tc.parent, tc.name)
		}
	}
	if got, ok := lazyVM.jsonSObjectChildRelationshipType("Widget__c", "Lines__r"); !ok || got != "List<WidgetLine__c>" {
		t.Fatalf("installed custom child relationship = %q, %v", got, ok)
	}
}

// The filter is read-only shared state: many VMs on their own lazy orgs ask at
// once. Run with -race.
func TestChildRelationshipFilterConcurrentVMs(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan string, 16)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			org := shapedLazyOrg([]string{"MultiCurrency"})
			machine := New(nil)
			machine.SetOrg(&org)
			for i := 0; i < 20; i++ {
				name := fmt.Sprintf("Nope%d_%d__r", worker, i)
				if _, ok := machine.jsonSObjectChildRelationshipType("Account", name); ok {
					errs <- name
					return
				}
				if !machine.childRelFilterMissIsFinal("Contact", strings.ToLower(name)) {
					errs <- "contact " + name
					return
				}
			}
			if !org.LazyStandardSpreadEnabled() {
				errs <- "spread completed"
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
}

// Every line is twelve digit letters, so no hex or id pattern can match it,
// and a table round-trips through the in-place search for the extreme values.
func TestChildRelationshipFilterEncoding(t *testing.T) {
	values := []uint64{0, 1, 15, 16, childRelFilterHashMask, childRelFilterHashMask - 1, 0x1a2b3c4d5e6f}
	random := rand.New(rand.NewSource(5))
	for i := 0; i < 2000; i++ {
		values = append(values, random.Uint64()&childRelFilterHashMask)
	}
	slices.Sort(values)
	values = slices.Compact(values)
	blob := encodeChildRelFilter("stamp", values, values[:10])
	filter := decodeChildRelFilter(blob, "stamp")
	if filter == nil {
		t.Fatal("encoded table does not decode")
	}
	line := regexp.MustCompile(`^[` + childRelFilterDigits + `]{12}$`)
	lines := strings.Split(strings.TrimSuffix(string(filter.pairs), "\n"), "\n")
	for _, text := range lines {
		if !line.MatchString(text) {
			t.Fatalf("line %q is not twelve digit letters", text)
		}
	}
	if !slices.IsSorted(lines) {
		t.Error("lines do not sort as their values do")
	}
	for _, v := range values {
		if !childRelFilterContains(filter.pairs, v) {
			t.Fatalf("value %x is not found", v)
		}
	}
	for _, v := range []uint64{2, 17, 0xffff_ffff_fffe} {
		if _, found := slices.BinarySearch(values, v); !found && childRelFilterContains(filter.pairs, v) {
			t.Errorf("absent value %x is found", v)
		}
	}
	for _, v := range values[:10] {
		if !childRelFilterContains(filter.keys, v) {
			t.Fatalf("key %x is not found", v)
		}
	}
}
