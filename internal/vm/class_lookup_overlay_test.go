package vm

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// classIdentity names a Class by the fields class lookup ranks on. A full
// build breaks a tie of rank and lower-cased namespace by map order, and tied
// candidates differ at most in letter case, so identities fold case.
func classIdentity(class Class, ok bool) string {
	if !ok {
		return "<miss>"
	}
	return strings.ToLower(fmt.Sprintf("%s|%s|dep=%t", class.Namespace, class.Name, class.Dependency))
}

// fullRebuildTwin returns a clone of base that takes the pre-overlay path:
// registration rebuilds the whole lookup and FreezeClassLookup builds a root.
func fullRebuildTwin(base *VM) *VM {
	twin := base.CloneRuntimeFrozenShared(nil)
	twin.classValuesWritten = true
	return twin
}

func registerAll(t *testing.T, machine *VM, classes []Class) {
	t.Helper()
	for _, class := range classes {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
}

// classLookupProbes returns every name a full build indexes, in several
// spellings, plus names no build indexes.
func classLookupProbes(machines ...*VM) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, machine := range machines {
		for alias, class := range machine.Classes {
			for _, name := range []string{alias, class.Name, class.Namespace + "." + class.Name} {
				add(name)
				add(strings.ToUpper(name))
				add(strings.ToLower(name))
				add(" " + name + " ")
			}
		}
	}
	for _, name := range []string{"Missing", "pkg.Missing", "Outer.Missing", "abc.abc.Alpha"} {
		add(name)
	}
	sort.Strings(out)
	return out
}

// tiedClassLookupKeys returns the canonical keys whose full-build result
// depends on map order in any of the given class maps: several best-ranked
// candidates with the lowest namespace but different class identities.
func tiedClassLookupKeys(maps ...map[string]Class) map[string]bool {
	type best struct {
		rank int
		ns   string
		ids  map[string]bool
	}
	tied := map[string]bool{}
	for _, classes := range maps {
		bests := map[string]*best{}
		consider := func(name string, class Class, exact bool) {
			key := canonicalClassLookupKey(name)
			rank := classLookupKeyRank(class, exact)
			ns := strings.ToLower(strings.TrimSpace(class.Namespace))
			id := classIdentity(class, true)
			current := bests[key]
			switch {
			case current == nil || rank > current.rank || (rank == current.rank && ns < current.ns):
				bests[key] = &best{rank: rank, ns: ns, ids: map[string]bool{id: true}}
			case rank == current.rank && ns == current.ns:
				current.ids[id] = true
			}
		}
		for alias, class := range classes {
			consider(alias, class, true)
			consider(class.Name, class, false)
			if class.Namespace != "" {
				consider(class.Namespace+"."+class.Name, class, true)
			}
		}
		for key, entry := range bests {
			if len(entry.ids) > 1 {
				tied[key] = true
			}
		}
	}
	return tied
}

func assertSameClassLookups(t *testing.T, label string, got, want *VM, tied map[string]bool) {
	t.Helper()
	for _, name := range classLookupProbes(got, want) {
		if tied[canonicalClassLookupKey(name)] {
			continue
		}
		gotClass, gotOK := got.lookupClass(name)
		wantClass, wantOK := want.lookupClass(name)
		if classIdentity(gotClass, gotOK) != classIdentity(wantClass, wantOK) {
			t.Fatalf("%s: lookupClass(%q) = %s, full rebuild %s", label, name, classIdentity(gotClass, gotOK), classIdentity(wantClass, wantOK))
		}
		// Namespace tables resolve every class name, so any tie can move them.
		for _, namespace := range []string{"", "pkg", "abc", "Pkg"} {
			if len(tied) > 0 {
				break
			}
			gotClass, gotOK := got.lookupClassInNamespace(namespace, name)
			wantClass, wantOK := want.lookupClassInNamespace(namespace, name)
			if classIdentity(gotClass, gotOK) != classIdentity(wantClass, wantOK) {
				t.Fatalf("%s: lookupClassInNamespace(%q, %q) = %s, full rebuild %s", label, namespace, name, classIdentity(gotClass, gotOK), classIdentity(wantClass, wantOK))
			}
		}
		gotTop, gotTopOK := got.resolveTopLevelClassName(name)
		wantTop, wantTopOK := want.resolveTopLevelClassName(name)
		if !strings.EqualFold(gotTop, wantTop) || gotTopOK != wantTopOK {
			t.Fatalf("%s: top-level %q = %q/%t, full rebuild %q/%t", label, name, gotTop, gotTopOK, wantTop, wantTopOK)
		}
	}
}

