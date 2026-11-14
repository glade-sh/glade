package storage

import (
	"reflect"
	"sync"
	"testing"
)

// resetSpreadMemos clears the template map and the decoded catalog, and turns
// release on for the test.
func resetSpreadMemos(t *testing.T) {
	t.Helper()
	was := SetStandardCatalogReleaseEnabled(true)
	t.Cleanup(func() { SetStandardCatalogReleaseEnabled(was) })
	clear := func() {
		spreadTemplates.Clear()
		droppedPlainTemplates.mu.Lock()
		droppedPlainTemplates.keys = nil
		droppedPlainTemplates.mu.Unlock()
		standardDescribeCatalogV2ProductionCache.entries.Clear()
		standardDescribeCatalogV2RawProductionCache.entries.Clear()
	}
	clear()
	t.Cleanup(clear)
}

func spreadTemplatePhases() (plain, shaped int) {
	spreadTemplates.Range(func(k, _ any) bool {
		switch k.(spreadTemplateKey).phase {
		case standardSpreadPlain:
			plain++
		case standardSpreadShaped:
			shaped++
		}
		return true
	})
	return
}

func TestMaterializeSpreadReleasesDecodedCatalog(t *testing.T) {
	resetSpreadMemos(t)
	lazy := sourceSpreadLazyOrg([]string{"Communities"})
	if !MaterializeStandardObjectSpread(&lazy) {
		t.Fatal("materialize reported no change")
	}
	if n := StandardCatalogDecodedObjectCount(); n != 0 {
		t.Fatalf("decoded catalog objects after completed spread = %d, want 0", n)
	}
	// Re-decode on demand, same result as an eager org.
	compareCompleteOrgs(t, sourceSpreadEagerOrg([]string{"Communities"}), lazy)
}

func TestMaterializeSpreadKeepsCatalogWhenReleaseDisabled(t *testing.T) {
	resetSpreadMemos(t)
	SetStandardCatalogReleaseEnabled(false)
	lazy := sourceSpreadLazyOrg([]string{"Communities"})
	MaterializeStandardObjectSpread(&lazy)
	if n := StandardCatalogDecodedObjectCount(); n == 0 {
		t.Fatal("catalog released although release is disabled")
	}
}

func TestLazyInstallsBoundDecodedCatalog(t *testing.T) {
	resetSpreadMemos(t)
	lazy := sourceSpreadLazyOrg([]string{"Communities"})
	releases := StandardCatalogReleaseCount()
	installed := 0
	for _, name := range KnownStandardObjectNames() {
		if _, ok := installSpreadObject(&lazy, name); ok {
			installed++
		}
		if n := StandardCatalogDecodedObjectCount(); n > spreadTemplateReleaseEvery {
			t.Fatalf("after %d installs %d decoded objects, want at most %d", installed, n, spreadTemplateReleaseEvery)
		}
	}
	if installed < 1100 {
		t.Fatalf("only %d installs; the bound is not exercised", installed)
	}
	if StandardCatalogReleaseCount() == releases {
		t.Fatalf("no release during %d lazy installs", installed)
	}
	MaterializeStandardObjectSpread(&lazy)
	compareCompleteOrgs(t, sourceSpreadEagerOrg([]string{"Communities"}), lazy)
}

func TestReleaseDecodesAgainOnDemand(t *testing.T) {
	resetSpreadMemos(t)
	first, ok := standardObjectCatalogEntryForName("Account")
	if !ok {
		t.Fatal("Account missing")
	}
	org := sourceSpreadEagerOrg(nil)
	before := StandardCatalogMemberDecodeCount()
	ReleaseStandardCatalogCaches()
	again := sourceSpreadEagerOrg(nil)
	if StandardCatalogMemberDecodeCount() == before {
		t.Fatal("no decode after release")
	}
	compareCompleteOrgs(t, org, again)
	second, _ := standardObjectCatalogEntryForName("Account")
	if !reflect.DeepEqual(first, second) {
		t.Fatal("catalog entry changed after release")
	}
}

