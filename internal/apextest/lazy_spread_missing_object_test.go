package apextest

import (
	"path/filepath"
	"reflect"
	"strings"
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
	@IsTest static void typoStandard() { report(probe('SELECT Id FROM Acount') + ' ' + probe('SELECT Id FROM Opportunty') + ' ' + probe('SELECT Id FROM Contact__c')); }
	@IsTest static void lowercaseUnknown() { report(probe('select id from nosuchthing__c') + ' ' + probe('select id from nosuchthing') + ' ' + probe('select id from ns__nosuchthing__c')); }
	@IsTest static void lowercaseKnown() { report(probe('select id from account') + ' ' + probe('select id from contentversion') + ' ' + probe('select id from accountfeed') + ' ' + probe('select id from knowledge__kav')); }
	@IsTest static void namespacedPrefixes() { report(probe('SELECT Id FROM ns__Account') + ' ' + probe('SELECT Id FROM FSL__Time_Dependency__c') + ' ' + probe('SELECT Id FROM ns__Time_Dependency__c') + ' ' + probe('SELECT Id FROM Knowledge__kav')); }
	@IsTest static void missThenLazyObjects() {
		String a = probe('SELECT Id FROM Acount');
		String b = probe('SELECT Id FROM ContentVersion') + ' ' + probe('SELECT Id FROM AccountFeed') + ' ' + probe('SELECT Id FROM Knowledge__kav') + ' ' + probe('SELECT Id FROM Time_Dependency__c');
		report(a + ' ' + b + ' ' + Schema.getGlobalDescribe().containsKey('contentversion') + ' ' + Schema.getGlobalDescribe().size());
	}
	@IsTest static void entityDefinition() {
		String a = probe('SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName = \'Nope__c\'');
		String b = probe('SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName IN (\'Account\', \'ContentVersion\', \'AccountFeed\') ORDER BY QualifiedApiName');
		String c = probe('SELECT Id FROM Nope__c');
		String d = probe('SELECT QualifiedApiName FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = \'ContentVersion\' AND QualifiedApiName = \'Title\'');
		report(a + ' | ' + b + ' | ' + c + ' | ' + d);
	}
	@IsTest static void soslUnknownAndKnown() {
		String out = '';
		for (String q : new List<String>{
			'FIND \'x\' RETURNING Nope__c(Id)', 'FIND \'x\' RETURNING Acount(Id)', 'FIND \'x\' RETURNING account(Id)',
			'FIND \'x\' RETURNING ContentVersion(Id)', 'FIND \'x\' RETURNING ns__Nope__c(Id)', 'FIND \'x\' RETURNING Account(Id), Nope__c(Id)'}) {
			try { List<List<SObject>> r = Search.query(q); out += 'ok ' + r.size() + ';'; }
			catch (Exception e) { out += e.getTypeName() + ' ' + e.getMessage() + ';'; }
		}
		report(out);
	}
	@IsTest static void describeAndTypes() {
		String out = '';
		for (String n : new List<String>{'Nope__c', 'Acount', 'account', 'ContentVersion', 'ns__Account', 'Knowledge__kav', 'nope__c'}) {
			try { out += Schema.describeSObjects(new List<String>{n})[0].getName() + ';'; }
			catch (Exception e) { out += e.getTypeName() + ' ' + e.getMessage() + ';'; }
			out += (Schema.getGlobalDescribe().get(n) != null) + ';' + (Type.forName('Schema.' + n) != null) + ';';
		}
		report(out);
	}
	@IsTest static void dmlOnUnknownAndLazy() {
		String out = '';
		for (String n : new List<String>{'Nope__c', 'Acount', 'ContentVersion', 'Account'}) {
			try {
				Schema.SObjectType t = Schema.getGlobalDescribe().get(n);
				SObject s = t.newSObject();
				Database.SaveResult r = Database.insert(s, false);
				out += n + ' ' + r.isSuccess() + ';';
			} catch (Exception e) { out += n + ' ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
		}
		report(out + probe('SELECT Id FROM Nope__c'));
	}
	@IsTest static void knownStandard() { report(probe('SELECT Id FROM Account') + ' ' + probe('SELECT Id FROM accountshare') + ' ' + probe('SELECT Id FROM Opportunity__Feed')); }
	@IsTest static void knownThenUnknown() {
		String a = probe('SELECT Id FROM Contact LIMIT 1');
		String b = probe('SELECT Id FROM Nope__c');
		report(a + ' ' + b + ' ' + Schema.getGlobalDescribe().containsKey('nope__c') + ' ' + Schema.getGlobalDescribe().size());
	}
}`

func runMissingObjectParity(t *testing.T, className, source string, eager bool) map[string]string {
	t.Helper()
	prior := eagerStandardSpread.Load()
	eagerStandardSpread.Store(eager)
	defer eagerStandardSpread.Store(prior)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/"+className+".cls"), source)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/"+className+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
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

// Names a catalog object answers to must resolve and return rows after a miss
// has run, exactly as under the eager spread. The eager answer is asserted on
// its own, so a name that the lazy path wrongly treats as final cannot pass by
// failing the same way in both modes.
const lazySpreadAnswerableClass = `@IsTest
private class AnswerableParity {
	static void report(String s) { System.assert(false, s); }
	static String probe(String soql) {
		try { List<SObject> rows = Database.query(soql); return 'rows ' + rows.size(); }
		catch (Exception e) { return e.getTypeName() + ' ' + e.getMessage(); }
	}
	static String afterMiss(String soql) { String a = probe('SELECT Id FROM Zzz__c'); return a.substring(0, 30) + ' ' + probe(soql); }
	@IsTest static void kav() { report(afterMiss('SELECT Id FROM Knowledge__kav')); }
	@IsTest static void kavLower() { report(afterMiss('select id from knowledge__kav')); }
	@IsTest static void ka() { report(afterMiss('SELECT Id FROM Knowledge__ka')); }
	@IsTest static void kaLower() { report(afterMiss('select id from KNOWLEDGE__KA')); }
	@IsTest static void feed() { report(afterMiss('SELECT Id FROM AccountFeed') + ' ' + probe('SELECT Id FROM Knowledge__Feed')); }
	@IsTest static void history() { report(afterMiss('SELECT Id FROM AccountHistory') + ' ' + probe('select id from contacthistory')); }
	@IsTest static void share() { report(afterMiss('SELECT Id FROM AccountShare') + ' ' + probe('select id from contactshare')); }
	@IsTest static void changeEvent() { report(afterMiss('SELECT Id FROM AccountChangeEvent') + ' ' + probe('SELECT Id FROM ContactChangeEvent')); }
	@IsTest static void namespaced() { report(afterMiss('SELECT Id FROM FSL__Time_Dependency__c') + ' ' + probe('SELECT Id FROM Time_Dependency__c') + ' ' + probe('SELECT Id FROM fsl__time_dependency__c')); }
	@IsTest static void alias() { report(afterMiss('SELECT a.Id FROM Account a') + ' ' + probe('SELECT Id FROM Account AS a') + ' ' + probe('SELECT c.Id FROM contact c WHERE c.Id != null')); }
	@IsTest static void dataCategory() { report(afterMiss('SELECT Id FROM Knowledge__DataCategorySelection') + ' ' + probe('SELECT Id FROM ArticleType__DataCategorySelection')); }
}`

func TestLazyStandardSpreadAnswerableNamesAfterMissMatchEagerSpread(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}
	eager := runMissingObjectParity(t, "AnswerableParity", lazySpreadAnswerableClass, true)
	lazy := runMissingObjectParity(t, "AnswerableParity", lazySpreadAnswerableClass, false)
	for k, v := range eager {
		t.Logf("DBG %s => %s", k, v)
	}
	if len(eager) != 11 {
		t.Fatalf("eager run answered %d methods, want 11: %v", len(eager), eager)
	}
	// Independent assertion on the eager answers: names the catalog holds
	// answer with rows, not with a missing-object error. The names that the
	// eager org does not hold are listed so a change in the catalog is noticed.
	notHeld := map[string]bool{"changeEvent": true}
	for name, got := range eager {
		if notHeld[name] {
			continue
		}
		if !strings.Contains(got, " rows ") || strings.Contains(got, "is not supported") {
			t.Errorf("%s: eager answer is not all row results: %s", name, got)
		}
	}
	for name, want := range eager {
		if got := lazy[name]; got != want {
			t.Errorf("%s:\n eager %s\n lazy  %s", name, want, got)
		}
	}
}

func TestLazyStandardSpreadMissingObjectMatchesEagerSpread(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}
	run := func(eager bool) map[string]string {
		return runMissingObjectParity(t, "MissingObjectParity", lazySpreadMissingObjectClass, eager)
	}
	eager := run(true)
	lazy := run(false)
	if len(eager) != 24 {
		t.Fatalf("eager run answered %d methods, want 24: %v", len(eager), eager)
	}
	if !reflect.DeepEqual(eager, lazy) {
		for name, want := range eager {
			if got := lazy[name]; got != want {
				t.Errorf("%s:\n eager %s\n lazy  %s", name, want, got)
			}
		}
	}
}