// comparableTopLevel drops what a full build leaves to map order: Unique of
// an ambiguous name, which resolveTopLevelClassName never reads, and the
// spelling among case variants of one candidate.
func comparableTopLevel(index map[string]topLevelClassLookup) map[string]topLevelClassLookup {
	out := make(map[string]topLevelClassLookup, len(index))
	for key, entry := range index {
		normalized := topLevelClassLookup{Unique: strings.ToLower(entry.Unique), Ambiguous: entry.Ambiguous, ByNamespace: map[string]string{}}
		if entry.Ambiguous {
			normalized.Unique = ""
		}
		for namespace, candidate := range entry.ByNamespace {
			normalized.ByNamespace[namespace] = strings.ToLower(candidate)
		}
		out[key] = normalized
	}
	return out
}

// copyPlanGroups flattens a plan into its classCopyKey groups.
func copyPlanGroups(plan *classCopyPlan) []string {
	primaryOf := map[string]string{}
	if base := plan.base; base != nil {
		for _, name := range base.primaries {
			if !plan.planSkips(name) {
				primaryOf[name] = name
			}
		}
		for alias, primary := range base.aliases {
			if !plan.planSkips(alias) {
				primaryOf[alias] = primary
			}
		}
	}
	for _, name := range plan.primaries {
		primaryOf[name] = name
	}
	for alias, primary := range plan.aliases {
		primaryOf[alias] = primary
	}
	members := map[string][]string{}
	for name, primary := range primaryOf {
		members[primary] = append(members[primary], name)
	}
	var groups []string
	for _, names := range members {
		sort.Strings(names)
		groups = append(groups, strings.Join(names, ","))
	}
	sort.Strings(groups)
	return groups
}

// assertSameFrozenGeneration compares a layered freeze with a full freeze
// over the same classes.
func assertSameFrozenGeneration(t *testing.T, label string, got, want *VM, tied map[string]bool) {
	t.Helper()
	if want.frozenClassLookup == nil || want.frozenClassLookup.base != nil {
		t.Fatalf("%s: twin is not a full root generation", label)
	}
	gotFrozen := got.frozenClassLookup
	if gotFrozen == nil {
		t.Fatalf("%s: overlay VM is not frozen", label)
	}
	keys := map[string]bool{}
	for key := range want.frozenClassLookup.keys {
		keys[key] = true
	}
	for key := range gotFrozen.rootGeneration().keys {
		keys[key] = true
	}
	for key := range gotFrozen.keys {
		keys[key] = true
	}
	for key := range keys {
		if tied[key] {
			continue
		}
		gotAlias, gotOK := gotFrozen.resolve(key)
		wantAlias, wantOK := want.frozenClassLookup.resolve(key)
		gotClass, gotFound := got.Classes[gotAlias]
		wantClass, wantFound := want.Classes[wantAlias]
		if gotOK != wantOK || classIdentity(gotClass, gotFound) != classIdentity(wantClass, wantFound) {
			t.Fatalf("%s: key %q -> %q (%t, %s), full build %q (%t, %s)", label, key, gotAlias, gotOK, classIdentity(gotClass, gotFound), wantAlias, wantOK, classIdentity(wantClass, wantFound))
		}
	}
	if !reflect.DeepEqual(got.classNameSearchCache, want.classNameSearchCache) {
		t.Fatalf("%s: search entries differ:\n got %v\nwant %v", label, got.classNameSearchCache, want.classNameSearchCache)
	}
	if !reflect.DeepEqual(comparableTopLevel(got.topLevelClassLookup), comparableTopLevel(want.topLevelClassLookup)) {
		t.Fatalf("%s: top-level index differs:\n got %v\nwant %v", label, got.topLevelClassLookup, want.topLevelClassLookup)
	}
	if gotGroups, wantGroups := copyPlanGroups(got.sharedClassCopyPlan), copyPlanGroups(want.sharedClassCopyPlan); !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Fatalf("%s: copy plan groups differ:\n got %v\nwant %v", label, gotGroups, wantGroups)
	}
	if got.sharedClassCopyPlan.uniformAliases != want.sharedClassCopyPlan.uniformAliases || got.canShareClassMap() != want.canShareClassMap() {
		t.Fatalf("%s: uniform aliases %t, full build %t", label, got.sharedClassCopyPlan.uniformAliases, want.sharedClassCopyPlan.uniformAliases)
	}
}

