package apextest

import (
	"path/filepath"
	"testing"
)

// Reading an unset field after DML reaches the child relationship lookup. The
// lazy spread answers it from the negative filter and must give the answers the
// eager spread gives, for relationships that exist and names that do not.
func TestLazyStandardSpreadUnsetFieldReadsMatchEagerSpread(t *testing.T) {
	const class = `@IsTest
private class UnsetFieldReadTest {
	@IsTest static void unsetFieldsAfterDml() {
		Account a = new Account(Name = 'Parent');
		insert a;
		Account fresh = [SELECT Id, Name FROM Account WHERE Id = :a.Id];
		System.assertEquals(null, a.Phone);
		System.assertEquals(null, a.get('Description'));
		Contact c = new Contact(LastName = 'Kid', AccountId = a.Id);
		insert c;
		Account withKids = [SELECT Id, (SELECT Id FROM Contacts) FROM Account WHERE Id = :a.Id];
		System.assertEquals(1, withKids.getSObjects('Contacts').size());
		System.assertEquals(null, c.Title);
	}
}`
	run := func(eager bool) (total, passed int) {
		prior := eagerStandardSpread.Load()
		eagerStandardSpread.Store(eager)
		defer eagerStandardSpread.Store(prior)
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/UnsetFieldReadTest.cls"), class)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/UnsetFieldReadTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
		index := loadTestIndex(t, root)
		report := Run(index, Options{NoDiskCache: true, Parallelism: 1})
		for _, suite := range report.Suites {
			for _, c := range suite.Cases {
				if c.Problem != nil {
					t.Logf("eager=%v %s: %s", eager, c.MethodName, c.Problem.Message)
				}
			}
		}
		summary := report.Summary()
		return summary.Total, summary.Passed
	}
	eagerTotal, eagerPassed := run(true)
	lazyTotal, lazyPassed := run(false)
	if eagerTotal != 1 || lazyTotal != 1 || eagerPassed != lazyPassed {
		t.Fatalf("eager %d/%d passed, lazy %d/%d passed", eagerPassed, eagerTotal, lazyPassed, lazyTotal)
	}
	if eagerPassed != 1 {
		t.Fatalf("the unset field reads fail in both routes (%d/%d)", eagerPassed, eagerTotal)
	}
}