func TestSeedReleaseDropsPlainTemplates(t *testing.T) {
	resetSpreadMemos(t)
	// Plain phase: an org before the shape applies.
	plain := NewOrgState()
	plain.OrgID = DefaultOrgID
	plain.EnableStandardSpreadSource()
	plainNames := owedNames(&plain, 4)
	for _, name := range plainNames {
		if _, ok := installSpreadObject(&plain, name); !ok {
			t.Fatalf("plain install %s", name)
		}
	}
	// Only a plain group exists: nothing is dead yet.
	ReleaseStandardCatalogCaches()
	if p, _ := spreadTemplatePhases(); p == 0 {
		t.Fatal("plain templates dropped while no shaped group exists")
	}
	before := map[string]ObjectState{}
	for _, name := range plainNames {
		tmpl, _ := spreadTemplate(&plain, name)
		before[name] = tmpl
	}
	shaped := sourceSpreadLazyOrg([]string{"Communities"})
	MaterializeStandardObjectSpread(&shaped)
	if p, s := spreadTemplatePhases(); p != 0 || s == 0 {
		t.Fatalf("after shaped seed: %d plain and %d shaped cells, want 0 and >0", p, s)
	}
	// A later plain org builds equal templates.
	for _, name := range plainNames {
		tmpl, ok := spreadTemplate(&plain, name)
		if !ok || !reflect.DeepEqual(before[name].Definition, tmpl.Definition) {
			t.Errorf("%s: rebuilt plain template differs", name)
		}
	}
}

// A plain org that installed objects before the drop still reports them as
// untouched after it, so a fingerprint does not change.
func TestSpreadObjectUntouchedSurvivesPlainDrop(t *testing.T) {
	resetSpreadMemos(t)
	plain := NewOrgState()
	plain.OrgID = DefaultOrgID
	plain.EnableStandardSpreadSource()
	names := owedNames(&plain, 3)
	for _, name := range names[:2] {
		installSpreadObject(&plain, name)
	}
	if !plain.SpreadObjectUntouched(names[0]) {
		t.Fatal("installed object not untouched before drop")
	}
	shaped := sourceSpreadLazyOrg([]string{"Communities"})
	MaterializeStandardObjectSpread(&shaped)
	if p, _ := spreadTemplatePhases(); p != 0 {
		t.Fatalf("%d plain cells left", p)
	}
	if !plain.SpreadObjectUntouched(names[0]) || !plain.SpreadObjectUntouched(names[1]) {
		t.Fatal("untouched answer changed after plain drop")
	}
	if plain.SpreadObjectUntouched(names[2]) {
		t.Fatal("object never installed reported untouched")
	}
}

func TestSpreadReleaseRaceMatchesEager(t *testing.T) {
	resetSpreadMemos(t)
	features := []string{"Communities"}
	eager := sourceSpreadEagerOrg(features)
	names := KnownStandardObjectNames()
	orgs := make([]OrgState, 4)
	for i := range orgs {
		orgs[i] = sourceSpreadLazyOrg(features)
	}
	stop := make(chan struct{})
	var releaser sync.WaitGroup
	releaser.Add(1)
	go func() {
		defer releaser.Done()
		for {
			select {
			case <-stop:
				return
			default:
				ReleaseStandardCatalogCaches()
			}
		}
	}()
	var wg sync.WaitGroup
	for i := range orgs {
		wg.Add(1)
		go func(org *OrgState, offset int) {
			defer wg.Done()
			for j := range names {
				installSpreadObject(org, names[(j+offset*97)%len(names)])
				org.SpreadObjectUntouched(names[j])
			}
			MaterializeStandardObjectSpread(org)
		}(&orgs[i], i)
	}
	wg.Wait()
	close(stop)
	releaser.Wait()
	for i := range orgs {
		compareCompleteOrgs(t, eager, orgs[i])
	}
}