func randomOverlayClass(rng *rand.Rand) Class {
	names := []string{"Alpha", "alpha", "Beta", "Outer", "Outer.Inner", "outer.inner", "Pkg", "Pkg.Alpha", "pkg.Alpha", "abc.Beta", "Gamma", "Outer.Deep.Leaf"}
	namespaces := []string{"", "", "pkg", "abc", "Pkg"}
	class := Class{Name: names[rng.Intn(len(names))], Namespace: namespaces[rng.Intn(len(namespaces))]}
	class.Dependency = class.Namespace != "" && rng.Intn(3) > 0
	return class
}

// TestClassLookupOverlayMatchesFullRebuild registers random class sets on a
// frozen clone through the overlay and through the full rebuild, then compares
// every lookup before and after freezing, at two layer depths.
func TestClassLookupOverlayMatchesFullRebuild(t *testing.T) {
	const seeds = 600
	untied, compared := 0, 0
	for seed := int64(1); seed <= seeds; seed++ {
		rng := rand.New(rand.NewSource(seed))
		base := New(nil)
		for i := rng.Intn(12); i >= 0; i-- {
			if err := base.RegisterClass(randomOverlayClass(rng)); err != nil {
				t.Fatal(err)
			}
		}
		base.FreezeClassLookup()
		if copyGroupsMixIdentities(base) {
			// Each clone of base takes a map-order primary's value.
			continue
		}
		root := base.frozenClassLookup
		rootKeys := snapshotStringMap(root.keys)
		history := []map[string]Class{base.Classes}
		anyTie := false
		tiedNow := func(machines ...*VM) map[string]bool {
			maps := append([]map[string]Class(nil), history...)
			for _, machine := range machines {
				maps = append(maps, machine.Classes)
			}
			tied := tiedClassLookupKeys(maps...)
			anyTie = anyTie || len(tied) > 0
			return tied
		}

		layered := base.CloneRuntimeFrozenShared(nil)
		twin := fullRebuildTwin(base)
		for i := 1 + rng.Intn(4); i > 0; i-- {
			class := randomOverlayClass(rng)
			registerAll(t, layered, []Class{class})
			registerAll(t, twin, []Class{class})
			assertSameClassLookups(t, fmt.Sprintf("seed %d pending", seed), layered, twin, tiedNow(layered, twin))
		}
		if layered.classOverlay == nil {
			t.Fatalf("seed %d: registration on a frozen clone did not use the overlay", seed)
		}
		layered.FreezeClassLookup()
		twin.FreezeClassLookup()
		if layered.classLookupBuilds != 0 {
			t.Fatalf("seed %d: layered clone ran %d full class-lookup builds", seed, layered.classLookupBuilds)
		}
		tied := tiedNow(layered, twin)
		assertSameFrozenGeneration(t, fmt.Sprintf("seed %d layer", seed), layered, twin, tied)
		assertSameClassLookups(t, fmt.Sprintf("seed %d layer", seed), layered, twin, tied)
		assertSameCloneLookups(t, fmt.Sprintf("seed %d layer clone", seed), layered, twin, tied)
		history = append(history, layered.Classes, twin.Classes)
		if copyGroupsMixIdentities(twin) || copyGroupsMixIdentities(layered) {
			// Clones of twin take a map-order primary's value; the deeper
			// stage would start from different class maps.
			continue
		}

		deeper := layered.CloneRuntimeFrozenShared(nil)
		deeperTwin := fullRebuildTwin(twin)
		for i := 1 + rng.Intn(3); i > 0; i-- {
			class := randomOverlayClass(rng)
			registerAll(t, deeper, []Class{class})
			registerAll(t, deeperTwin, []Class{class})
			assertSameClassLookups(t, fmt.Sprintf("seed %d deeper pending", seed), deeper, deeperTwin, tiedNow(deeper, deeperTwin))
		}
		deeper.FreezeClassLookup()
		deeperTwin.FreezeClassLookup()
		if deeper.frozenClassLookup.base != root {
			t.Fatalf("seed %d: second layer is not flattened onto the root", seed)
		}
		tied = tiedNow(deeper, deeperTwin)
		assertSameFrozenGeneration(t, fmt.Sprintf("seed %d deeper", seed), deeper, deeperTwin, tied)
		assertSameClassLookups(t, fmt.Sprintf("seed %d deeper", seed), deeper, deeperTwin, tied)
		assertSameCloneLookups(t, fmt.Sprintf("seed %d deeper clone", seed), deeper, deeperTwin, tied)

		if !reflect.DeepEqual(rootKeys, root.keys) {
			t.Fatalf("seed %d: layers changed the shared root keys", seed)
		}
		sibling := base.CloneRuntimeFrozenShared(nil)
		assertSameClassLookups(t, fmt.Sprintf("seed %d sibling", seed), sibling, base.CloneRuntimeFrozenShared(nil), nil)
		if !anyTie {
			untied++
		}
		compared++
	}
	// Ties are map-order choices in the full build and are skipped above;
	// most seeds must still be compared on every key and namespace table.
	if untied < seeds*2/5 || compared < seeds*2/5 {
		t.Fatalf("only %d of %d seeds reached the second layer and %d were free of map-order ties", compared, seeds, untied)
	}
	t.Logf("%d of %d seeds reached the second layer; %d had no map-order ties", compared, seeds, untied)
}

