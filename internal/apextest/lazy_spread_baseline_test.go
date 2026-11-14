package apextest

import (
	"path/filepath"
	"testing"
)

// TestLazySpreadBaselineObservations emits stable observable results so the
// same test can be run on the baseline and compared byte for byte.
func TestLazySpreadBaselineObservations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Neighbor.trigger"), `trigger Neighbor on Account (before insert) {
    for (Account row : Trigger.new) {
        try { Database.query('SELECT Id FROM UncapturedNeighbor__mdt'); }
        catch (Exception e) { row.Description = e.getTypeName() + ': ' + e.getMessage(); }
    }
}`)
	writeFile(t, filepath.Join(root, "NeighborTest.cls"), `@IsTest private class NeighborTest {
    static String miss() {
        try { Database.query('SELECT Id FROM UncapturedNeighbor__mdt'); return 'unexpected rows'; }
        catch (Exception e) { return e.getTypeName() + ': ' + e.getMessage(); }
    }
    @IsTest static void spread() { System.assert(false, miss()); }
    @IsTest static void triggerRoute() {
        Account row = new Account(Name='Sprout'); insert row;
        row = [SELECT Description FROM Account WHERE Id = :row.Id];
        System.assert(false, row.Description);
    }
}`)
	report := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
	observed := map[string]string{}
	for _, suite := range report.Suites {
		for _, c := range suite.Cases {
			if c.Problem == nil {
				t.Fatalf("%s: expected an observation, got %s", c.MethodName, c.Status)
			}
			observed[c.MethodName] = string(c.Status) + ": " + c.Problem.Message
		}
	}
	for _, name := range []string{"spread", "triggerRoute"} {
		if observed[name] == "" {
			t.Fatalf("missing %s observation: %v", name, observed)
		}
		t.Logf("OBSERVATION %s %s", name, observed[name])
	}
}
