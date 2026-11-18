package apextest

import (
	"path/filepath"
	"testing"
)

const oneShotWorkloadSource = `@IsTest
private class OneShotWorkloadTest {
    @IsTest static void runs() {
        Account parent = new Account(Name = 'Parent');
        insert parent;
        System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Id = :parent.Id]);
        System.assertEquals(7, StableOwner.value());
    }
}`

func runOneShotWorkload(t *testing.T, oneShot bool) (calls int64, summary [2]int, fixture runtimeTransitionFixture) {
	t.Helper()
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture = newRuntimeTransitionFixture(t)
	writeFile(t, filepath.Join(fixture.root, "force-app/main/default/classes/OneShotWorkloadTest.cls"), oneShotWorkloadSource)
	index, digests := fixture.fullIndex(t)
	before := runtimePatchAmbientFingerprintCalls.Load()
	run := Run(index, Options{SourceDigests: digests, NoDiskCache: true, OneShotProcess: oneShot})
	got := run.Summary()
	return runtimePatchAmbientFingerprintCalls.Load() - before, [2]int{got.Total, got.Passed}, fixture
}

// Fails if a one-shot run computes the ambient org fingerprint: no later
// transition can use it, and it costs about 0.1 s per cold process.
func TestOneShotRunSkipsAmbientFingerprint(t *testing.T) {
	calls, summary, _ := runOneShotWorkload(t, true)
	if calls != 0 {
		t.Fatalf("one-shot run computed %d ambient fingerprints, want 0", calls)
	}
	if summary != [2]int{1, 1} {
		t.Fatalf("one-shot run summary total/passed = %v, want [1 1]", summary)
	}
}

// The default route keeps the fingerprint: cached, daemon, server and watch
// runs can transition later.
func TestDefaultRunKeepsAmbientFingerprint(t *testing.T) {
	calls, summary, _ := runOneShotWorkload(t, false)
	if calls != 1 {
		t.Fatalf("default run computed %d ambient fingerprints, want 1", calls)
	}
	if summary != [2]int{1, 1} {
		t.Fatalf("default run summary total/passed = %v, want [1 1]", summary)
	}
}

// A transition after a one-shot entry fails closed: it rebuilds cold and
// matches a cold build of the changed project.
func TestTransitionAfterOneShotEntryRebuildsCold(t *testing.T) {
	_, _, fixture := runOneShotWorkload(t, true)
	previous, _ := fixture.fullIndex(t)
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe Apex transition did not produce affected closure")
	}
	_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied {
		t.Fatal("transition applied on a one-shot entry; it must fail closed")
	}
	InvalidateRuntimeCaches()
	_, cold, err := runtimeFromIndexWithSourceDigests(current, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Methods) != len(cold.Methods) || len(entry.Classes) != len(cold.Classes) {
		t.Fatalf("rebuilt entry methods/classes = %d/%d, cold = %d/%d", len(entry.Methods), len(entry.Classes), len(cold.Methods), len(cold.Classes))
	}
}