// assertSameCloneLookups compares clones of two frozen runtimes, unless a
// copy group's map-order primary decides the clones' values.
func assertSameCloneLookups(t *testing.T, label string, got, want *VM, tied map[string]bool) {
	t.Helper()
	if copyGroupsMixIdentities(want) {
		return
	}
	assertSameClassLookups(t, label, got.CloneRuntimeFrozenShared(nil), want.CloneRuntimeFrozenShared(nil), tied)
}

// copyGroupsMixIdentities reports a classCopyKey group whose names hold
// different values. A clone gives the whole group one primary's value, and a
// full build picks that primary by map order.
func copyGroupsMixIdentities(machine *VM) bool {
	return !machine.sharedClassCopyPlan.uniformAliases
}

func snapshotStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// A local inner class Pkg.Worker outranks the managed pkg.Worker at the same
// canonical key, so the layer entry must be read before the root's.
func TestClassLookupOverlayLayerEntryShadowsRoot(t *testing.T) {
	base := New(nil)
	registerAll(t, base, []Class{{Name: "Worker", Namespace: "pkg", Dependency: true}})
	base.FreezeClassLookup()
	rootAlias, ok := base.frozenClassLookup.resolve("pkg.worker")
	if !ok || base.Classes[rootAlias].Namespace != "pkg" {
		t.Fatalf("root pkg.worker = %q, %t", rootAlias, ok)
	}

	clone := base.CloneRuntimeFrozenShared(nil)
	registerAll(t, clone, []Class{{Name: "Pkg.Worker"}})
	clone.FreezeClassLookup()
	layer := clone.frozenClassLookup
	if layer.base != base.frozenClassLookup {
		t.Fatal("clone freeze did not layer over the shared root")
	}
	if alias, ok := layer.keys["pkg.worker"]; !ok || alias != "Pkg.Worker" {
		t.Fatalf("layer pkg.worker = %q, %t; want the local inner class", alias, ok)
	}
	for _, name := range []string{"PKG.WORKER", "pkg.worker", "Pkg.Worker"} {
		class, ok := clone.lookupClass(name)
		if !ok || class.Dependency || class.Name != "Pkg.Worker" {
			t.Fatalf("clone lookupClass(%q) = %s", name, classIdentity(class, ok))
		}
	}
	if class, ok := clone.lookupClass("pkg.Worker"); !ok || class.Dependency {
		t.Fatalf("clone pkg.Worker = %s, want the local class", classIdentity(class, ok))
	}
	sibling := base.CloneRuntimeFrozenShared(nil)
	if class, ok := sibling.lookupClass("PKG.WORKER"); !ok || !class.Dependency {
		t.Fatalf("sibling PKG.WORKER = %s, want the managed class", classIdentity(class, ok))
	}
}