// owedNames returns the first n catalog objects the lazy org still owes.
func owedNames(org *OrgState, n int) []string {
	var out []string
	for _, name := range KnownStandardObjectNames() {
		if _, owed := spreadObjectOwed(org, name); owed {
			out = append(out, name)
			if len(out) == n {
				break
			}
		}
	}
	return out
}

// Untouched checks on a populated plain org after a plain drop rebuild cells.
// The rebuilds count toward the release cadence, so the decoded catalog stays
// bounded, and a second pass over the same names decodes nothing.
func TestSpreadObjectUntouchedAfterPlainDropDoesNotDecodeStorm(t *testing.T) {
	resetSpreadMemos(t)
	plain := NewOrgState()
	plain.OrgID = DefaultOrgID
	plain.EnableStandardSpreadSource()
	names := owedNames(&plain, 1300)
	if len(names) < 1300 {
		t.Fatalf("only %d owed objects", len(names))
	}
	for _, name := range names {
		installSpreadObject(&plain, name)
	}
	shaped := sourceSpreadLazyOrg([]string{"Communities"})
	installSpreadObject(&shaped, owedNames(&shaped, 1)[0])
	ReleaseStandardCatalogCaches()
	if p, s := spreadTemplatePhases(); p != 0 || s == 0 {
		t.Fatalf("after release: %d plain and %d shaped cells", p, s)
	}
	for _, name := range names {
		if !plain.SpreadObjectUntouched(name) {
			t.Fatalf("%s not untouched after drop", name)
		}
		if n := StandardCatalogDecodedObjectCount(); n > spreadTemplateReleaseEvery {
			t.Fatalf("decoded %d objects, want at most %d", n, spreadTemplateReleaseEvery)
		}
	}
	// Completing another shaped spread releases the catalog again. It must
	// retain the plain cells rebuilt by the first pass.
	again := sourceSpreadLazyOrg([]string{"Communities"})
	MaterializeStandardObjectSpread(&again)
	before := StandardCatalogMemberDecodeCount()
	for _, name := range names {
		if !plain.SpreadObjectUntouched(name) {
			t.Fatalf("%s not untouched on second pass", name)
		}
	}
	if got := StandardCatalogMemberDecodeCount() - before; got != 0 {
		t.Fatalf("second pass decoded %d members, want 0", got)
	}
}

// A dropped key protects only its own rebuilt cell; each template-key input
// distinguishes another cell that can still be released at its first drop.
func TestPlainDropTracksEveryTemplateKeyInput(t *testing.T) {
	resetSpreadMemos(t)
	base := spreadTemplateKey{phase: standardSpreadPlain, signature: "base", object: "Account"}
	spreadTemplates.Store(spreadTemplateKey{phase: standardSpreadShaped}, &spreadTemplateCell{})
	spreadTemplates.Store(base, &spreadTemplateCell{})
	ReleaseStandardCatalogCaches()
	rebuilt := &spreadTemplateCell{}
	spreadTemplates.Store(base, rebuilt)
	variants := map[string]spreadTemplateKey{
		"phase":      {phase: standardSpreadShaped, signature: base.signature, object: base.object},
		"signature":  {phase: base.phase, signature: "other", object: base.object},
		"currency":   {phase: base.phase, signature: base.signature, multiCurr: true, object: base.object},
		"recordType": {phase: base.phase, signature: base.signature, recordType: true, object: base.object},
		"object":     {phase: base.phase, signature: base.signature, object: "Contact"},
	}
	for _, key := range variants {
		spreadTemplates.Store(key, &spreadTemplateCell{})
	}
	ReleaseStandardCatalogCaches()
	if cell, ok := spreadTemplates.Load(base); !ok || cell != rebuilt {
		t.Fatal("rebuilt cell was dropped")
	}
	for name, key := range variants {
		_, present := spreadTemplates.Load(key)
		if present != (name == "phase") {
			t.Errorf("%s: present = %t", name, present)
		}
	}
}
