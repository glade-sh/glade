package apextest

import (
	"path/filepath"
	"reflect"
	"testing"
)

// A query on an object no catalog object can answer to fails without the lazy
// spread completing. Value, exception type and message must match the eager
// spread, for misses in every suffix form and for hits.
const lazySpreadMissingObjectClass = `@IsTest
private class MissingObjectParity {
	static void report(String s) { System.assert(false, s); }
	static String probe(String soql) {
		try { List<SObject> rows = Database.query(soql); return 'rows ' + rows.size(); }
		catch (Exception e) { return e.getTypeName() + ' ' + e.getMessage(); }
	}
	@IsTest static void unknownCustom() { report(probe('SELECT Id FROM NoSuchThing__c')); }
	@IsTest static void unknownStandard() { report(probe('SELECT Id FROM NoSuchThing')); }
	@IsTest static void unknownShare() { report(probe('SELECT Id FROM Fooo__Share')); }
	@IsTest static void unknownHistory() { report(probe('SELECT Id FROM Fooo__History')); }
	@IsTest static void unknownFeed() { report(probe('SELECT Id FROM Fooo__Feed')); }
	@IsTest static void unknownChangeEvent() { report(probe('SELECT Id FROM Fooo__ChangeEvent')); }
	@IsTest static void unknownExternal() { report(probe('SELECT Id FROM Fooo__x')); }
	@IsTest static void unknownMetadata() { report(probe('SELECT Id FROM Fooo__mdt')); }
	@IsTest static void unknownEvent() { report(probe('SELECT Id FROM Fooo__e')); }
	@IsTest static void unknownNamespaced() { report(probe('SELECT Id FROM ns__Fooo__c')); }
	@IsTest static void unknownSemiJoin() { report(probe('SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Zzz__c)')); }
	@IsTest static void unknownTypeof() { report(probe('SELECT TYPEOF What WHEN Zzz THEN Id END FROM Event')); }
	@IsTest static void unknownCount() {
		try { Database.countQuery('SELECT count() FROM Nope__c'); report('no error'); }
		catch (Exception e) { report(e.getTypeName() + ' ' + e.getMessage()); }
	}
	@IsTest static void knownStandard() { report(probe('SELECT Id FROM Account') + ' ' + probe('SELECT Id FROM accountshare') + ' ' + probe('SELECT Id FROM Opportunity__Feed')); }
	@IsTest static void knownThenUnknown() {
		String a = probe('SELECT Id FROM Contact LIMIT 1');
		String b = probe('SELECT Id FROM Nope__c');
		report(a + ' ' + b + ' ' + Schema.getGlobalDescribe().containsKey('nope__c') + ' ' + Schema.getGlobalDescribe().size());
	}
}`

func TestLazyStandardSpreadMissingObjectMatchesEagerSpread(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}
	run := func(eager bool) map[string]string {
		prior := eagerStandardSpread.Load()
		eagerStandardSpread.Store(eager)
		defer eagerStandardSpread.Store(prior)
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/MissingObjectParity.cls"), lazySpreadMissingObjectClass)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/MissingObjectParity.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
		index := loadTestIndex(t, root)
		report := Run(index, Options{NoDiskCache: true, Parallelism: 1})
		out := make(map[string]string)
		for _, suite := range report.Suites {
			for _, c := range suite.Cases {
				answer := string(c.Status)
				if c.Problem != nil {
					answer += ": " + c.Problem.Message
				}
				out[c.MethodName] = answer
			}
		}
		return out
	}
	eager := run(true)
	lazy := run(false)
	if len(eager) != 15 {
		t.Fatalf("eager run answered %d methods, want 15: %v", len(eager), eager)
	}
	if !reflect.DeepEqual(eager, lazy) {
		for name, want := range eager {
			if got := lazy[name]; got != want {
				t.Errorf("%s:\n eager %s\n lazy  %s", name, want, got)
			}
		}
	}
}