// Replacing the class at a frozen alias can leave a canonical key with no
// contributor. The layer hides the root entry with a tombstone.
func TestClassLookupOverlayTombstonesReplacedAlias(t *testing.T) {
	base := New(nil)
	registerAll(t, base, []Class{{Name: "ns.Foo", Namespace: "ns", Dependency: true}})
	base.FreezeClassLookup()
	if _, ok := base.lookupClass("ns.ns.Foo"); !ok {
		t.Fatal("root does not index the namespace-qualified spelling")
	}

	clone := base.CloneRuntimeFrozenShared(nil)
	twin := fullRebuildTwin(base)
	registerAll(t, clone, []Class{{Name: "ns.Foo"}})
	registerAll(t, twin, []Class{{Name: "ns.Foo"}})
	// Before the freeze the old lookup snapshot still answers the stale key.
	assertSameClassLookups(t, "pending", clone, twin, nil)
	if class, ok := clone.lookupClass("ns.ns.foo"); !ok || !class.Dependency {
		t.Fatalf("pending ns.ns.foo = %s, want the replaced class value", classIdentity(class, ok))
	}

	clone.FreezeClassLookup()
	twin.FreezeClassLookup()
	if alias, ok := clone.frozenClassLookup.keys["ns.ns.foo"]; !ok || alias != "" {
		t.Fatalf("layer ns.ns.foo = %q, %t; want a tombstone", alias, ok)
	}
	if class, ok := clone.lookupClass("ns.ns.Foo"); ok {
		t.Fatalf("tombstoned key resolved to %s", classIdentity(class, ok))
	}
	assertSameFrozenGeneration(t, "frozen", clone, twin, nil)
	if _, ok := base.CloneRuntimeFrozenShared(nil).lookupClass("ns.ns.Foo"); !ok {
		t.Fatal("tombstone reached a sibling clone")
	}
}

// A cached miss, in the name cache or in a shared namespace table, must not
// hide a class registered afterwards, pending or frozen.
func TestClassLookupOverlayNegativeEntriesDoNotHideRegistration(t *testing.T) {
	base := New(nil)
	registerAll(t, base, []Class{{Name: "Worker", Namespace: "pkg", Dependency: true}, {Name: "Local"}})
	base.FreezeClassLookup()
	warm := base.CloneRuntimeFrozenShared(nil)
	for _, namespace := range []string{"", "pkg"} {
		if _, ok := warm.lookupClassInNamespace(namespace, "Late"); ok {
			t.Fatalf("Late resolved in %q before registration", namespace)
		}
	}

	clone := base.CloneRuntimeFrozenShared(nil)
	for _, name := range []string{"Late", "late", "pkg.Late", "Local.Late"} {
		if _, ok := clone.lookupClass(name); ok {
			t.Fatalf("%s resolved before registration", name)
		}
	}
	if _, ok := clone.lookupClassInNamespace("pkg", "Late"); ok {
		t.Fatal("pkg Late resolved before registration")
	}
	registerAll(t, clone, []Class{{Name: "Late", Namespace: "pkg", Dependency: true}, {Name: "Local.Late"}})
	check := func(label string, machine *VM) {
		t.Helper()
		for _, name := range []string{"Late", "late", "pkg.Late", "PKG.LATE", "Local.Late", "local.late"} {
			if _, ok := machine.lookupClass(name); !ok {
				t.Fatalf("%s: %s still misses after registration", label, name)
			}
		}
		if _, ok := machine.lookupClassInNamespace("pkg", "Late"); !ok {
			t.Fatalf("%s: pkg Late still misses after registration", label)
		}
	}
	check("pending", clone)
	clone.FreezeClassLookup()
	check("frozen", clone)
	check("frozen clone", clone.CloneRuntimeFrozenShared(nil))
	if _, ok := base.CloneRuntimeFrozenShared(nil).lookupClassInNamespace("pkg", "Late"); ok {
		t.Fatal("registration reached the shared namespace table")
	}
}

