package apextest

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func recordDeferredRuntimeInputs(t *testing.T) func() [][2]string {
	t.Helper()
	var mu sync.Mutex
	var pairs [][2]string
	runtimePatchVerifyDeferredRuntimeInputs = func(deferred, asBuilt string) {
		mu.Lock()
		pairs = append(pairs, [2]string{deferred, asBuilt})
		mu.Unlock()
	}
	t.Cleanup(func() { runtimePatchVerifyDeferredRuntimeInputs = nil })
	return func() [][2]string {
		mu.Lock()
		defer mu.Unlock()
		return append([][2]string(nil), pairs...)
	}
}

// Tests run on clones of the cached template between its cold build and the
// first transition attempt. The deferred fingerprint, computed from the
// retained template org before the run's first CloneOrg, must equal the
// fingerprint of the org as built, and must still authorize the patch.
func TestRuntimePatchDeferredRuntimeInputsDescribeOrgAsBuilt(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	pairs := recordDeferredRuntimeInputs(t)
	fixture := newRuntimeTransitionFixture(t)
	writeFile(t, filepath.Join(fixture.root, "force-app/main/default/classes/TransitionWorkloadTest.cls"), `@IsTest
private class TransitionWorkloadTest {
    @IsTest static void dmlDescribeAndSchemaQueries() {
        Account parent = new Account(Name = 'Parent');
        insert parent;
        insert new Contact(LastName = 'Child', AccountId = parent.Id);
        Map<String, Schema.SObjectType> globalDescribe = Schema.getGlobalDescribe();
        Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
        List<Schema.ChildRelationship> children = Account.SObjectType.getDescribe().getChildRelationships();
        List<EntityDefinition> entities = [SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName = 'Account'];
        System.assertEquals(1, [SELECT COUNT() FROM Contact WHERE AccountId = :parent.Id]);
        System.assert(globalDescribe.containsKey('account'));
        System.assert(fields.containsKey('name'));
        System.assert(!children.isEmpty());
        System.assertEquals(1, entities.size());
        System.assertEquals(7, StableOwner.value());
    }
}`)
	previous, previousDigests := fixture.fullIndex(t)
	run := Run(previous, Options{SourceDigests: previousDigests, NoDiskCache: true})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("workload run summary = %+v, run %+v", summary, run)
	}
	// The run's CloneOrg computes the deferred fingerprint before any clone
	// exists.
	if got := pairs(); len(got) != 1 || got[0][0] == "" || got[0][0] != got[0][1] {
		t.Fatalf("run did not compute the deferred fingerprint of the org as built: %v", got)
	}

	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe Apex transition did not produce affected closure")
	}
	if _, _, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected); err != nil || !outcome.Applied {
		t.Fatalf("transition after test execution was not applied: %+v, %v", outcome, err)
	}
	got := pairs()
	if len(got) != 1 {
		t.Fatalf("deferred fingerprint computed %d times, want once: %v", len(got), got)
	}
	for _, pair := range got {
		if pair[0] == "" || pair[0] != pair[1] {
			t.Fatalf("deferred fingerprint %q differs from the org as built %q", pair[0], pair[1])
		}
	}
}

// CloneOrg hands out clones that share the template's object definitions, so
// an in-place write through a clone reaches the template. The deferred
// fingerprint must still describe the org as built (it is computed before the
// first CloneOrg returns), or the transition must fail closed. The label case
// leaves the schema stamp unchanged.
func TestRuntimePatchDeferredRuntimeInputsIgnoreWritesThroughCloneOrg(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*storage.Field)
		read   func(storage.Field) string
		value  string
	}{
		{"Account.Name.Label", func(field *storage.Field) { field.Label = "Mutated Name" }, func(field storage.Field) string { return field.Label }, "Mutated Name"},
		{"Account.Name.RelationshipName", func(field *storage.Field) { field.RelationshipName = "Broken" }, func(field storage.Field) string { return field.RelationshipName }, "Broken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			InvalidateRuntimeCaches()
			t.Cleanup(InvalidateRuntimeCaches)
			pairs := recordDeferredRuntimeInputs(t)
			fixture := newRuntimeTransitionFixture(t)
			previous, previousDigests := fixture.fullIndex(t)
			previousKey, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}
			cached, ok := validMemoryRuntimeEntry(previousKey)
			if !ok || cached.patchAuthority == nil {
				t.Fatal("cold build did not publish a patch authority")
			}
			clone := cached.restored.CloneOrg()
			fields := clone.Objects["Account"].Definition.Fields
			field := fields["Name"]
			tc.mutate(&field)
			fields["Name"] = field
			if got := tc.read(cached.restored.CloneOrg().Objects["Account"].Definition.Fields["Name"]); got != tc.value {
				t.Fatalf("in-place edit did not reach the shared template (got %q); the test no longer models the hazard", got)
			}

			writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
			current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
			affected, ok := runtimePatchOneModifiedOwner(previous, current)
			if !ok {
				t.Fatal("safe Apex transition did not produce affected closure")
			}
			_, _, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
			if err != nil {
				t.Fatal(err)
			}
			got := pairs()
			if len(got) != 1 {
				t.Fatalf("deferred fingerprint computed %d times, want 1: %v", len(got), got)
			}
			deferred, asBuilt := got[0][0], got[0][1]
			if asBuilt == "" {
				t.Fatal("as-built fingerprint is empty")
			}
			failedClosed := deferred == "" && !outcome.Applied
			if deferred != asBuilt && !failedClosed {
				t.Fatalf("deferred fingerprint %q describes the mutated template, not the org as built %q (applied %v)", deferred, asBuilt, outcome.Applied)
			}
		})
	}
}

// Fails if the ambient fingerprint is computed eagerly again: a cold build
// computes none. The transition computes the current org's twice, in the
// attempt and in its flight, and the deferred predecessor's once.
func TestRuntimePatchColdBuildDefersAmbientFingerprint(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	before := runtimePatchAmbientFingerprintCalls.Load()
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	if calls := runtimePatchAmbientFingerprintCalls.Load() - before; calls != 0 {
		t.Fatalf("cold build computed %d ambient fingerprints, want 0", calls)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe Apex transition did not produce affected closure")
	}
	before = runtimePatchAmbientFingerprintCalls.Load()
	if _, _, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected); err != nil || !outcome.Applied {
		t.Fatalf("transition was not applied: %+v, %v", outcome, err)
	}
	if calls := runtimePatchAmbientFingerprintCalls.Load() - before; calls != 3 {
		t.Fatalf("transition computed %d ambient fingerprints, want 3", calls)
	}
}