// Every artifact of the shared root generation stays as it was.
func TestClassLookupOverlayLeavesSharedRootUnchanged(t *testing.T) {
	base := New(nil)
	registerAll(t, base, []Class{
		{Name: "Worker", Namespace: "pkg", Dependency: true},
		{Name: "Worker"},
		{Name: "Outer"},
		{Name: "Outer.Inner"},
	})
	base.FreezeClassLookup()
	root := base.frozenClassLookup
	warm := base.CloneRuntimeFrozenShared(nil)
	warm.lookupClassInNamespace("", "Worker")
	warm.lookupClassInNamespace("pkg", "Worker")
	keys := snapshotStringMap(root.keys)
	search := append([]classNameSearchEntry(nil), root.searchEntries...)
	topLevel := fmt.Sprint(root.topLevel)
	plan := copyPlanGroups(root.copyPlan)
	planPrimaries := append([]string(nil), root.copyPlan.primaries...)
	classes := snapshotClassMap(base.Classes)
	root.namespaceMu.RLock()
	namespaces := fmt.Sprint(root.namespaceAliases)
	root.namespaceMu.RUnlock()

	clone := base.CloneRuntimeFrozenShared(nil)
	registerAll(t, clone, []Class{{Name: "Worker", Namespace: "abc", Dependency: true}, {Name: "Outer.Added"}, {Name: "Added"}})
	clone.FreezeClassLookup()
	deeper := clone.CloneRuntimeFrozenShared(nil)
	registerAll(t, deeper, []Class{{Name: "Deeper"}})
	deeper.FreezeClassLookup()
	deeper.lookupClassInNamespace("", "Added")

	if !reflect.DeepEqual(keys, root.keys) {
		t.Fatal("root keys changed")
	}
	if !reflect.DeepEqual(search, root.searchEntries) {
		t.Fatal("root search entries changed")
	}
	if fmt.Sprint(root.topLevel) != topLevel {
		t.Fatal("root top-level index changed")
	}
	if !reflect.DeepEqual(plan, copyPlanGroups(root.copyPlan)) || !reflect.DeepEqual(planPrimaries, root.copyPlan.primaries) {
		t.Fatal("root copy plan changed")
	}
	root.namespaceMu.RLock()
	afterNamespaces := fmt.Sprint(root.namespaceAliases)
	root.namespaceMu.RUnlock()
	if afterNamespaces != namespaces {
		t.Fatalf("root namespace tables changed:\n%s\n%s", namespaces, afterNamespaces)
	}
	if base.frozenClassLookup != root {
		t.Fatal("base frozen generation replaced")
	}
	assertClassMapUnchanged(t, "base classes", base.Classes, classes)
}

// Sibling clones registering different classes on one root run concurrently
// and see only their own registrations. Run with -race.
func TestClassLookupOverlaySiblingClonesStayIsolated(t *testing.T) {
	base := classMapShareBase(t, 200)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			clone := base.CloneRuntimeFrozenShared(nil)
			own := fmt.Sprintf("Sibling%02d", worker)
			if err := clone.RegisterClass(Class{Name: own, Namespace: "pkg", Dependency: worker%2 == 0}); err != nil {
				errs <- err
				return
			}
			clone.FreezeClassLookup()
			row := clone.CloneRuntimeFrozenShared(nil)
			for other := 0; other < 16; other++ {
				name := fmt.Sprintf("PKG.SIBLING%02d", other)
				_, ok := row.lookupClass(name)
				if _, nsOK := row.lookupClassInNamespace("pkg", fmt.Sprintf("sibling%02d", other)); nsOK != ok {
					errs <- fmt.Errorf("worker %d: namespace lookup of %s = %t, lookupClass %t", worker, name, nsOK, ok)
					return
				}
				if ok != (other == worker) {
					errs <- fmt.Errorf("worker %d: lookupClass(%s) = %t", worker, name, ok)
					return
				}
			}
			if _, ok := row.lookupClass("pkg.Registry"); !ok {
				errs <- fmt.Errorf("worker %d lost a root class", worker)
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for worker := 0; worker < 16; worker++ {
		if _, ok := base.lookupClass(fmt.Sprintf("pkg.Sibling%02d", worker)); ok {
			t.Fatalf("Sibling%02d reached the base", worker)
		}
	}
}

// Every write path that can follow a registration keeps the result equal to
// the full rebuild: class value writes while the overlay is pending, a clone of
// a pending runtime, re-registration of an existing class, and a write
// inherited from the source runtime.
func TestClassLookupOverlayInvalidatesOnEveryMutationPath(t *testing.T) {
	newBase := func() *VM {
		base := New(nil)
		registerAll(t, base, []Class{
			{Name: "Worker", Namespace: "pkg", Dependency: true, StaticFields: map[string]Field{"Count": {Name: "Count", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)}}},
			{Name: "Local"},
		})
		base.FreezeClassLookup()
		return base
	}
	t.Run("value write while pending", func(t *testing.T) {
		base := newBase()
		clone := base.CloneRuntimeFrozenShared(nil)
		twin := fullRebuildTwin(base)
		for _, machine := range []*VM{clone, twin} {
			registerAll(t, machine, []Class{{Name: "Added"}})
			class, ok := machine.lookupClass("pkg.Worker")
			if !ok {
				t.Fatal("pkg.Worker missing")
			}
			class.StaticFields = copyFieldMap(class.StaticFields)
			class.StaticFields["Count"] = Field{Name: "Count", Type: "Integer", Static: true, Value: Int(7), InitialValue: Int(0)}
			machine.storeClassValue(class)
		}
		assertSameClassLookups(t, "pending after value write", clone, twin, nil)
		clone.FreezeClassLookup()
		twin.FreezeClassLookup()
		if clone.frozenClassLookup.base != nil || clone.classLookupBuilds != 1 {
			t.Fatalf("value write kept the layered freeze: base=%v builds=%d", clone.frozenClassLookup.base != nil, clone.classLookupBuilds)
		}
		assertSameFrozenGeneration(t, "value write", clone, twin, nil)
		if class, _ := clone.lookupClass("PKG.WORKER"); class.StaticFields["Count"].Value.Int != 7 {
			t.Fatalf("written static value lost: %+v", class.StaticFields["Count"])
		}
	})
	t.Run("clone of pending runtime", func(t *testing.T) {
		base := newBase()
		clone := base.CloneRuntimeFrozenShared(nil)
		twin := fullRebuildTwin(base)
		registerAll(t, clone, []Class{{Name: "Added"}})
		registerAll(t, twin, []Class{{Name: "Added"}})
		pendingClone := clone.CloneRuntime(nil)
		if pendingClone.classOverlay != nil {
			t.Fatal("clone inherited a pending overlay")
		}
		assertSameClassLookups(t, "clone of pending", pendingClone, twin.CloneRuntime(nil), nil)
		registerAll(t, pendingClone, []Class{{Name: "Other"}})
		if _, ok := clone.lookupClass("Other"); ok {
			t.Fatal("registration on a clone of a pending runtime reached its source")
		}
	})
	t.Run("re-registration", func(t *testing.T) {
		base := newBase()
		clone := base.CloneRuntimeFrozenShared(nil)
		twin := fullRebuildTwin(base)
		again := Class{Name: "Worker", Namespace: "pkg", Dependency: true, Fields: map[string]Field{"extra": {Name: "extra", Type: "Integer"}}}
		registerAll(t, clone, []Class{again, {Name: "Local"}})
		registerAll(t, twin, []Class{again, {Name: "Local"}})
		assertSameClassLookups(t, "pending re-registration", clone, twin, nil)
		clone.FreezeClassLookup()
		twin.FreezeClassLookup()
		assertSameFrozenGeneration(t, "re-registration", clone, twin, nil)
		if class, _ := clone.lookupClass("pkg.worker"); class.Fields["extra"].Name != "extra" {
			t.Fatal("re-registered class value not visible")
		}
	})
	t.Run("inherited value write", func(t *testing.T) {
		base := newBase()
		source := base.CloneRuntimeFrozenShared(nil)
		class, _ := source.ensureMutableClass("pkg.Worker")
		class.StaticFields["Count"] = Field{Name: "Count", Type: "Integer", Static: true, Value: Int(3)}
		source.storeMutableClassAtAlias("pkg.Worker", class)
		clone := source.CloneRuntimeFrozenShared(nil)
		if !clone.classValuesWritten {
			t.Fatal("clone did not inherit the source's value write")
		}
		registerAll(t, clone, []Class{{Name: "Added"}})
		if clone.classOverlay != nil {
			t.Fatal("overlay used after an inherited class value write")
		}
	})
}

// The overlay is what removes the per-registration rebuild: a frozen clone
// that registers classes and freezes must run no full build, and its
// allocations must not grow with the number of frozen classes.
func TestClassLookupOverlayCostIndependentOfClassCount(t *testing.T) {
	small := classMapShareBase(t, 20)
	large := classMapShareBase(t, 4000)
	register := func(base *VM) *VM {
		clone := base.CloneRuntimeFrozenShared(nil)
		for _, class := range []Class{{Name: "RowHelper"}, {Name: "RowWorker", Namespace: "pkg", Dependency: true}, {Name: "RowHelper.Inner"}} {
			if err := clone.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
		}
		clone.FreezeClassLookup()
		return clone
	}
	clone := register(large)
	if clone.classLookupBuilds != 0 {
		t.Fatalf("register and freeze on a frozen clone ran %d full class-lookup builds", clone.classLookupBuilds)
	}
	if clone.frozenClassLookup.base != large.frozenClassLookup || len(clone.frozenClassLookup.keys) > 16 {
		t.Fatalf("layer holds %d keys, want only the registered names", len(clone.frozenClassLookup.keys))
	}
	// The copy of the shared Classes map on the first write is the one
	// O(len(Classes)) step left (Class values are stored indirectly, one
	// allocation per entry). Measure registration and freeze after it.
	mallocs := func(base *VM) float64 {
		const runs = 10
		var total uint64
		for i := 0; i < runs; i++ {
			clone := base.CloneRuntimeFrozenShared(nil)
			clone.prepareClassMapWrite()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for _, class := range []Class{{Name: "RowHelper"}, {Name: "RowWorker", Namespace: "pkg", Dependency: true}, {Name: "RowHelper.Inner"}} {
				if err := clone.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			clone.FreezeClassLookup()
			runtime.ReadMemStats(&after)
			total += after.Mallocs - before.Mallocs
		}
		return float64(total) / runs
	}
	smallAllocs := mallocs(small)
	largeAllocs := mallocs(large)
	if largeAllocs > smallAllocs+64 {
		t.Fatalf("register+freeze over %d classes allocates %.0f times, over %d classes %.0f",
			len(large.Classes), largeAllocs, len(small.Classes), smallAllocs)
	}
}
